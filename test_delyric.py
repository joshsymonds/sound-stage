"""Tests for delyric.py — update_song_txt and is_processed logic."""

import ast
import hashlib
import re
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


class TestResolveMsstDir:
    """resolve_msst_dir() must fail loudly instead of silently proceeding
    with a broken/missing MSST checkout — same fail-fast shape as
    resolve_audio_separator had for the old engine."""

    def test_raises_when_unset(self, monkeypatch: pytest.MonkeyPatch) -> None:
        monkeypatch.delenv("DELYRIC_MSST_DIR", raising=False)
        with pytest.raises(RuntimeError, match="DELYRIC_MSST_DIR"):
            delyric.resolve_msst_dir()

    def test_raises_when_inference_py_missing(
        self, tmp_path: Path, monkeypatch: pytest.MonkeyPatch
    ) -> None:
        monkeypatch.setenv("DELYRIC_MSST_DIR", str(tmp_path))
        with pytest.raises(RuntimeError, match="inference.py"):
            delyric.resolve_msst_dir()

    def test_returns_path_when_valid(
        self, tmp_path: Path, monkeypatch: pytest.MonkeyPatch
    ) -> None:
        (tmp_path / "inference.py").write_text("")
        monkeypatch.setenv("DELYRIC_MSST_DIR", str(tmp_path))
        assert delyric.resolve_msst_dir() == tmp_path


class TestResolveModelDir:
    """resolve_model_dir() must fail loudly when DELYRIC_MODEL_DIR is unset —
    unlike audio-separator, MSST checkpoints have no baked-in registry cache
    to fall back to."""

    def test_raises_when_unset(self, monkeypatch: pytest.MonkeyPatch) -> None:
        monkeypatch.delenv("DELYRIC_MODEL_DIR", raising=False)
        with pytest.raises(RuntimeError, match="DELYRIC_MODEL_DIR"):
            delyric.resolve_model_dir()

    def test_returns_path_when_set(
        self, tmp_path: Path, monkeypatch: pytest.MonkeyPatch
    ) -> None:
        monkeypatch.setenv("DELYRIC_MODEL_DIR", str(tmp_path))
        assert delyric.resolve_model_dir() == tmp_path


class TestEnsureModelFile:
    """_ensure_model_file downloads+verifies a single checkpoint/config file."""

    def test_downloads_and_verifies_new_file(
        self, tmp_path: Path, monkeypatch: pytest.MonkeyPatch
    ) -> None:
        content = b"fake checkpoint bytes"
        expected_sha = hashlib.sha256(content).hexdigest()
        calls = []

        def fake_download(url: str, dest: Path) -> None:
            calls.append(url)
            Path(dest).write_bytes(content)

        monkeypatch.setattr(delyric, "_download_file", fake_download)
        result = delyric._ensure_model_file(
            tmp_path, "model.ckpt", "https://example.com/model.ckpt", expected_sha
        )
        assert result == tmp_path / "model.ckpt"
        assert result.read_bytes() == content
        assert calls == ["https://example.com/model.ckpt"]
        # No leftover partial-download file.
        assert list(tmp_path.iterdir()) == [tmp_path / "model.ckpt"]

    def test_raises_loudly_on_sha256_mismatch_after_download(
        self, tmp_path: Path, monkeypatch: pytest.MonkeyPatch
    ) -> None:
        def fake_download(url: str, dest: Path) -> None:
            Path(dest).write_bytes(b"wrong bytes")

        monkeypatch.setattr(delyric, "_download_file", fake_download)
        with pytest.raises(RuntimeError, match="sha256"):
            delyric._ensure_model_file(
                tmp_path, "model.ckpt", "https://example.com/model.ckpt", "0" * 64
            )
        # Refuses to leave a corrupted file at the real destination or a stray partial.
        assert list(tmp_path.iterdir()) == []

    def test_skips_download_when_existing_file_matches(
        self, tmp_path: Path, monkeypatch: pytest.MonkeyPatch
    ) -> None:
        content = b"already here"
        expected_sha = hashlib.sha256(content).hexdigest()
        (tmp_path / "model.ckpt").write_bytes(content)

        def fail_download(url: str, dest: Path) -> None:
            raise AssertionError("should not download when the existing file already verifies")

        monkeypatch.setattr(delyric, "_download_file", fail_download)
        result = delyric._ensure_model_file(
            tmp_path, "model.ckpt", "https://example.com/model.ckpt", expected_sha
        )
        assert result == tmp_path / "model.ckpt"

    def test_raises_loudly_when_existing_file_sha256_mismatches(
        self, tmp_path: Path, monkeypatch: pytest.MonkeyPatch
    ) -> None:
        (tmp_path / "model.ckpt").write_bytes(b"corrupted")

        def fail_download(url: str, dest: Path) -> None:
            raise AssertionError("should raise on the bad existing file, not attempt a download")

        monkeypatch.setattr(delyric, "_download_file", fail_download)
        with pytest.raises(RuntimeError, match="sha256"):
            delyric._ensure_model_file(
                tmp_path, "model.ckpt", "https://example.com/model.ckpt", "0" * 64
            )


