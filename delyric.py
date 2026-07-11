#!/usr/bin/env python3
"""Delyric — Vocal separation pipeline for UltraStar karaoke songs.

Runs the validated 3-model karaoke ensemble — anvuew + frazer/becruily
BS-Roformer, plus gabox_v2 Mel-Band Roformer, averaged with avg_wave and
test-time augmentation — via ZFTurbo's Music-Source-Separation-Training
(MSST) `inference.py`/`ensemble.py` CLIs, invoked as subprocesses, to remove
lead vocals while keeping backing vocals, producing instrumental and vocal
tracks from existing audio.webm files.
"""

import concurrent.futures
import hashlib
import logging
import os
import re
import shutil
import subprocess
import sys
import tempfile
import urllib.request
from pathlib import Path

import click
from tqdm import tqdm

# Model constants — the validated 3-model karaoke ensemble (BS-Roformer x2 +
# Mel-Band Roformer, avg_wave, TTA) chosen after an A/B listening test
# rejected the earlier audio-separator-based ensemble. All three checkpoints
# are loaded directly via MSST's inference.py rather than audio-separator's
# model registry — audio-separator 0.44.3 could not load anvuew's
# karaoke_bs_roformer checkpoint at all (Separator.download_model_files()
# only resolves registry-listed filenames, even when the file is already
# present via --model_file_dir), which is why this engine replaced it
# outright.
MSST_MODELS = {
    "anvuew": {
        "model_type": "bs_roformer",
        "ckpt_filename": "karaoke_bs_roformer_anvuew.ckpt",
        "ckpt_url": (
            "https://huggingface.co/anvuew/karaoke_bs_roformer/resolve/main/"
            "karaoke_bs_roformer_anvuew.ckpt"
        ),
        "ckpt_sha256": "206d04757cb5f75ca3b55f8a0a48f5c26aa2351d4ff3c7adbfc9affa30ea3ae4",
        "yaml_filename": "karaoke_bs_roformer_anvuew.yaml",
        "yaml_url": (
            "https://huggingface.co/anvuew/karaoke_bs_roformer/resolve/main/"
            "karaoke_bs_roformer_anvuew.yaml"
        ),
        "yaml_sha256": "5cb3f127ecbc6a8e37f31ea7e05f60f360a44da43e857bde805b7b68558f6338",
    },
    "frazer": {
        "model_type": "bs_roformer",
        "ckpt_filename": "bs_roformer_karaoke_frazer_becruily.ckpt",
        "ckpt_url": (
            "https://huggingface.co/becruily/bs-roformer-karaoke/resolve/main/"
            "bs_roformer_karaoke_frazer_becruily.ckpt"
        ),
        "ckpt_sha256": "eb90ee24c1154d83fbcfd27e96182f19e061557cc6e4746953125e08c29389f9",
        "yaml_filename": "config_karaoke_frazer_becruily.yaml",
        "yaml_url": (
            "https://huggingface.co/becruily/bs-roformer-karaoke/resolve/main/"
            "config_karaoke_frazer_becruily.yaml"
        ),
        "yaml_sha256": "1d3b58b473025183d0d3c91e7a444cb5f1418d9f34e8f46b8d2fb10d2cc8ab34",
    },
    "gabox_v2": {
        "model_type": "mel_band_roformer",
        "ckpt_filename": "mel_band_roformer_karaoke_gabox_v2.ckpt",
        "ckpt_url": (
            "https://github.com/nomadkaraoke/python-audio-separator/releases/download/"
            "model-configs/mel_band_roformer_karaoke_gabox_v2.ckpt"
        ),
        "ckpt_sha256": "ec34be50327aeaf1a996c27977f5c30d1ac80c0076d69683d3e5184c31ea29d3",
        "yaml_filename": "config_mel_band_roformer_karaoke_gabox.yaml",
        "yaml_url": (
            "https://github.com/nomadkaraoke/python-audio-separator/releases/download/"
            "model-configs/config_mel_band_roformer_karaoke_gabox.yaml"
        ),
        "yaml_sha256": "16b726891076fd0bebab9ce038240b51afb22b04eac6026a7f0356178dcb65b7",
    },
}
ENSEMBLE_ALGORITHM = "avg_wave"

DEFAULT_LIBRARY = "/mnt/music/sound-stage"
OPUS_BITRATE = "128k"

LOG_FILENAME = "delyric-errors.log"

MSST_INFERENCE_TIMEOUT = 1800  # 30 minutes per model — headroom for TTA (~3x runtime)
MSST_ENSEMBLE_TIMEOUT = 300  # 5 minutes — simple waveform averaging of already-separated stems
FFMPEG_TIMEOUT = 120  # 2 minutes per decode/encode

