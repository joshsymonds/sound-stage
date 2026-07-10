"""Tests for runpod/startup_probe.py — startup GPU probe.

delyric.verify_cuda / resolve_audio_separator / separate_song are mocked
(matching test_delyric.py's style); these tests exercise the probe's own
timing/threshold and error-propagation logic, not real GPU inference.
"""

from pathlib import Path

import pytest

import delyric
from runpod import startup_probe


@pytest.fixture(autouse=True)
def _fake_test_clip(tmp_path: Path, monkeypatch: pytest.MonkeyPatch) -> Path:
    clip_dir = tmp_path / "testclip"
    clip_dir.mkdir()
    (clip_dir / "audio.webm").write_bytes(b"fake clip")
    monkeypatch.setattr(startup_probe, "TEST_CLIP_DIR", clip_dir)
    return clip_dir


class TestRunProbe:
    def test_passes_when_separation_completes_under_threshold(
        self, monkeypatch: pytest.MonkeyPatch
    ) -> None:
        monkeypatch.setattr(delyric, "resolve_audio_separator", lambda: "/fake/audio-separator")
        monkeypatch.setattr(delyric, "verify_cuda", lambda: None)
        monkeypatch.setattr(delyric, "separate_song", lambda song_dir, tmp: (Path("v"), Path("i")))

        startup_probe.run_probe(timeout_seconds=120)  # should not raise

    def test_raises_when_verify_cuda_fails(self, monkeypatch: pytest.MonkeyPatch) -> None:
        monkeypatch.setattr(delyric, "resolve_audio_separator", lambda: "/fake/audio-separator")

        def boom() -> None:
            raise RuntimeError("CUDA is not available")

        monkeypatch.setattr(delyric, "verify_cuda", boom)

        with pytest.raises(RuntimeError, match="CUDA"):
            startup_probe.run_probe(timeout_seconds=120)

    def test_raises_when_resolve_audio_separator_fails(
        self, monkeypatch: pytest.MonkeyPatch
    ) -> None:
        def boom() -> str:
            raise RuntimeError("audio-separator not found on PATH")

        monkeypatch.setattr(delyric, "resolve_audio_separator", boom)

        with pytest.raises(RuntimeError, match="audio-separator"):
            startup_probe.run_probe(timeout_seconds=120)

    def test_raises_when_separation_exceeds_threshold(
        self, monkeypatch: pytest.MonkeyPatch
    ) -> None:
        monkeypatch.setattr(delyric, "resolve_audio_separator", lambda: "/fake/audio-separator")
        monkeypatch.setattr(delyric, "verify_cuda", lambda: None)

        times = iter([0.0, 200.0])  # start, end -> 200s elapsed
        monkeypatch.setattr(startup_probe.time, "monotonic", lambda: next(times))
        monkeypatch.setattr(delyric, "separate_song", lambda song_dir, tmp: (Path("v"), Path("i")))

        with pytest.raises(RuntimeError, match="120"):
            startup_probe.run_probe(timeout_seconds=120)

    def test_raises_when_separation_itself_fails(self, monkeypatch: pytest.MonkeyPatch) -> None:
        monkeypatch.setattr(delyric, "resolve_audio_separator", lambda: "/fake/audio-separator")
        monkeypatch.setattr(delyric, "verify_cuda", lambda: None)

        def boom(song_dir: Path, tmp: Path) -> tuple[Path, Path]:
            raise RuntimeError("audio-separator crashed")

        monkeypatch.setattr(delyric, "separate_song", boom)

        with pytest.raises(RuntimeError, match="crashed"):
            startup_probe.run_probe(timeout_seconds=120)

    def test_raises_when_test_clip_missing(
        self, monkeypatch: pytest.MonkeyPatch, tmp_path: Path
    ) -> None:
        empty_dir = tmp_path / "empty"
        empty_dir.mkdir()
        monkeypatch.setattr(startup_probe, "TEST_CLIP_DIR", empty_dir)
        monkeypatch.setattr(delyric, "resolve_audio_separator", lambda: "/fake/audio-separator")
        monkeypatch.setattr(delyric, "verify_cuda", lambda: None)

        with pytest.raises(RuntimeError, match="test clip"):
            startup_probe.run_probe(timeout_seconds=120)


class TestMain:
    def test_main_exits_zero_on_success(
        self, monkeypatch: pytest.MonkeyPatch
    ) -> None:
        monkeypatch.setattr(startup_probe, "run_probe", lambda timeout_seconds: None)
        with pytest.raises(SystemExit) as exc_info:
            startup_probe.main()
        assert exc_info.value.code == 0

    def test_main_exits_nonzero_and_logs_on_failure(
        self, monkeypatch: pytest.MonkeyPatch, caplog: pytest.LogCaptureFixture
    ) -> None:
        def boom(timeout_seconds: float) -> None:
            raise RuntimeError("GPU probe separation took too long")

        monkeypatch.setattr(startup_probe, "run_probe", boom)
        with caplog.at_level("ERROR"):
            with pytest.raises(SystemExit) as exc_info:
                startup_probe.main()
        assert exc_info.value.code != 0
        assert "STARTUP GPU PROBE FAILED" in caplog.text