class TestEnsureMsstModels:
    """ensure_msst_models() must resolve every model's ckpt+yaml, keyed by model."""

    def test_downloads_all_models_ckpt_and_yaml(
        self, tmp_path: Path, monkeypatch: pytest.MonkeyPatch
    ) -> None:
        fake_table = {
            "a": {
                "model_type": "bs_roformer",
                "ckpt_filename": "a.ckpt",
                "ckpt_url": "https://example.com/a.ckpt",
                "ckpt_sha256": hashlib.sha256(b"a-ckpt").hexdigest(),
                "yaml_filename": "a.yaml",
                "yaml_url": "https://example.com/a.yaml",
                "yaml_sha256": hashlib.sha256(b"a-yaml").hexdigest(),
            },
            "b": {
                "model_type": "mel_band_roformer",
                "ckpt_filename": "b.ckpt",
                "ckpt_url": "https://example.com/b.ckpt",
                "ckpt_sha256": hashlib.sha256(b"b-ckpt").hexdigest(),
                "yaml_filename": "b.yaml",
                "yaml_url": "https://example.com/b.yaml",
                "yaml_sha256": hashlib.sha256(b"b-yaml").hexdigest(),
            },
        }
        content_by_url = {
            "https://example.com/a.ckpt": b"a-ckpt",
            "https://example.com/a.yaml": b"a-yaml",
            "https://example.com/b.ckpt": b"b-ckpt",
            "https://example.com/b.yaml": b"b-yaml",
        }

        def fake_download(url: str, dest: Path) -> None:
            Path(dest).write_bytes(content_by_url[url])

        monkeypatch.setattr(delyric, "MSST_MODELS", fake_table)
        monkeypatch.setattr(delyric, "_download_file", fake_download)

        resolved = delyric.ensure_msst_models(tmp_path)

        assert set(resolved) == {"a", "b"}
        assert resolved["a"]["model_type"] == "bs_roformer"
        assert resolved["a"]["ckpt"] == tmp_path / "a.ckpt"
        assert resolved["a"]["yaml"] == tmp_path / "a.yaml"
        assert resolved["b"]["model_type"] == "mel_band_roformer"
        assert resolved["b"]["ckpt"] == tmp_path / "b.ckpt"
        assert resolved["b"]["yaml"] == tmp_path / "b.yaml"


