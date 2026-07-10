"""Per-song output validation and done-marker semantics for the RunPod
backfill shard runner and supervisor.

A song is "valid" only once instrumental.webm and vocals.webm exist AND their
durations match audio.webm within tolerance — this catches truncated or
corrupt outputs from a pod that died mid-encode, which bare file-existence
checks would miss. A done-marker is written only after validation passes, so
resume/skip logic never trusts an output that was never actually verified.
"""

import subprocess
from pathlib import Path

DONE_MARKER_NAME = ".delyric-done"
DEFAULT_DURATION_TOLERANCE_SECONDS = 2.0


def ffprobe_duration_seconds(path: Path) -> float:
    result = subprocess.run(
        [
            "ffprobe", "-v", "error",
            "-show_entries", "format=duration",
            "-of", "default=noprint_wrappers=1:nokey=1",
            str(path),
        ],
        capture_output=True,
        text=True,
        check=False,
        timeout=30,
    )
    if result.returncode != 0:
        raise RuntimeError(f"ffprobe failed on {path}: {result.stderr}")
    try:
        return float(result.stdout.strip())
    except ValueError as exc:
        raise RuntimeError(f"ffprobe returned no duration for {path}: {result.stdout!r}") from exc


def validate_song_outputs(
    song_dir: Path,
    tolerance_seconds: float = DEFAULT_DURATION_TOLERANCE_SECONDS,
) -> tuple[bool, str]:
    """Validate a song's separation outputs. Returns (ok, reason) — reason is
    "ok" on success, otherwise a human-readable cause for the shard runner's
    logs / the supervisor's sweep pass."""
    instrumental = song_dir / "instrumental.webm"
    vocals = song_dir / "vocals.webm"
    audio = song_dir / "audio.webm"

    if not audio.exists():
        return False, "missing audio.webm (cannot compare duration)"
    if not instrumental.exists():
        return False, "missing instrumental.webm"
    if not vocals.exists():
        return False, "missing vocals.webm"

    try:
        audio_duration = ffprobe_duration_seconds(audio)
    except RuntimeError as exc:
        return False, f"ffprobe failed on audio.webm: {exc}"

    for label, path in (("instrumental.webm", instrumental), ("vocals.webm", vocals)):
        try:
            duration = ffprobe_duration_seconds(path)
        except RuntimeError as exc:
            return False, f"ffprobe failed on {label}: {exc}"
        if abs(duration - audio_duration) > tolerance_seconds:
            return False, (
                f"{label} duration {duration:.2f}s differs from audio.webm "
                f"duration {audio_duration:.2f}s by more than {tolerance_seconds}s"
            )

    return True, "ok"


def is_done(song_dir: Path) -> bool:
    return (song_dir / DONE_MARKER_NAME).exists()


def should_skip(song_dir: Path) -> bool:
    """Resume predicate for the shard runner: skip a song already validated
    in a previous attempt on this or an earlier pod."""
    return is_done(song_dir)


def write_done_marker(song_dir: Path) -> None:
    (song_dir / DONE_MARKER_NAME).write_text("")
