#!/usr/bin/env python3
"""Shard runner — processes one shard of the RunPod backfill on a single pod.

Runs inside the Docker image built from runpod/Dockerfile (invoked as
`python -m runpod.shard_runner`). Reuses delyric.py's separation and encoding
machinery directly rather than reimplementing it; per-song outputs are
validated before a done-marker is written, so the supervisor can safely
resume a shard after a stalled/terminated pod relaunches on a fresh one.

Tag rewriting (delyric.update_song_txt) deliberately does NOT happen here —
shard directories on the pod only ever receive audio.webm (not song.txt), so
tagging happens in the supervisor's local post-sync pass once outputs are
pulled back into the real library.
"""

import logging
import sys
import tempfile
import time
from pathlib import Path

import click

import delyric
from runpod.validation import is_done, validate_song_outputs, write_done_marker

logger = logging.getLogger("shard_runner")


def read_song_list(path: Path) -> list[str]:
    """Read one song directory name per line, skipping blanks and #comments."""
    names = []
    for line in path.read_text(encoding="utf-8").splitlines():
        line = line.strip()
        if not line or line.startswith("#"):
            continue
        names.append(line)
    return names


def process_shard_song(shard_dir: Path, song_name: str) -> bool:
    """Process a single song in the shard dir. Returns True on success
    (including the already-done resume case), False on any failure."""
    song_dir = shard_dir / song_name

    if is_done(song_dir):
        logger.info("skip (already done): %s", song_name)
        return True

    if not (song_dir / "audio.webm").exists():
        logger.error("missing audio.webm, cannot process: %s", song_name)
        return False

    try:
        with tempfile.TemporaryDirectory(prefix="shard_") as tmp:
            tmp_path = Path(tmp)
            vocals_wav, instrumental_wav = delyric.separate_song(song_dir, tmp_path)
            delyric.encode_to_webm(vocals_wav, song_dir / "vocals.webm")
            delyric.encode_to_webm(instrumental_wav, song_dir / "instrumental.webm")
    except Exception:
        logger.exception("separation failed: %s", song_name)
        return False

    ok, reason = validate_song_outputs(song_dir)
    if not ok:
        logger.error("validation failed for %s: %s", song_name, reason)
        return False

    write_done_marker(song_dir)
    return True


@click.command()
@click.option(
    "--shard-dir", required=True,
    type=click.Path(exists=True, file_okay=False, path_type=Path),
    help="Directory containing this pod's shard of song directories.",
)
@click.option(
    "--song-list", required=True,
    type=click.Path(exists=True, dir_okay=False, path_type=Path),
    help="File listing this shard's song directory names, one per line.",
)
def main(shard_dir: Path, song_list: Path) -> None:
    """Process every song in SONG_LIST under SHARD_DIR, resuming from any
    already-validated outputs left by a previous (stalled/terminated) pod."""
    delyric.AUDIO_SEPARATOR = delyric.resolve_audio_separator()
    delyric.verify_cuda()

    songs = read_song_list(song_list)
    click.echo(f"Processing {len(songs)} songs in shard {shard_dir}")

    processed = 0
    failed = 0
    for name in songs:
        ok = process_shard_song(shard_dir, name)
        ts = time.strftime("%Y-%m-%dT%H:%M:%S")
        if ok:
            processed += 1
            print(f"[{ts}] DONE {name}", flush=True)
        else:
            failed += 1
            print(f"[{ts}] FAILED {name}", flush=True)

    click.echo(f"Shard complete: {processed} done, {failed} failed")
    if failed:
        sys.exit(1)


if __name__ == "__main__":
    main()