class TestMsstModelsTableIntegrity:
    """Guards the transcribed model constants — these exact URLs/hashes came
    from the validated listening-test recipe and must match verbatim."""

    def test_anvuew(self) -> None:
        spec = delyric.MSST_MODELS["anvuew"]
        assert spec["model_type"] == "bs_roformer"
        assert spec["ckpt_url"] == (
            "https://huggingface.co/anvuew/karaoke_bs_roformer/resolve/main/"
            "karaoke_bs_roformer_anvuew.ckpt"
        )
        assert spec["ckpt_sha256"] == (
            "206d04757cb5f75ca3b55f8a0a48f5c26aa2351d4ff3c7adbfc9affa30ea3ae4"
        )
        assert spec["yaml_url"] == (
            "https://huggingface.co/anvuew/karaoke_bs_roformer/resolve/main/"
            "karaoke_bs_roformer_anvuew.yaml"
        )
        assert spec["yaml_sha256"] == (
            "5cb3f127ecbc6a8e37f31ea7e05f60f360a44da43e857bde805b7b68558f6338"
        )

    def test_frazer(self) -> None:
        spec = delyric.MSST_MODELS["frazer"]
        assert spec["model_type"] == "bs_roformer"
        assert spec["ckpt_url"] == (
            "https://huggingface.co/becruily/bs-roformer-karaoke/resolve/main/"
            "bs_roformer_karaoke_frazer_becruily.ckpt"
        )
        assert spec["ckpt_sha256"] == (
            "eb90ee24c1154d83fbcfd27e96182f19e061557cc6e4746953125e08c29389f9"
        )
        assert spec["yaml_url"] == (
            "https://huggingface.co/becruily/bs-roformer-karaoke/resolve/main/"
            "config_karaoke_frazer_becruily.yaml"
        )
        assert spec["yaml_sha256"] == (
            "1d3b58b473025183d0d3c91e7a444cb5f1418d9f34e8f46b8d2fb10d2cc8ab34"
        )

    def test_gabox_v2(self) -> None:
        spec = delyric.MSST_MODELS["gabox_v2"]
        assert spec["model_type"] == "mel_band_roformer"
        assert spec["ckpt_url"] == (
            "https://github.com/nomadkaraoke/python-audio-separator/releases/download/"
            "model-configs/mel_band_roformer_karaoke_gabox_v2.ckpt"
        )
        assert spec["ckpt_sha256"] == (
            "ec34be50327aeaf1a996c27977f5c30d1ac80c0076d69683d3e5184c31ea29d3"
        )
        assert spec["yaml_url"] == (
            "https://github.com/nomadkaraoke/python-audio-separator/releases/download/"
            "model-configs/config_mel_band_roformer_karaoke_gabox.yaml"
        )
        assert spec["yaml_sha256"] == (
            "16b726891076fd0bebab9ce038240b51afb22b04eac6026a7f0356178dcb65b7"
        )


class TestDecodeToWav:
    """decode_to_wav must ffmpeg-decode audio.webm to a 44.1kHz stereo mix.wav."""

    def test_builds_correct_ffmpeg_command(
        self, tmp_path: Path, monkeypatch: pytest.MonkeyPatch
    ) -> None:
        captured: dict[str, list[str]] = {}

        def fake_run(cmd: list[str], **kwargs: object) -> subprocess.CompletedProcess:
            captured["cmd"] = cmd
            Path(cmd[-1]).write_bytes(b"")
            return subprocess.CompletedProcess(cmd, 0, stdout="", stderr="")

        monkeypatch.setattr(delyric.subprocess, "run", fake_run)
        audio_path = tmp_path / "audio.webm"
        audio_path.write_bytes(b"fake audio")
        out_dir = tmp_path / "input"

        result = delyric.decode_to_wav(audio_path, out_dir)

        assert result == out_dir / "mix.wav"
        cmd = captured["cmd"]
        assert cmd[0] == "ffmpeg"
        assert cmd[cmd.index("-i") + 1] == str(audio_path)
        assert cmd[cmd.index("-ar") + 1] == "44100"
        assert cmd[cmd.index("-ac") + 1] == "2"
        assert cmd[-1] == str(out_dir / "mix.wav")

    def test_raises_on_ffmpeg_failure(
        self, tmp_path: Path, monkeypatch: pytest.MonkeyPatch
    ) -> None:
        def fake_run(cmd: list[str], **kwargs: object) -> subprocess.CompletedProcess:
            return subprocess.CompletedProcess(cmd, 1, stdout="", stderr="boom")

        monkeypatch.setattr(delyric.subprocess, "run", fake_run)
        audio_path = tmp_path / "audio.webm"
        audio_path.write_bytes(b"fake audio")

        with pytest.raises(RuntimeError, match="ffmpeg decode failed"):
            delyric.decode_to_wav(audio_path, tmp_path / "input")


