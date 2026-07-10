"""Tests for runpod/supervisor.py's pure/local logic: catalog scanning,
create-pod payload construction, the local song-list writer, and the local
song.txt tag pass.

Pod lifecycle (create/terminate/SSH/rsync against the real RunPod API) is
deliberately NOT tested here — per the task brief, no RunPod pod may be
created during this work; that plumbing is validated by a separate, later,
explicitly-gated live demo. See supervisor.py's module docstring.
"""

import shutil
import subprocess
from pathlib import Path

import pytest

import delyric
from runpod import supervisor

FFMPEG_AVAILABLE = shutil.which("ffmpeg") is not None and shutil.which("ffprobe") is not None


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


def _make_song(root: Path, name: str, *, with_outputs: bool = False) -> Path:
    d = root / name
    d.mkdir()
    (d / "audio.webm").write_bytes(b"fake audio")
    (d / "song.txt").write_text(
        "#ARTIST:Test\n#TITLE:Test\n#MP3:audio.webm\n", encoding="utf-8"
    )
    if with_outputs:
        (d / "instrumental.webm").write_bytes(b"fake instrumental")
        (d / "vocals.webm").write_bytes(b"fake vocals")
    return d


class TestSongsMissingOutputs:
    def test_lists_only_unprocessed_songs(self, tmp_path: Path) -> None:
        _make_song(tmp_path, "Done Song", with_outputs=True)
        _make_song(tmp_path, "Missing Song", with_outputs=False)

        missing = supervisor.songs_missing_outputs(tmp_path)
        assert missing == ["Missing Song"]

    def test_empty_when_all_processed(self, tmp_path: Path) -> None:
        _make_song(tmp_path, "A", with_outputs=True)
        _make_song(tmp_path, "B", with_outputs=True)
        assert supervisor.songs_missing_outputs(tmp_path) == []


@pytest.mark.skipif(not FFMPEG_AVAILABLE, reason="ffmpeg/ffprobe not on PATH")
class TestSongsNeedingReprocessing:
    def test_flags_missing_outputs(self, tmp_path: Path) -> None:
        song_dir = _make_song(tmp_path, "No Outputs")
        _make_clip(song_dir / "audio.webm", 5.0)

        needing = supervisor.songs_needing_reprocessing(tmp_path)
        assert needing == ["No Outputs"]

    def test_flags_corrupt_outputs_not_just_missing(self, tmp_path: Path) -> None:
        song_dir = _make_song(tmp_path, "Corrupt Outputs")
        _make_clip(song_dir / "audio.webm", 10.0)
        _make_clip(song_dir / "instrumental.webm", 1.0)  # too short vs. audio
        _make_clip(song_dir / "vocals.webm", 10.0)

        needing = supervisor.songs_needing_reprocessing(tmp_path)
        assert needing == ["Corrupt Outputs"]

    def test_skips_valid_songs(self, tmp_path: Path) -> None:
        song_dir = _make_song(tmp_path, "Valid Song")
        _make_clip(song_dir / "audio.webm", 5.0)
        _make_clip(song_dir / "instrumental.webm", 5.0)
        _make_clip(song_dir / "vocals.webm", 5.0)

        assert supervisor.songs_needing_reprocessing(tmp_path) == []


class TestBuildCreatePodPayload:
    def test_basic_fields_pass_through(self) -> None:
        payload = supervisor.build_create_pod_payload(
            pod_name="backfill-shard-0",
            image="ghcr.io/joshsymonds/sound-stage-backfill:latest",
            shard_song_count=100,
            env={"RUNPOD_JOB": "python -m runpod.shard_runner"},
        )
        assert payload["name"] == "backfill-shard-0"
        assert payload["imageName"] == "ghcr.io/joshsymonds/sound-stage-backfill:latest"
        assert payload["cloudType"] == "COMMUNITY"
        assert payload["gpuTypeIds"] == [supervisor.DEFAULT_GPU_TYPE_ID]
        assert payload["gpuCount"] == 1
        assert payload["env"] == {"RUNPOD_JOB": "python -m runpod.shard_runner"}

    def test_bandwidth_and_cuda_filters_present(self) -> None:
        payload = supervisor.build_create_pod_payload(
            pod_name="p", image="img", shard_song_count=1, env={},
        )
        assert payload["minDownloadMbps"] == supervisor.DEFAULT_MIN_DOWNLOAD_MBPS
        assert payload["minUploadMbps"] == supervisor.DEFAULT_MIN_UPLOAD_MBPS
        assert payload["allowedCudaVersions"] == supervisor.DEFAULT_ALLOWED_CUDA_VERSIONS

    def test_disk_size_scales_with_shard_song_count(self) -> None:
        small = supervisor.build_create_pod_payload(
            pod_name="p", image="img", shard_song_count=1, env={},
        )
        large = supervisor.build_create_pod_payload(
            pod_name="p", image="img", shard_song_count=2000, env={},
        )
        assert large["containerDiskInGb"] > small["containerDiskInGb"]

    def test_disk_size_floored_at_default_for_small_shards(self) -> None:
        payload = supervisor.build_create_pod_payload(
            pod_name="p", image="img", shard_song_count=1, env={},
        )
        assert payload["containerDiskInGb"] >= supervisor.DEFAULT_CONTAINER_DISK_GB

    def test_custom_gpu_type_and_cuda_versions_override_defaults(self) -> None:
        payload = supervisor.build_create_pod_payload(
            pod_name="p", image="img", shard_song_count=1, env={},
            gpu_type_id="NVIDIA GeForce RTX 3090",
            allowed_cuda_versions=["12.8", "13.0"],
        )
        assert payload["gpuTypeIds"] == ["NVIDIA GeForce RTX 3090"]
        assert payload["allowedCudaVersions"] == ["12.8", "13.0"]