# Abort after this many consecutive failures — catches environmental breakage
# (e.g. a Nix GC that deletes the venv's python mid-run) before it silently
# burns through the queue producing 1000+ identical FileNotFoundError entries.
MAX_CONSECUTIVE_FAILURES = 5

logger = logging.getLogger("delyric")


def find_song_dirs(library_dir: Path) -> list[Path]:
    """Find all song directories containing audio.webm."""
    dirs = []
    for entry in sorted(library_dir.iterdir()):
        if entry.is_dir() and (entry / "audio.webm").exists():
            dirs.append(entry)
    return dirs


def is_processed(song_dir: Path) -> bool:
    """Check if a song directory already has separation outputs."""
    return (song_dir / "instrumental.webm").exists() and (song_dir / "vocals.webm").exists()


def resolve_msst_dir() -> Path:
    """Resolve the MSST checkout providing inference.py/ensemble.py, and verify it.

    DELYRIC_MSST_DIR is exported by nix/wrapper.sh, pointing at the pinned
    fetchFromGitHub store path baked into the delyric-worker package (see
    nix/delyric-worker.nix) — build-time immutable, no runtime bootstrap
    needed. Required and verified eagerly, mirroring the old
    resolve_audio_separator's fail-loud-at-startup shape: a bad/missing MSST
    checkout would otherwise fail deep into the first song's subprocess call.
    """
    raw = os.environ.get("DELYRIC_MSST_DIR")
    if not raw:
        raise RuntimeError(
            "DELYRIC_MSST_DIR is not set. It must point at a pinned "
            "Music-Source-Separation-Training checkout (see "
            "nix/delyric-worker.nix, exported by nix/wrapper.sh)."
        )
    msst_dir = Path(raw)
    if not (msst_dir / "inference.py").exists():
        raise RuntimeError(
            f"DELYRIC_MSST_DIR={msst_dir} has no inference.py — not a valid "
            "Music-Source-Separation-Training checkout."
        )
    return msst_dir


def resolve_model_dir() -> Path:
    """Resolve the directory MSST checkpoints/configs are downloaded into.

    Unlike audio-separator, MSST has no baked-in model registry/cache to
    fall back to — this is required, with no default, so a misconfigured
    deployment fails loudly instead of downloading checkpoints somewhere
    unexpected (or silently missing them).
    """
    raw = os.environ.get("DELYRIC_MODEL_DIR")
    if not raw:
        raise RuntimeError(
            "DELYRIC_MODEL_DIR is not set. MSST checkpoints have no baked-in "
            "cache location (unlike audio-separator's model registry) — set "
            "it to a writable directory for the ensemble's checkpoints and "
            "configs."
        )
    return Path(raw)


def _sha256_file(path: Path) -> str:
    digest = hashlib.sha256()
    with path.open("rb") as f:
        for chunk in iter(lambda: f.read(1 << 20), b""):
            digest.update(chunk)
    return digest.hexdigest()


def _download_file(url: str, dest: Path) -> None:
    """Thin wrapper around urlretrieve so tests can mock network access."""
    urllib.request.urlretrieve(url, dest)


def _ensure_model_file(model_dir: Path, filename: str, url: str, expected_sha256: str) -> Path:
    """Download `filename` into model_dir if missing, verifying its sha256.

    A checksum mismatch — whether on an already-present file or a fresh
    download — raises immediately rather than silently proceeding with a
    corrupt or tampered model file, mirroring the "fail loudly instead of
    quietly running broken" philosophy used elsewhere in this file.
    """
    dest = model_dir / filename
    if dest.exists():
        actual = _sha256_file(dest)
        if actual == expected_sha256:
            return dest
        raise RuntimeError(
            f"{dest} exists but its sha256 does not match the pinned checksum "
            f"(expected {expected_sha256}, got {actual}) — delete it and "
            "retry rather than risk running against a corrupted or tampered "
            "model file."
        )

    model_dir.mkdir(parents=True, exist_ok=True)
    tmp_dest = dest.with_name(dest.name + ".part")
    logger.info("Downloading %s from %s", filename, url)
    _download_file(url, tmp_dest)

    actual = _sha256_file(tmp_dest)
    if actual != expected_sha256:
        tmp_dest.unlink(missing_ok=True)
        raise RuntimeError(
            f"Downloaded {filename} from {url} but its sha256 does not "
            f"match the pinned checksum (expected {expected_sha256}, got "
            f"{actual}) — refusing to use a corrupted or tampered download."
        )
    tmp_dest.rename(dest)
    return dest


