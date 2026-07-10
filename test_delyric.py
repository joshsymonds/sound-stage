"""Tests for delyric.py — update_song_txt and is_processed logic."""

import subprocess
import sys
import tempfile
import pytest
from pathlib import Path

import delyric
from delyric import is_processed, update_song_txt, separate_song, verify_cuda


@pytest.fixture
def song_dir(tmp_path: Path) -> Path:
    """Create a minimal song directory with audio.webm and song.txt."""
    d = tmp_path / "Artist - Title"
    d.mkdir()
    (d / "audio.webm").write_bytes(b"fake audio")
    (d / "song.txt").write_text(
        "#ARTIST:Test Artist\n"
        "#TITLE:Test Title\n"
        "#MP3:audio.webm\n"
        "#VIDEO:video.webm\n"
        "#COVER:cover.jpg\n"
        "#BPM:120\n"
        "#GAP:1000\n"
        ": 0 2 13 Test\n",
        encoding="utf-8",
    )
    return d


class TestIsProcessed:
    def test_unprocessed(self, song_dir: Path) -> None:
        assert not is_processed(song_dir)

    def test_only_instrumental(self, song_dir: Path) -> None:
        (song_dir / "instrumental.webm").write_bytes(b"fake")
        assert not is_processed(song_dir)

    def test_only_vocals(self, song_dir: Path) -> None:
        (song_dir / "vocals.webm").write_bytes(b"fake")
        assert not is_processed(song_dir)

    def test_both_present(self, song_dir: Path) -> None:
        (song_dir / "instrumental.webm").write_bytes(b"fake")
        (song_dir / "vocals.webm").write_bytes(b"fake")
        assert is_processed(song_dir)


class TestUpdateSongTxt:
    def test_inserts_tags_after_last_header(self, song_dir: Path) -> None:
        update_song_txt(song_dir)
        content = (song_dir / "song.txt").read_text(encoding="utf-8")
        lines = content.split("\n")

        # Tags should be after #GAP (last header) and before lyrics
        gap_idx = next(i for i, l in enumerate(lines) if l.startswith("#GAP:"))
        instrumental_idx = next(i for i, l in enumerate(lines) if l.startswith("#INSTRUMENTAL:"))
        vocals_idx = next(i for i, l in enumerate(lines) if l.startswith("#VOCALS:"))
        lyrics_idx = next(i for i, l in enumerate(lines) if l.startswith(": "))

        assert instrumental_idx == gap_idx + 1
        assert vocals_idx == gap_idx + 2
        assert lyrics_idx == gap_idx + 3

    def test_correct_tag_values(self, song_dir: Path) -> None:
        update_song_txt(song_dir)
        content = (song_dir / "song.txt").read_text(encoding="utf-8")
        assert "#INSTRUMENTAL:instrumental.webm" in content
        assert "#VOCALS:vocals.webm" in content

    def test_preserves_existing_content(self, song_dir: Path) -> None:
        original = (song_dir / "song.txt").read_text(encoding="utf-8")
        update_song_txt(song_dir)
        updated = (song_dir / "song.txt").read_text(encoding="utf-8")

        # All original lines should still be present
        for line in original.split("\n"):
            assert line in updated

    def test_idempotent(self, song_dir: Path) -> None:
        update_song_txt(song_dir)
        first = (song_dir / "song.txt").read_text(encoding="utf-8")
        update_song_txt(song_dir)
        second = (song_dir / "song.txt").read_text(encoding="utf-8")
        assert first == second

    def test_skips_if_tags_already_present(self, song_dir: Path) -> None:
        # Manually add tags
        content = (song_dir / "song.txt").read_text(encoding="utf-8")
        content = content.replace(
            "#BPM:120\n",
            "#BPM:120\n#INSTRUMENTAL:instrumental.webm\n#VOCALS:vocals.webm\n",
        )
        (song_dir / "song.txt").write_text(content, encoding="utf-8")

        update_song_txt(song_dir)
        result = (song_dir / "song.txt").read_text(encoding="utf-8")
        # Should not duplicate tags
        assert result.count("#INSTRUMENTAL:") == 1
        assert result.count("#VOCALS:") == 1

    def test_adds_missing_vocal_tag_only(self, song_dir: Path) -> None:
        content = (song_dir / "song.txt").read_text(encoding="utf-8")
        content = content.replace("#BPM:120\n", "#BPM:120\n#INSTRUMENTAL:instrumental.webm\n")
        (song_dir / "song.txt").write_text(content, encoding="utf-8")

        update_song_txt(song_dir)
        result = (song_dir / "song.txt").read_text(encoding="utf-8")
        assert result.count("#INSTRUMENTAL:") == 1
        assert result.count("#VOCALS:") == 1

    def test_no_song_txt(self, tmp_path: Path) -> None:
        """Should not crash when song.txt is missing."""
        d = tmp_path / "No Txt Song"
        d.mkdir()
        update_song_txt(d)  # should log warning, not crash

    def test_headers_only_file(self, tmp_path: Path) -> None:
        """Handle song.txt that has only headers and no lyrics."""
        d = tmp_path / "Headers Only"
        d.mkdir()
        (d / "song.txt").write_text(
            "#ARTIST:Test\n#TITLE:Test\n#MP3:audio.webm\n",
            encoding="utf-8",
        )
        update_song_txt(d)
        content = (d / "song.txt").read_text(encoding="utf-8")
        assert "#INSTRUMENTAL:instrumental.webm" in content
        assert "#VOCALS:vocals.webm" in content


