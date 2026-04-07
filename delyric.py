#!/usr/bin/env python3
"""Delyric — Vocal separation pipeline for UltraStar karaoke songs.

Uses audio-separator with Mel-Band Roformer + HTDemucs_ft ensemble to produce
instrumental and vocal tracks from existing audio.webm files.
"""

import concurrent.futures
import logging
import re
import subprocess
import sys
import tempfile
from pathlib import Path

import click
from tqdm import tqdm

# Model constants — update these when better checkpoints are released
PRIMARY_MODEL = "mel_band_roformer_viperx_1143.ckpt"
ENSEMBLE_MODEL = "htdemucs_ft.yaml"
ENSEMBLE_ALGORITHM = "max_fft"

DEFAULT_LIBRARY = "/mnt/music/sound-stage"
OPUS_BITRATE = "128k"

LOG_FILENAME = "delyric-errors.log"

SEPARATOR_TIMEOUT = 600  # 10 minutes per song for GPU separation
FFMPEG_TIMEOUT = 120  # 2 minutes per encode

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


def separate_song(song_dir: Path, tmpdir: Path) -> tuple[Path, Path]:
    """Run ensemble separation on audio.webm, return paths to vocals and instrumental WAVs."""
    audio_path = song_dir / "audio.webm"

    cmd = [
        "audio-separator",
        str(audio_path),
        "--model_filename", PRIMARY_MODEL,
        "--extra_models", ENSEMBLE_MODEL,
        "--ensemble_algorithm", ENSEMBLE_ALGORITHM,
        "--output_dir", str(tmpdir),
        "--output_format", "WAV",
    ]

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

    with tempfile.TemporaryDirectory(prefix="delyric_") as tmpdir:
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
def main(library_dir: Path, dry_run: bool, song_name: str | None, force: bool) -> None:
    """Separate vocals from instrumentals in UltraStar karaoke songs.

    Processes songs in LIBRARY_DIR (default: /mnt/music/sound-stage/) using
    AI ensemble separation (Mel-Band Roformer + HTDemucs_ft).
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

    click.echo(f"Processing {to_process} songs ({skipped} already done)")

    processed = 0
    failed = 0

    with tqdm(unprocessed, unit="song", desc="Separating") as pbar:
        for song_dir in pbar:
            pbar.set_postfix_str(song_dir.name[:40], refresh=True)
            try:
                process_song(song_dir)
                processed += 1
            except Exception:
                failed += 1
                logger.exception("Failed to process %s", song_dir.name)
                tqdm.write(f"  FAILED: {song_dir.name} (see {LOG_FILENAME})")

    click.echo()
    click.echo(f"Done: {processed} processed, {skipped} skipped, {failed} failed")
    if failed > 0:
        click.echo(f"Error details in: {log_path}")


if __name__ == "__main__":
    main()
