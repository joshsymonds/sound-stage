"""Tests for runpod/shard_runner.py — per-shard song processing loop.

Reuses delyric.py's machinery; here it's mocked out (matching test_delyric.py's
style) so these tests exercise only the shard runner's resume/validate/marker
orchestration, not real GPU separation.
"""

from pathlib import Path

import pytest

import delyric
from runpod import shard_runner
from runpod.validation import DONE_MARKER_NAME, validate_song_outputs


def _make_song_dir(root: Path, name: str) -> Path:
    d = root / name
    d.mkdir()
    (d / "audio.webm").write_bytes(b"fake audio")
    return d


class TestReadSongList:
    def test_reads_one_name_per_line(self, tmp_path: Path) -> None:
        list_path = tmp_path / "songs.txt"
        list_path.write_text("Song A\nSong B\nSong C\n")
        assert shard_runner.read_song_list(list_path) == ["Song A", "Song B", "Song C"]

    def test_skips_blank_lines_and_comments(self, tmp_path: Path) -> None:
        list_path = tmp_path / "songs.txt"
        list_path.write_text("# a comment\nSong A\n\n  \nSong B\n#another\n")
        assert shard_runner.read_song_list(list_path) == ["Song A", "Song B"]

    def test_strips_whitespace(self, tmp_path: Path) -> None:
        list_path = tmp_path / "songs.txt"
        list_path.write_text("  Song A  \n")
        assert shard_runner.read_song_list(list_path) == ["Song A"]


class TestProcessShardSong:
    def test_skips_song_with_existing_done_marker(
        self, tmp_path: Path, monkeypatch: pytest.MonkeyPatch
    ) -> None:
        song_dir = _make_song_dir(tmp_path, "Already Done")
        (song_dir / DONE_MARKER_NAME).write_text("")

        called = []
        monkeypatch.setattr(delyric, "separate_song", lambda *a, **k: called.append(True))

        result = shard_runner.process_shard_song(tmp_path, "Already Done")
        assert result is True
        assert called == []

    def test_fails_when_audio_webm_missing(self, tmp_path: Path) -> None:
        d = tmp_path / "No Audio"
        d.mkdir()
        result = shard_runner.process_shard_song(tmp_path, "No Audio")
        assert result is False

    def test_processes_and_validates_and_writes_marker(
        self, tmp_path: Path, monkeypatch: pytest.MonkeyPatch
    ) -> None:
        song_dir = _make_song_dir(tmp_path, "Good Song")

        def fake_separate(song_dir_arg: Path, tmp_dir: Path) -> tuple[Path, Path]:
            vocals = tmp_dir / "vocals.wav"
            instrumental = tmp_dir / "instrumental.wav"
            vocals.write_bytes(b"v")
            instrumental.write_bytes(b"i")
            return vocals, instrumental

        def fake_encode(src: Path, dst: Path) -> None:
            dst.write_bytes(b"encoded")

        monkeypatch.setattr(delyric, "separate_song", fake_separate)
        monkeypatch.setattr(delyric, "encode_to_webm", fake_encode)
        monkeypatch.setattr(
            shard_runner, "validate_song_outputs", lambda d, **k: (True, "ok")
        )

        result = shard_runner.process_shard_song(tmp_path, "Good Song")
        assert result is True
        assert (song_dir / "vocals.webm").exists()
        assert (song_dir / "instrumental.webm").exists()
        assert (song_dir / DONE_MARKER_NAME).exists()

    def test_no_marker_written_when_validation_fails(
        self, tmp_path: Path, monkeypatch: pytest.MonkeyPatch
    ) -> None:
        song_dir = _make_song_dir(tmp_path, "Bad Song")

        def fake_separate(song_dir_arg: Path, tmp_dir: Path) -> tuple[Path, Path]:
            vocals = tmp_dir / "vocals.wav"
            instrumental = tmp_dir / "instrumental.wav"
            vocals.write_bytes(b"v")
            instrumental.write_bytes(b"i")
            return vocals, instrumental

        monkeypatch.setattr(delyric, "separate_song", fake_separate)
        monkeypatch.setattr(delyric, "encode_to_webm", lambda src, dst: dst.write_bytes(b""))
        monkeypatch.setattr(
            shard_runner, "validate_song_outputs", lambda d, **k: (False, "durations mismatch")
        )

        result = shard_runner.process_shard_song(tmp_path, "Bad Song")
        assert result is False
        assert not (song_dir / DONE_MARKER_NAME).exists()

    def test_separation_exception_is_caught_and_fails_without_marker(
        self, tmp_path: Path, monkeypatch: pytest.MonkeyPatch
    ) -> None:
        song_dir = _make_song_dir(tmp_path, "Explodes")

        def boom(song_dir_arg: Path, tmp_dir: Path) -> tuple[Path, Path]:
            raise RuntimeError("gpu on fire")

        monkeypatch.setattr(delyric, "separate_song", boom)

        result = shard_runner.process_shard_song(tmp_path, "Explodes")
        assert result is False
        assert not (song_dir / DONE_MARKER_NAME).exists()

    def test_real_validate_song_outputs_rejects_missing_outputs_on_encode_failure(
        self, tmp_path: Path, monkeypatch: pytest.MonkeyPatch
    ) -> None:
        """Without mocking validate_song_outputs: if encode silently no-ops
        (writes nothing), validation must still catch it and refuse the marker."""
        song_dir = _make_song_dir(tmp_path, "Encode Noop")

        def fake_separate(song_dir_arg: Path, tmp_dir: Path) -> tuple[Path, Path]:
            vocals = tmp_dir / "vocals.wav"
            instrumental = tmp_dir / "instrumental.wav"
            vocals.write_bytes(b"v")
            instrumental.write_bytes(b"i")
            return vocals, instrumental

        monkeypatch.setattr(delyric, "separate_song", fake_separate)
        monkeypatch.setattr(delyric, "encode_to_webm", lambda src, dst: None)  # writes nothing

        result = shard_runner.process_shard_song(tmp_path, "Encode Noop")
        assert result is False
        assert not (song_dir / DONE_MARKER_NAME).exists()