class TestWriteShardSongList:
    def test_writes_one_name_per_line(self, tmp_path: Path) -> None:
        list_path = tmp_path / "songs.txt"
        supervisor.write_shard_song_list(list_path, ["Song A", "Song B"])
        assert list_path.read_text(encoding="utf-8").splitlines() == ["Song A", "Song B"]

    def test_creates_parent_directories(self, tmp_path: Path) -> None:
        list_path = tmp_path / "nested" / "dir" / "songs.txt"
        supervisor.write_shard_song_list(list_path, ["Song A"])
        assert list_path.exists()


class TestStageShardDir:
    def test_copies_audio_webm_for_each_song(self, tmp_path: Path) -> None:
        library_dir = tmp_path / "library"
        library_dir.mkdir()
        song_dir = _make_song(library_dir, "Song A")
        staging_dir = tmp_path / "staging"

        supervisor.stage_shard_dir(library_dir, staging_dir, ["Song A"])

        staged_audio = staging_dir / "Song A" / "audio.webm"
        assert staged_audio.exists()
        assert staged_audio.read_bytes() == (song_dir / "audio.webm").read_bytes()

    def test_does_not_overwrite_existing_staged_copy(self, tmp_path: Path) -> None:
        library_dir = tmp_path / "library"
        library_dir.mkdir()
        _make_song(library_dir, "Song A")
        staging_dir = tmp_path / "staging"

        supervisor.stage_shard_dir(library_dir, staging_dir, ["Song A"])
        # Simulate a done-marker left by a previous attempt.
        (staging_dir / "Song A" / ".delyric-done").write_text("")
        supervisor.stage_shard_dir(library_dir, staging_dir, ["Song A"])

        assert (staging_dir / "Song A" / ".delyric-done").exists()

    def test_skips_song_missing_from_library(self, tmp_path: Path) -> None:
        library_dir = tmp_path / "library"
        library_dir.mkdir()
        staging_dir = tmp_path / "staging"

        supervisor.stage_shard_dir(library_dir, staging_dir, ["Ghost Song"])

        assert not (staging_dir / "Ghost Song" / "audio.webm").exists()


class TestRemainingSongsInShard:
    def test_excludes_songs_with_done_marker(self, tmp_path: Path) -> None:
        staging_dir = tmp_path / "staging"
        (staging_dir / "Done Song").mkdir(parents=True)
        (staging_dir / "Done Song" / ".delyric-done").write_text("")
        (staging_dir / "Pending Song").mkdir(parents=True)

        remaining = supervisor.remaining_songs_in_shard(
            staging_dir, ["Done Song", "Pending Song"]
        )
        assert remaining == ["Pending Song"]

    def test_all_remaining_when_no_markers(self, tmp_path: Path) -> None:
        staging_dir = tmp_path / "staging"
        (staging_dir / "A").mkdir(parents=True)
        (staging_dir / "B").mkdir(parents=True)

        assert supervisor.remaining_songs_in_shard(staging_dir, ["A", "B"]) == ["A", "B"]

    def test_empty_when_all_done(self, tmp_path: Path) -> None:
        staging_dir = tmp_path / "staging"
        (staging_dir / "A").mkdir(parents=True)
        (staging_dir / "A" / ".delyric-done").write_text("")

        assert supervisor.remaining_songs_in_shard(staging_dir, ["A"]) == []


class TestRunLocalTagPass:
    def test_tags_only_songs_with_outputs_present(self, tmp_path: Path) -> None:
        _make_song(tmp_path, "Processed", with_outputs=True)
        _make_song(tmp_path, "Unprocessed", with_outputs=False)

        tagged = supervisor.run_local_tag_pass(tmp_path)

        assert tagged == 1
        processed_txt = (tmp_path / "Processed" / "song.txt").read_text(encoding="utf-8")
        assert "#INSTRUMENTAL:instrumental.webm" in processed_txt
        assert "#VOCALS:vocals.webm" in processed_txt
        unprocessed_txt = (tmp_path / "Unprocessed" / "song.txt").read_text(encoding="utf-8")
        assert "#INSTRUMENTAL:" not in unprocessed_txt

    def test_idempotent_across_repeated_passes(self, tmp_path: Path) -> None:
        _make_song(tmp_path, "Processed", with_outputs=True)
        supervisor.run_local_tag_pass(tmp_path)
        first = (tmp_path / "Processed" / "song.txt").read_text(encoding="utf-8")
        supervisor.run_local_tag_pass(tmp_path)
        second = (tmp_path / "Processed" / "song.txt").read_text(encoding="utf-8")
        assert first == second

    def test_returns_zero_for_empty_library(self, tmp_path: Path) -> None:
        assert supervisor.run_local_tag_pass(tmp_path) == 0