class TestSeparateSongCommand:
    """separate_song must invoke the 3-model karaoke Roformer ensemble.

    Uses tempfile directly (not the tmp_path fixture) so these directory
    paths never embed the test's own name into argv — pytest's tmp_path is
    derived from the test function name, which would make e.g.
    "test_no_htdemucs_anywhere_in_argv" falsely fail a substring scan of its
    own --output_dir path.
    """

    def _run_separate_song(self, monkeypatch: pytest.MonkeyPatch) -> list[str]:
        """Run separate_song with subprocess.run mocked, return the captured argv."""
        captured: dict[str, list[str]] = {}

        def fake_run(cmd: list[str], **kwargs: object) -> subprocess.CompletedProcess:
            captured["cmd"] = cmd
            out_idx = cmd.index("--output_dir") + 1
            out_dir = Path(cmd[out_idx])
            (out_dir / "audio_(Vocals)_ensemble.wav").write_bytes(b"")
            (out_dir / "audio_(Instrumental)_ensemble.wav").write_bytes(b"")
            return subprocess.CompletedProcess(cmd, 0, stdout="", stderr="")

        monkeypatch.setattr(delyric.subprocess, "run", fake_run)
        monkeypatch.setattr(delyric, "AUDIO_SEPARATOR", "/fake/audio-separator")

        with tempfile.TemporaryDirectory(prefix="delyric_test_") as base:
            base_path = Path(base)
            song_dir = base_path / "song"
            song_dir.mkdir()
            (song_dir / "audio.webm").write_bytes(b"fake audio")
            out_dir = base_path / "out"
            out_dir.mkdir()

            separate_song(song_dir, out_dir)
        return captured["cmd"]

    def test_includes_all_three_karaoke_models(
        self, monkeypatch: pytest.MonkeyPatch
    ) -> None:
        cmd = self._run_separate_song(monkeypatch)
        assert "mel_band_roformer_karaoke_aufr33_viperx_sdr_10.1956.ckpt" in cmd
        assert "mel_band_roformer_karaoke_gabox_v2.ckpt" in cmd
        assert "mel_band_roformer_karaoke_becruily.ckpt" in cmd

    def test_uses_avg_wave_ensemble_algorithm(
        self, monkeypatch: pytest.MonkeyPatch
    ) -> None:
        cmd = self._run_separate_song(monkeypatch)
        idx = cmd.index("--ensemble_algorithm")
        assert cmd[idx + 1] == "avg_wave"

    def test_sets_mdxc_overlap_16(self, monkeypatch: pytest.MonkeyPatch) -> None:
        cmd = self._run_separate_song(monkeypatch)
        idx = cmd.index("--mdxc_overlap")
        assert cmd[idx + 1] == "16"

    def test_uses_autocast(self, monkeypatch: pytest.MonkeyPatch) -> None:
        cmd = self._run_separate_song(monkeypatch)
        assert "--use_autocast" in cmd

    def test_no_htdemucs_anywhere_in_argv(self, monkeypatch: pytest.MonkeyPatch) -> None:
        cmd = self._run_separate_song(monkeypatch)
        assert not any("htdemucs" in str(arg).lower() for arg in cmd)

    def test_primary_model_is_aufr33_viperx(
        self, monkeypatch: pytest.MonkeyPatch
    ) -> None:
        """The primary --model_filename is the aufr33/viperx checkpoint, not an extra."""
        cmd = self._run_separate_song(monkeypatch)
        idx = cmd.index("--model_filename")
        assert cmd[idx + 1] == "mel_band_roformer_karaoke_aufr33_viperx_sdr_10.1956.ckpt"

    def test_model_file_dir_absent_when_env_unset(
        self, monkeypatch: pytest.MonkeyPatch
    ) -> None:
        monkeypatch.delenv("DELYRIC_MODEL_DIR", raising=False)
        cmd = self._run_separate_song(monkeypatch)
        assert "--model_file_dir" not in cmd

    def test_model_file_dir_present_when_env_set(
        self, monkeypatch: pytest.MonkeyPatch, tmp_path: Path
    ) -> None:
        model_dir = tmp_path / "models"
        monkeypatch.setenv("DELYRIC_MODEL_DIR", str(model_dir))
        cmd = self._run_separate_song(monkeypatch)
        idx = cmd.index("--model_file_dir")
        assert cmd[idx + 1] == str(model_dir)