class TestEncodeToWebm:
    """encode_to_webm must ffmpeg-encode a WAV to Opus/WebM at 128k."""

    def test_builds_correct_ffmpeg_command(
        self, tmp_path: Path, monkeypatch: pytest.MonkeyPatch
    ) -> None:
        captured: dict[str, list[str]] = {}

        def fake_run(cmd: list[str], **kwargs: object) -> subprocess.CompletedProcess:
            captured["cmd"] = cmd
            Path(cmd[-1]).write_bytes(b"")
            return subprocess.CompletedProcess(cmd, 0, stdout="", stderr="")

        monkeypatch.setattr(delyric.subprocess, "run", fake_run)
        wav_path = tmp_path / "vocals.wav"
        wav_path.write_bytes(b"fake wav")
        out_path = tmp_path / "vocals.webm"

        delyric.encode_to_webm(wav_path, out_path)

        cmd = captured["cmd"]
        assert cmd[0] == "ffmpeg"
        assert cmd[cmd.index("-i") + 1] == str(wav_path)
        assert cmd[cmd.index("-c:a") + 1] == "libopus"
        assert cmd[cmd.index("-b:a") + 1] == "128k"
        assert cmd[-1] == str(out_path)

    def test_raises_on_ffmpeg_failure(
        self, tmp_path: Path, monkeypatch: pytest.MonkeyPatch
    ) -> None:
        def fake_run(cmd: list[str], **kwargs: object) -> subprocess.CompletedProcess:
            return subprocess.CompletedProcess(cmd, 1, stdout="", stderr="boom")

        monkeypatch.setattr(delyric.subprocess, "run", fake_run)

        with pytest.raises(RuntimeError, match="ffmpeg encode failed"):
            delyric.encode_to_webm(tmp_path / "in.wav", tmp_path / "out.webm")


class TestRunMsstInference:
    """run_msst_inference must invoke inference.py with the validated flags."""

    def _run(
        self, monkeypatch: pytest.MonkeyPatch, tmp_path: Path
    ) -> tuple[list[str], dict[str, object]]:
        captured: dict[str, object] = {}

        def fake_run(cmd: list[str], **kwargs: object) -> subprocess.CompletedProcess:
            captured["cmd"] = cmd
            captured["kwargs"] = kwargs
            return subprocess.CompletedProcess(cmd, 0, stdout="", stderr="")

        monkeypatch.setattr(delyric.subprocess, "run", fake_run)
        msst_dir = tmp_path / "msst"
        model_spec = {
            "model_type": "bs_roformer",
            "ckpt": tmp_path / "model.ckpt",
            "yaml": tmp_path / "model.yaml",
        }
        delyric.run_msst_inference(
            model_spec, msst_dir, tmp_path / "input", tmp_path / "store"
        )
        return captured["cmd"], captured["kwargs"]

    def test_command_construction(
        self, tmp_path: Path, monkeypatch: pytest.MonkeyPatch
    ) -> None:
        cmd, kwargs = self._run(monkeypatch, tmp_path)
        assert cmd[0] == sys.executable
        assert cmd[1] == "inference.py"
        assert cmd[cmd.index("--model_type") + 1] == "bs_roformer"
        assert cmd[cmd.index("--config_path") + 1] == str(tmp_path / "model.yaml")
        assert cmd[cmd.index("--start_check_point") + 1] == str(tmp_path / "model.ckpt")
        assert cmd[cmd.index("--input_folder") + 1] == str(tmp_path / "input")
        assert cmd[cmd.index("--store_dir") + 1] == str(tmp_path / "store")
        assert "--extract_instrumental" in cmd
        assert "--use_tta" in cmd

    def test_runs_with_msst_dir_as_cwd(
        self, tmp_path: Path, monkeypatch: pytest.MonkeyPatch
    ) -> None:
        _cmd, kwargs = self._run(monkeypatch, tmp_path)
        assert kwargs["cwd"] == tmp_path / "msst"

    def test_raises_on_failure(
        self, tmp_path: Path, monkeypatch: pytest.MonkeyPatch
    ) -> None:
        def fake_run(cmd: list[str], **kwargs: object) -> subprocess.CompletedProcess:
            return subprocess.CompletedProcess(cmd, 1, stdout="", stderr="boom")

        monkeypatch.setattr(delyric.subprocess, "run", fake_run)
        model_spec = {
            "model_type": "bs_roformer",
            "ckpt": tmp_path / "model.ckpt",
            "yaml": tmp_path / "model.yaml",
        }
        with pytest.raises(RuntimeError, match="MSST inference failed"):
            delyric.run_msst_inference(
                model_spec, tmp_path / "msst", tmp_path / "input", tmp_path / "store"
            )

    def test_mel_band_roformer_model_type(
        self, tmp_path: Path, monkeypatch: pytest.MonkeyPatch
    ) -> None:
        captured: dict[str, list[str]] = {}

        def fake_run(cmd: list[str], **kwargs: object) -> subprocess.CompletedProcess:
            captured["cmd"] = cmd
            return subprocess.CompletedProcess(cmd, 0, stdout="", stderr="")

        monkeypatch.setattr(delyric.subprocess, "run", fake_run)
        model_spec = {
            "model_type": "mel_band_roformer",
            "ckpt": tmp_path / "model.ckpt",
            "yaml": tmp_path / "model.yaml",
        }
        delyric.run_msst_inference(
            model_spec, tmp_path / "msst", tmp_path / "input", tmp_path / "store"
        )
        cmd = captured["cmd"]
        assert cmd[cmd.index("--model_type") + 1] == "mel_band_roformer"