def ensure_msst_models(model_dir: Path) -> dict[str, dict[str, object]]:
    """Ensure every MSST ensemble model's checkpoint+config exist locally,
    downloading and sha256-verifying any that are missing.

    Returns a dict keyed by MSST_MODELS' keys, each holding "model_type",
    "ckpt", and "yaml" (resolved local Paths) — the shape consumed by
    run_msst_inference.
    """
    resolved: dict[str, dict[str, object]] = {}
    for key, spec in MSST_MODELS.items():
        ckpt_path = _ensure_model_file(
            model_dir, spec["ckpt_filename"], spec["ckpt_url"], spec["ckpt_sha256"]
        )
        yaml_path = _ensure_model_file(
            model_dir, spec["yaml_filename"], spec["yaml_url"], spec["yaml_sha256"]
        )
        resolved[key] = {"model_type": spec["model_type"], "ckpt": ckpt_path, "yaml": yaml_path}
    return resolved


MSST_DIR = None  # Populated at startup by main()/the worker lifespan before any processing.
MSST_MODEL_PATHS = None  # ditto — see ensure_msst_models.


def verify_cuda() -> None:
    """Verify the venv's PyTorch can see a CUDA device before processing begins.

    onnxruntime/torch can silently fall back to CPU on a Blackwell GPU when
    wheels mismatch — the separation still runs, just ~100x slower, with no
    error. On a queue of any real size that silent fallback burns days before
    anyone notices. Fail loudly here instead, using the same "probe at
    startup" shape as resolve_msst_dir/ensure_msst_models above.

    Probes with sys.executable rather than resolving "python3" off PATH:
    nix/wrapper.sh execs the venv's python directly without ever adding its
    bin/ to PATH (it only prepends ffmpeg's), so a which()-based probe would
    find a torch-less system python3 under the systemd deployment and fail
    startup on a perfectly healthy GPU. sys.executable is always the
    interpreter actually running this process, which is the venv python in
    every real path — the wrapper execs it for the worker, and the devenv
    shell puts venv bin first for the CLI.
    """
    try:
        result = subprocess.run(
            [sys.executable, "-c", "import torch; print(torch.cuda.is_available())"],
            capture_output=True,
            text=True,
            check=False,
            timeout=30,
        )
    except (subprocess.TimeoutExpired, OSError) as exc:
        raise RuntimeError(f"CUDA probe failed to run: {exc}") from exc

    if result.returncode != 0 or result.stdout.strip() != "True":
        raise RuntimeError(
            "CUDA is not available to this venv's PyTorch build "
            "(torch.cuda.is_available() returned False, or the probe itself "
            "failed to import torch). Running separation like this would "
            "silently fall back to CPU and burn days processing the queue. "
            "Check the GPU driver and that the installed torch wheel "
            "supports this GPU's compute capability (see requirements.txt), "
            f"then retry. probe returncode={result.returncode} "
            f"stdout={result.stdout!r} stderr={result.stderr!r}"
        )


def decode_to_wav(audio_path: Path, out_dir: Path) -> Path:
    """ffmpeg-decode audio.webm to a 44.1kHz stereo mix.wav for MSST input."""
    out_dir.mkdir(parents=True, exist_ok=True)
    wav_path = out_dir / "mix.wav"
    cmd = [
        "ffmpeg", "-y",
        "-i", str(audio_path),
        "-ar", "44100",
        "-ac", "2",
        str(wav_path),
    ]
    result = subprocess.run(cmd, capture_output=True, text=True, check=False, timeout=FFMPEG_TIMEOUT)
    if result.returncode != 0:
        raise RuntimeError(f"ffmpeg decode failed:\n{result.stderr}")
    return wav_path


def run_msst_inference(
    model_spec: dict[str, object], msst_dir: Path, input_dir: Path, store_dir: Path
) -> None:
    """Run MSST's inference.py for a single ensemble model.

    Writes {store_dir}/{input stem}/Vocals.wav and .../instrumental.wav (the
    stem name comes from the model's config — all three ensemble models
    target "Vocals", confirmed against their yaml configs).
    """
    cmd = [
        sys.executable, "inference.py",
        "--model_type", model_spec["model_type"],
        "--config_path", str(model_spec["yaml"]),
        "--start_check_point", str(model_spec["ckpt"]),
        "--input_folder", str(input_dir),
        "--store_dir", str(store_dir),
        "--extract_instrumental",
        "--use_tta",
    ]
    result = subprocess.run(
        cmd, capture_output=True, text=True, check=False, timeout=MSST_INFERENCE_TIMEOUT, cwd=msst_dir
    )
    if result.returncode != 0:
        raise RuntimeError(
            f"MSST inference failed (exit {result.returncode}):\n{result.stderr}"
        )