class TestVerifyCuda:
    """verify_cuda() must fail loudly when the venv's torch has no CUDA device."""

    def test_raises_actionable_error_when_cuda_unavailable(
        self, monkeypatch: pytest.MonkeyPatch
    ) -> None:
        def fake_run(cmd: list[str], **kwargs: object) -> subprocess.CompletedProcess:
            return subprocess.CompletedProcess(cmd, 0, stdout="False\n", stderr="")

        monkeypatch.setattr(delyric.subprocess, "run", fake_run)

        with pytest.raises(RuntimeError, match="CUDA"):
            verify_cuda()

    def test_raises_when_probe_process_fails(
        self, monkeypatch: pytest.MonkeyPatch
    ) -> None:
        def fake_run(cmd: list[str], **kwargs: object) -> subprocess.CompletedProcess:
            return subprocess.CompletedProcess(cmd, 1, stdout="", stderr="ImportError: torch")

        monkeypatch.setattr(delyric.subprocess, "run", fake_run)

        with pytest.raises(RuntimeError, match="CUDA"):
            verify_cuda()

    def test_probes_with_sys_executable_not_a_path_lookup(
        self, monkeypatch: pytest.MonkeyPatch
    ) -> None:
        """nix/wrapper.sh execs the venv's python directly without ever adding
        its bin/ to PATH, so a which()-based probe would silently resolve to a
        torch-less system python3 under the systemd deployment and fail
        startup on a perfectly healthy GPU. sys.executable is always the
        interpreter actually running this process in every real deployment
        path, so the probe must use it instead of a PATH lookup — mock
        shutil.which to a decoy path to prove it's never consulted.
        """
        captured: dict[str, list[str]] = {}

        def fake_run(cmd: list[str], **kwargs: object) -> subprocess.CompletedProcess:
            captured["cmd"] = cmd
            return subprocess.CompletedProcess(cmd, 0, stdout="True\n", stderr="")

        monkeypatch.setattr(delyric.subprocess, "run", fake_run)
        monkeypatch.setattr(delyric.shutil, "which", lambda _name: "/decoy/python3")

        verify_cuda()
        assert captured["cmd"][0] == sys.executable
        assert captured["cmd"][0] != "/decoy/python3"

    def test_succeeds_when_cuda_available(
        self, monkeypatch: pytest.MonkeyPatch
    ) -> None:
        def fake_run(cmd: list[str], **kwargs: object) -> subprocess.CompletedProcess:
            return subprocess.CompletedProcess(cmd, 0, stdout="True\n", stderr="")

        monkeypatch.setattr(delyric.subprocess, "run", fake_run)

        verify_cuda()  # should not raise
