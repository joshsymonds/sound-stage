#!/usr/bin/env python3
"""Delyric — Vocal separation pipeline for UltraStar karaoke songs.

Uses audio-separator with a 3-model karaoke MelBand Roformer ensemble
(aufr33/viperx + gabox v2 + becruily, avg_wave) to remove lead vocals while
keeping backing vocals, producing instrumental and vocal tracks from
existing audio.webm files.
"""

import concurrent.futures
import logging
import os
import re
import shutil
import subprocess
import sys
import tempfile
from pathlib import Path

import click
from tqdm import tqdm

# Model constants — update these when better checkpoints are released.
#
# This is audio-separator's built-in `karaoke` ensemble preset (3-model MelBand
# Roformer, avg_wave — see `audio-separator --list_presets` /
# ensemble_presets.json), reproduced explicitly to match this file's existing
# --model_filename/--extra_models/--ensemble_algorithm style rather than the
# newer --ensemble_preset shortcut.
#
# anvuew's karaoke_bs_roformer checkpoint scores higher on the MVSEP lead/back
# separation leaderboard than aufr33_viperx and was the first choice for the
# primary slot, but it is NOT in audio-separator's model registry — only
# anvuew's De-Reverb checkpoints are. audio-separator 0.44.3's CLI has no path
# to load an unlisted checkpoint even if the file is already sitting in
# --model_file_dir: Separator.download_model_files() unconditionally looks the
# requested filename up in its bundled registry before ever touching local
# disk, and raises `ValueError: Model file <name> not found in supported model
# files` when it isn't there (confirmed by downloading the real checkpoint
# from HuggingFace and pointing --model_file_dir at it — same error). Next
# time a model bump is considered, re-check whether audio-separator has added
# an anvuew karaoke entry to its registry before assuming this trio is final.
#
# No htdemucs model — that task removes backing vocals along with the lead,
# which defeats the point of a karaoke ensemble.
PRIMARY_MODEL = "mel_band_roformer_karaoke_aufr33_viperx_sdr_10.1956.ckpt"
ENSEMBLE_EXTRA_MODELS = [
    "mel_band_roformer_karaoke_gabox_v2.ckpt",
    "mel_band_roformer_karaoke_becruily.ckpt",
]
ENSEMBLE_ALGORITHM = "avg_wave"
MDXC_OVERLAP = "16"

DEFAULT_LIBRARY = "/mnt/music/sound-stage"
OPUS_BITRATE = "128k"

LOG_FILENAME = "delyric-errors.log"

SEPARATOR_TIMEOUT = 1800  # 30 minutes per song — headroom for the 3-model ensemble at mdxc_overlap 16
FFMPEG_TIMEOUT = 120  # 2 minutes per encode

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


def resolve_audio_separator() -> str:
    """Resolve audio-separator to an absolute path and verify it actually runs.

    A bare `audio-separator` subprocess call will fail with FileNotFoundError not
    only when the binary is missing but also when its shebang target has been
    garbage-collected out of the Nix store — the failure is instant, so a long
    run can silently churn through thousands of songs logging identical errors.
    Resolve at startup and probe with --help so we detect breakage loudly
    before processing begins. --help is used instead of a feature-specific
    flag because it's universal and only verifies the interpreter can start.
    """
    path = shutil.which("audio-separator")
    if path is None:
        raise RuntimeError(
            "audio-separator not found on PATH. Enter the devenv shell "
            "(direnv allow / devenv shell) to rebuild the venv."
        )
    try:
        subprocess.run(
            [path, "--help"],
            capture_output=True,
            check=True,
            timeout=30,
        )
    except (subprocess.CalledProcessError, FileNotFoundError, OSError) as exc:
        raise RuntimeError(
            f"audio-separator at {path} is broken (likely a GC'd Nix store "
            f"dependency). Delete .devenv/state/delyric-venv and re-enter "
            f"the devenv shell to rebuild. Underlying error: {exc}"
        ) from exc
    return path


AUDIO_SEPARATOR = None  # Populated at startup by main() before any processing.


def verify_cuda() -> None:
    """Verify the venv's PyTorch can see a CUDA device before processing begins.

    onnxruntime/torch can silently fall back to CPU on a Blackwell GPU when
    wheels mismatch — the separation still runs, just ~100x slower, with no
    error. On a queue of any real size that silent fallback burns days before
    anyone notices. Fail loudly here instead, using the same "probe at
    startup" shape as resolve_audio_separator above.

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


def separate_song(song_dir: Path, tmpdir: Path) -> tuple[Path, Path]:
    """Run ensemble separation on audio.webm, return paths to vocals and instrumental WAVs."""
    audio_path = song_dir / "audio.webm"

    cmd = [
        AUDIO_SEPARATOR,
        str(audio_path),
        "--model_filename", PRIMARY_MODEL,
        "--extra_models", *ENSEMBLE_EXTRA_MODELS,
        "--ensemble_algorithm", ENSEMBLE_ALGORITHM,
        "--mdxc_overlap", MDXC_OVERLAP,
        "--use_autocast",
        "--output_dir", str(tmpdir),
        "--output_format", "WAV",
    ]

    # DELYRIC_MODEL_DIR overrides audio-separator's own model cache directory.
    # Under the hardened systemd unit, PrivateTmp wipes the default /tmp cache
    # on every restart; the unit points this at its persistent StateDirectory
    # instead. Unset, audio-separator falls back to its own baked-in default —
    # no need to duplicate that default here.
    model_dir = os.environ.get("DELYRIC_MODEL_DIR")
    if model_dir:
        cmd += ["--model_file_dir", model_dir]

    result = subprocess.run(cmd, capture_output=True, text=True, check=False, timeout=SEPARATOR_TIMEOUT)
    if result.returncode != 0:
        raise RuntimeError(
            f"audio-separator failed (exit {result.returncode}):\n{result.stderr}"
        )

    # audio-separator outputs files named like:
    # audio_(Vocals)_<model_slug>.wav and audio_(Instrumental)_<model_slug>.wav
    vocals_path = None
    instrumental_path = None
    for f in tmpdir.iterdir():
        if f.suffix != ".wav":
            continue
        if "(Vocals)" in f.name:
            vocals_path = f
        elif "(Instrumental)" in f.name:
            instrumental_path = f

    if vocals_path is None or instrumental_path is None:
        found = [f.name for f in tmpdir.iterdir()]
        raise RuntimeError(
            f"Expected vocals and instrumental WAVs in {tmpdir}, found: {found}"
        )

    return vocals_path, instrumental_path


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
    a 3-model karaoke MelBand Roformer ensemble (avg_wave).
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

    # Resolve and probe audio-separator and CUDA BEFORE touching any songs — if
    # the venv is broken, or GPU inference would silently fall back to CPU, we
    # want to know now, not 1000 failures (or days of CPU-speed runs) later.
    global AUDIO_SEPARATOR
    AUDIO_SEPARATOR = resolve_audio_separator()
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