class TestRunEnsemble:
    """run_ensemble must invoke ensemble.py with avg_wave over the given files."""

    def test_command_construction(
        self, tmp_path: Path, monkeypatch: pytest.MonkeyPatch
    ) -> None:
        captured: dict[str, object] = {}

        def fake_run(cmd: list[str], **kwargs: object) -> subprocess.CompletedProcess:
            captured["cmd"] = cmd
            captured["kwargs"] = kwargs
            return subprocess.CompletedProcess(cmd, 0, stdout="", stderr="")

        monkeypatch.setattr(delyric.subprocess, "run", fake_run)
        msst_dir = tmp_path / "msst"
        files = [tmp_path / "a.wav", tmp_path / "b.wav", tmp_path / "c.wav"]
        output = tmp_path / "out.wav"

        delyric.run_ensemble(msst_dir, files, output)

        cmd = captured["cmd"]
        assert cmd[0] == sys.executable
        assert cmd[1] == "ensemble.py"
        assert cmd[cmd.index("--type") + 1] == "avg_wave"
        files_idx = cmd.index("--files")
        assert cmd[files_idx + 1 : files_idx + 4] == [str(f) for f in files]
        assert cmd[cmd.index("--output") + 1] == str(output)
        assert captured["kwargs"]["cwd"] == msst_dir

    def test_raises_on_failure(
        self, tmp_path: Path, monkeypatch: pytest.MonkeyPatch
    ) -> None:
        def fake_run(cmd: list[str], **kwargs: object) -> subprocess.CompletedProcess:
            return subprocess.CompletedProcess(cmd, 1, stdout="", stderr="boom")

        monkeypatch.setattr(delyric.subprocess, "run", fake_run)
        with pytest.raises(RuntimeError, match="MSST ensemble failed"):
            delyric.run_ensemble(
                tmp_path / "msst", [tmp_path / "a.wav"], tmp_path / "out.wav"
            )


