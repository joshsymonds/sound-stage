"""Tests for runpod/validation.py — per-song output validation and done-marker
semantics.

Uses real ffmpeg-generated fixtures (not mocked ffprobe) per the task brief:
these exercise the actual ffprobe subprocess call against short synthesized
clips of controlled duration.
"""

import shutil
import subprocess
from pathlib import Path

import pytest

from runpod.validation import (
    DONE_MARKER_NAME,
    is_done,
    should_skip,
    validate_song_outputs,
    write_done_marker,
)

FFMPEG_AVAILABLE = shutil.which("ffmpeg") is not None and shutil.which("ffprobe") is not None

pytestmark = pytest.mark.skipif(not FFMPEG_AVAILABLE, reason="ffmpeg/ffprobe not on PATH")


def _make_clip(path: Path, duration_seconds: float) -> None:
    subprocess.run(
        [
            "ffmpeg", "-y", "-f", "lavfi",
            "-i", f"sine=frequency=440:duration={duration_seconds}",
            "-c:a", "libopus", "-b:a", "64k",
            str(path),
        ],
        check=True,
        capture_output=True,
    )


@pytest.fixture
def song_dir(tmp_path: Path) -> Path:
    d = tmp_path / "Artist - Title"
    d.mkdir()
    return d


class TestValidateSongOutputs:
    def test_valid_when_durations_match(self, song_dir: Path) -> None:
        _make_clip(song_dir / "audio.webm", 5.0)
        _make_clip(song_dir / "instrumental.webm", 5.0)
        _make_clip(song_dir / "vocals.webm", 5.0)

        ok, reason = validate_song_outputs(song_dir)
        assert ok is True, reason

    def test_valid_when_within_tolerance(self, song_dir: Path) -> None:
        _make_clip(song_dir / "audio.webm", 5.0)
        _make_clip(song_dir / "instrumental.webm", 6.5)
        _make_clip(song_dir / "vocals.webm", 5.0)

        ok, reason = validate_song_outputs(song_dir, tolerance_seconds=2.0)
        assert ok is True, reason

    def test_invalid_when_instrumental_too_short(self, song_dir: Path) -> None:
        _make_clip(song_dir / "audio.webm", 10.0)
        _make_clip(song_dir / "instrumental.webm", 2.0)
        _make_clip(song_dir / "vocals.webm", 10.0)

        ok, reason = validate_song_outputs(song_dir, tolerance_seconds=2.0)
        assert ok is False
        assert "instrumental" in reason.lower()

    def test_invalid_when_vocals_too_long(self, song_dir: Path) -> None:
        _make_clip(song_dir / "audio.webm", 10.0)
        _make_clip(song_dir / "instrumental.webm", 10.0)
        _make_clip(song_dir / "vocals.webm", 30.0)

        ok, reason = validate_song_outputs(song_dir, tolerance_seconds=2.0)
        assert ok is False
        assert "vocals" in reason.lower()

    def test_invalid_when_instrumental_missing(self, song_dir: Path) -> None:
        _make_clip(song_dir / "audio.webm", 5.0)
        _make_clip(song_dir / "vocals.webm", 5.0)

        ok, reason = validate_song_outputs(song_dir)
        assert ok is False
        assert "instrumental" in reason.lower()

    def test_invalid_when_vocals_missing(self, song_dir: Path) -> None:
        _make_clip(song_dir / "audio.webm", 5.0)
        _make_clip(song_dir / "instrumental.webm", 5.0)

        ok, reason = validate_song_outputs(song_dir)
        assert ok is False
        assert "vocals" in reason.lower()

    def test_invalid_when_audio_missing(self, song_dir: Path) -> None:
        _make_clip(song_dir / "instrumental.webm", 5.0)
        _make_clip(song_dir / "vocals.webm", 5.0)

        ok, reason = validate_song_outputs(song_dir)
        assert ok is False
        assert "audio" in reason.lower()

    def test_invalid_when_output_is_corrupt(self, song_dir: Path) -> None:
        _make_clip(song_dir / "audio.webm", 5.0)
        (song_dir / "instrumental.webm").write_bytes(b"not a real webm file")
        _make_clip(song_dir / "vocals.webm", 5.0)

        ok, reason = validate_song_outputs(song_dir)
        assert ok is False


class TestDoneMarker:
    def test_not_done_before_marker_written(self, song_dir: Path) -> None:
        assert is_done(song_dir) is False
        assert should_skip(song_dir) is False

    def test_done_after_marker_written(self, song_dir: Path) -> None:
        write_done_marker(song_dir)
        assert is_done(song_dir) is True
        assert should_skip(song_dir) is True

    def test_marker_uses_documented_filename(self, song_dir: Path) -> None:
        write_done_marker(song_dir)
        assert (song_dir / DONE_MARKER_NAME).exists()