def run_ensemble(msst_dir: Path, files: list[Path], output_path: Path) -> None:
    """Run MSST's ensemble.py to average several stem WAVs with avg_wave."""
    cmd = [
        sys.executable, "ensemble.py",
        "--type", ENSEMBLE_ALGORITHM,
        "--files", *[str(f) for f in files],
        "--output", str(output_path),
    ]
    result = subprocess.run(
        cmd, capture_output=True, text=True, check=False, timeout=MSST_ENSEMBLE_TIMEOUT, cwd=msst_dir
    )
    if result.returncode != 0:
        raise RuntimeError(
            f"MSST ensemble failed (exit {result.returncode}):\n{result.stderr}"
        )


def encode_to_webm(wav_path: Path, output_path: Path) -> None:
    """Encode a WAV file to Opus in WebM container."""
    cmd = [
        "ffmpeg", "-y",
        "-i", str(wav_path),
        "-c:a", "libopus",
        "-b:a", OPUS_BITRATE,
        str(output_path),
    ]
    result = subprocess.run(cmd, capture_output=True, text=True, check=False, timeout=FFMPEG_TIMEOUT)
    if result.returncode != 0:
        raise RuntimeError(f"ffmpeg encode failed:\n{result.stderr}")


def _expect_file(path: Path) -> Path:
    """Fail loudly if an expected MSST/ensemble output is missing, instead of
    letting a later step (e.g. ensembling) fail on a confusing FileNotFoundError."""
    if not path.exists():
        raise RuntimeError(f"Expected MSST output at {path}, but it does not exist.")
    return path


def separate_song(song_dir: Path, tmpdir: Path) -> tuple[Path, Path]:
    """Run the MSST 3-model ensemble on audio.webm, return (vocals_wav, instrumental_wav)."""
    audio_path = song_dir / "audio.webm"

    input_dir = tmpdir / "input"
    mix_wav = decode_to_wav(audio_path, input_dir)

    instrumental_paths = []
    vocals_paths = []
    for key, spec in MSST_MODEL_PATHS.items():
        store_dir = tmpdir / f"out-{key}"
        run_msst_inference(spec, MSST_DIR, input_dir, store_dir)
        stem_dir = store_dir / mix_wav.stem
        instrumental_paths.append(_expect_file(stem_dir / "instrumental.wav"))
        vocals_paths.append(_expect_file(stem_dir / "Vocals.wav"))

    instrumental_out = tmpdir / "instrumental.wav"
    vocals_out = tmpdir / "vocals.wav"
    run_ensemble(MSST_DIR, instrumental_paths, instrumental_out)
    run_ensemble(MSST_DIR, vocals_paths, vocals_out)

    return vocals_out, instrumental_out


def update_song_txt(song_dir: Path) -> None:
    """Add #INSTRUMENTAL: and #VOCALS: tags to song.txt."""
    txt_path = song_dir / "song.txt"
    if not txt_path.exists():
        logger.warning("No song.txt in %s, skipping tag update", song_dir.name)
        return

    content = txt_path.read_text(encoding="utf-8")
    lines = content.split("\n")

    # Check if tags already present
    has_instrumental = any(
        re.match(r"^#INSTRUMENTAL:", line, re.IGNORECASE) for line in lines
    )
    has_vocals = any(
        re.match(r"^#VOCALS:", line, re.IGNORECASE) for line in lines
    )

    if has_instrumental and has_vocals:
        return

    # Find insertion point: after the last header line
    insert_idx = 0
    for i, line in enumerate(lines):
        if line.startswith("#"):
            insert_idx = i + 1
        else:
            break

    new_tags = []
    if not has_instrumental:
        new_tags.append("#INSTRUMENTAL:instrumental.webm")
    if not has_vocals:
        new_tags.append("#VOCALS:vocals.webm")

    for j, tag in enumerate(new_tags):
        lines.insert(insert_idx + j, tag)

    txt_path.write_text("\n".join(lines), encoding="utf-8")