class TestSeparateSongOrchestration:
    """separate_song must decode, run all three real models, then ensemble
    the results — exercised against the real MSST_MODELS table (with
    throwaway ckpt/yaml files standing in for the real multi-GB downloads)
    so this test catches a wrong key/model_type/filename in production data,
    not just in a synthetic fixture.
    """

    def _fake_model_paths(self, tmp_path: Path) -> dict:
        resolved = {}
        for key, spec in delyric.MSST_MODELS.items():
            ckpt = tmp_path / "models" / spec["ckpt_filename"]
            yaml_path = tmp_path / "models" / spec["yaml_filename"]
            ckpt.parent.mkdir(parents=True, exist_ok=True)
            ckpt.write_bytes(b"")
            yaml_path.write_bytes(b"")
            resolved[key] = {"model_type": spec["model_type"], "ckpt": ckpt, "yaml": yaml_path}
        return resolved

    def _run_separate_song(
        self, monkeypatch: pytest.MonkeyPatch, tmp_path: Path
    ) -> tuple[list[list[str]], Path, Path]:
        calls: list[list[str]] = []

        def fake_run(cmd: list[str], **kwargs: object) -> subprocess.CompletedProcess:
            calls.append(cmd)
            if cmd[1] == "inference.py":
                store_dir = Path(cmd[cmd.index("--store_dir") + 1])
                stem_dir = store_dir / "mix"
                stem_dir.mkdir(parents=True, exist_ok=True)
                (stem_dir / "instrumental.wav").write_bytes(b"")
                (stem_dir / "Vocals.wav").write_bytes(b"")
            elif cmd[1] == "ensemble.py":
                Path(cmd[cmd.index("--output") + 1]).write_bytes(b"")
            return subprocess.CompletedProcess(cmd, 0, stdout="", stderr="")

        monkeypatch.setattr(delyric.subprocess, "run", fake_run)
        monkeypatch.setattr(delyric, "MSST_DIR", tmp_path / "msst")
        monkeypatch.setattr(delyric, "MSST_MODEL_PATHS", self._fake_model_paths(tmp_path))

        song_dir = tmp_path / "song"
        song_dir.mkdir()
        (song_dir / "audio.webm").write_bytes(b"fake audio")
        work_dir = tmp_path / "work"
        work_dir.mkdir()

        vocals_out, instrumental_out = separate_song(song_dir, work_dir)
        return calls, vocals_out, instrumental_out

    def test_decodes_audio_first(
        self, tmp_path: Path, monkeypatch: pytest.MonkeyPatch
    ) -> None:
        calls, _vocals, _instrumental = self._run_separate_song(monkeypatch, tmp_path)
        decode_calls = [c for c in calls if c[0] == "ffmpeg"]
        assert len(decode_calls) == 1
        assert decode_calls[0][decode_calls[0].index("-i") + 1] == str(tmp_path / "song" / "audio.webm")

    def test_runs_inference_for_all_three_real_models(
        self, tmp_path: Path, monkeypatch: pytest.MonkeyPatch
    ) -> None:
        calls, _vocals, _instrumental = self._run_separate_song(monkeypatch, tmp_path)
        inference_calls = [c for c in calls if c[1] == "inference.py"]
        assert len(inference_calls) == 3
        model_types = {c[c.index("--model_type") + 1] for c in inference_calls}
        assert model_types == {"bs_roformer", "mel_band_roformer"}
        checkpoints = {c[c.index("--start_check_point") + 1] for c in inference_calls}
        assert checkpoints == {
            str(tmp_path / "models" / spec["ckpt_filename"])
            for spec in delyric.MSST_MODELS.values()
        }

    def test_runs_two_ensemble_passes_over_three_files_each(
        self, tmp_path: Path, monkeypatch: pytest.MonkeyPatch
    ) -> None:
        calls, _vocals, _instrumental = self._run_separate_song(monkeypatch, tmp_path)
        ensemble_calls = [c for c in calls if c[1] == "ensemble.py"]
        assert len(ensemble_calls) == 2
        for c in ensemble_calls:
            files_idx = c.index("--files")
            assert len(c[files_idx + 1 : files_idx + 4]) == 3
            assert c[c.index("--type") + 1] == "avg_wave"

    def test_ensembles_instrumental_and_vocals_separately(
        self, tmp_path: Path, monkeypatch: pytest.MonkeyPatch
    ) -> None:
        calls, _vocals, _instrumental = self._run_separate_song(monkeypatch, tmp_path)
        ensemble_calls = [c for c in calls if c[1] == "ensemble.py"]
        instrumental_call = next(
            c for c in ensemble_calls if "instrumental.wav" in c[c.index("--files") + 1]
        )
        vocals_call = next(
            c for c in ensemble_calls if "Vocals.wav" in c[c.index("--files") + 1]
        )
        assert all("instrumental.wav" in f for f in instrumental_call[
            instrumental_call.index("--files") + 1 : instrumental_call.index("--files") + 4
        ])
        assert all("Vocals.wav" in f for f in vocals_call[
            vocals_call.index("--files") + 1 : vocals_call.index("--files") + 4
        ])

    def test_returns_vocals_then_instrumental(
        self, tmp_path: Path, monkeypatch: pytest.MonkeyPatch
    ) -> None:
        _calls, vocals_out, instrumental_out = self._run_separate_song(monkeypatch, tmp_path)
        assert vocals_out.name == "vocals.wav"
        assert instrumental_out.name == "instrumental.wav"
        assert vocals_out.exists()
        assert instrumental_out.exists()

    def test_raises_when_expected_msst_output_missing(
        self, tmp_path: Path, monkeypatch: pytest.MonkeyPatch
    ) -> None:
        """If MSST doesn't produce the expected stem file (e.g. a config/CLI
        drift), fail loudly instead of silently ensembling a missing input."""

        def fake_run(cmd: list[str], **kwargs: object) -> subprocess.CompletedProcess:
            # Deliberately don't create any output files.
            return subprocess.CompletedProcess(cmd, 0, stdout="", stderr="")

        monkeypatch.setattr(delyric.subprocess, "run", fake_run)
        monkeypatch.setattr(delyric, "MSST_DIR", tmp_path / "msst")
        monkeypatch.setattr(delyric, "MSST_MODEL_PATHS", self._fake_model_paths(tmp_path))

        song_dir = tmp_path / "song"
        song_dir.mkdir()
        (song_dir / "audio.webm").write_bytes(b"fake audio")
        work_dir = tmp_path / "work"
        work_dir.mkdir()

        with pytest.raises(RuntimeError, match="Expected MSST output"):
            separate_song(song_dir, work_dir)


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


class TestRequirementsDeclareDirectImports:
    """Every top-level third-party import in delyric.py must be declared in
    requirements.txt. Previously click/tqdm arrived transitively via
    audio-separator[gpu]; the MSST rewrite dropped that dependency, so a
    direct `import click` / `from tqdm import tqdm` now needs a direct
    requirement, or a fresh venv install will ImportError at startup."""

    @staticmethod
    def _third_party_imports(source: str) -> set[str]:
        tree = ast.parse(source)
        imported: set[str] = set()
        for node in ast.walk(tree):
            if isinstance(node, ast.Import):
                for alias in node.names:
                    imported.add(alias.name.split(".")[0])
            elif isinstance(node, ast.ImportFrom):
                if node.module is not None and node.level == 0:
                    imported.add(node.module.split(".")[0])
        return imported - set(sys.stdlib_module_names)

    @staticmethod
    def _declared_requirements(requirements_text: str) -> set[str]:
        declared: set[str] = set()
        for line in requirements_text.splitlines():
            line = line.split("#", 1)[0].strip()
            if not line:
                continue
            name = re.split(r"[<>=\[\s]", line, maxsplit=1)[0].strip()
            if name:
                declared.add(name.lower())
        return declared

    def test_delyric_third_party_imports_are_all_declared(self) -> None:
        delyric_source = Path(delyric.__file__).read_text(encoding="utf-8")
        requirements_text = (Path(__file__).parent / "requirements.txt").read_text(
            encoding="utf-8"
        )

        third_party = self._third_party_imports(delyric_source)
        declared = self._declared_requirements(requirements_text)

        missing = {name for name in third_party if name.lower() not in declared}
        assert not missing, (
            f"delyric.py imports {sorted(missing)} directly, but "
            "requirements.txt does not declare them"
        )