def process_song(song_dir: Path, dry_run: bool = False) -> None:
    """Process a single song directory end-to-end."""
    if dry_run:
        click.echo(f"  Would process: {song_dir.name}")
        return

    with tempfile.TemporaryDirectory(prefix="delyric_", dir="/var/tmp") as tmpdir:
        tmpdir_path = Path(tmpdir)

        # Separate
        vocals_wav, instrumental_wav = separate_song(song_dir, tmpdir_path)

        # Encode to Opus/WebM (parallel — independent operations)
        with concurrent.futures.ThreadPoolExecutor(max_workers=2) as pool:
            fut_vocals = pool.submit(encode_to_webm, vocals_wav, song_dir / "vocals.webm")
            fut_instrumental = pool.submit(encode_to_webm, instrumental_wav, song_dir / "instrumental.webm")
            fut_vocals.result()
            fut_instrumental.result()

    # Update song.txt tags
    update_song_txt(song_dir)


@click.command()
@click.argument(
    "library_dir",
    type=click.Path(exists=True, file_okay=False, path_type=Path),
    default=DEFAULT_LIBRARY,
)
@click.option("--dry-run", is_flag=True, help="Preview what would be processed.")
@click.option("--song", "song_name", help="Process a single song directory by name.")
@click.option("--force", is_flag=True, help="Reprocess even if outputs exist.")
@click.option("--limit", type=int, default=None, help="Process at most N songs.")
def main(library_dir: Path, dry_run: bool, song_name: str | None, force: bool, limit: int | None) -> None:
    """Separate vocals from instrumentals in UltraStar karaoke songs.

    Processes songs in LIBRARY_DIR (default: /mnt/music/sound-stage/) using
    the validated MSST BS-Roformer/Mel-Band-Roformer 3-model ensemble
    (avg_wave, TTA).
    """
    # Set up error logging
    log_path = library_dir / LOG_FILENAME
    file_handler = logging.FileHandler(log_path, encoding="utf-8")
    file_handler.setFormatter(logging.Formatter("%(asctime)s %(levelname)s %(message)s"))
    logger.addHandler(file_handler)
    logger.setLevel(logging.WARNING)

    # Find songs to process
    if song_name:
        song_dir = library_dir / song_name
        if not song_dir.is_dir():
            click.echo(f"Song directory not found: {song_dir}", err=True)
            sys.exit(1)
        if not (song_dir / "audio.webm").exists():
            click.echo(f"No audio.webm in {song_dir}", err=True)
            sys.exit(1)
        songs = [song_dir]
    else:
        songs = find_song_dirs(library_dir)

    if not force:
        unprocessed = [s for s in songs if not is_processed(s)]
    else:
        unprocessed = songs

    if limit is not None:
        unprocessed = unprocessed[:limit]

    total = len(songs)
    to_process = len(unprocessed)
    skipped = total - to_process

    if dry_run:
        click.echo(f"Library: {library_dir}")
        click.echo(f"Total songs: {total}")
        click.echo(f"Already processed: {skipped}")
        click.echo(f"Would process: {to_process}")
        if unprocessed:
            click.echo()
            for s in unprocessed:
                click.echo(f"  {s.name}")
        return

    # Resolve the MSST checkout and models, and probe CUDA BEFORE touching any
    # songs — if the checkout is broken, models fail to verify, or GPU
    # inference would silently fall back to CPU, we want to know now, not
    # 1000 failures (or days of CPU-speed runs) later.
    global MSST_DIR, MSST_MODEL_PATHS
    MSST_DIR = resolve_msst_dir()
    MSST_MODEL_PATHS = ensure_msst_models(resolve_model_dir())
    verify_cuda()

    click.echo(f"Processing {to_process} songs ({skipped} already done)")

    processed = 0
    failed = 0
    consecutive_failures = 0
    aborted = False

    with tqdm(unprocessed, unit="song", desc="Separating") as pbar:
        for song_dir in pbar:
            pbar.set_postfix_str(song_dir.name[:40], refresh=True)
            try:
                process_song(song_dir)
                processed += 1
                consecutive_failures = 0
            except Exception:
                failed += 1
                consecutive_failures += 1
                logger.exception("Failed to process %s", song_dir.name)
                tqdm.write(f"  FAILED: {song_dir.name} (see {LOG_FILENAME})")
                if consecutive_failures >= MAX_CONSECUTIVE_FAILURES:
                    aborted = True
                    tqdm.write(
                        f"  ABORTING: {MAX_CONSECUTIVE_FAILURES} consecutive "
                        f"failures — environment likely broken. See {log_path}."
                    )
                    break

    click.echo()
    status = "Aborted" if aborted else "Done"
    click.echo(f"{status}: {processed} processed, {skipped} skipped, {failed} failed")
    if failed > 0:
        click.echo(f"Error details in: {log_path}")
    if aborted:
        sys.exit(1)


if __name__ == "__main__":
    main()
