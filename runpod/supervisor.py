#!/usr/bin/env python3
"""Supervisor for the RunPod backfill: shards the sound-stage library across
N community-cloud RTX 4090 pods, launches/monitors/relaunches them, and pulls
validated outputs back for a local song.txt tag pass.

Runs on gnomon (not in the pod image). Talks to the real RunPod REST API
(pod creation — supports the minDownloadMbps/minUploadMbps/allowedCudaVersions
filters natively) and the RunPod GraphQL API (runtime port polling, mirroring
backlot's scripts/runpod-sync.sh get_ssh_info — the REST API's port/IP detail
is thinner than GraphQL's `runtime.ports` shape). Uses stdlib urllib rather
than adding a requests dependency.

HARD GATE: this module must never create a live pod during this project's
development — see the epic. Its pod-lifecycle functions (create_pod,
get_pod_runtime_ports, terminate_pod, rsync_push/pull, tail_shard_log) are
therefore deliberately NOT unit tested; per the task brief, "no RunPod API
mocking theater — pod lifecycle is validated by the (gated, later) live
demo." Only the pure/local logic below (catalog scanning, payload
construction, the song-list writer, and the local tag pass) is covered by
supervisor_test.py. Sharding, spend-guard, and stall-detection logic live in
sibling modules (sharding.py, budget.py, stall.py) with their own tests.
"""

import concurrent.futures
import json
import logging
import os
import shutil
import subprocess
import threading
import time
import urllib.request
from dataclasses import dataclass
from pathlib import Path

import click

import delyric
from runpod.budget import SpendGuard, pod_hours_elapsed
from runpod.sharding import shard_songs
from runpod.stall import DEFAULT_STALL_THRESHOLD_SECONDS, is_stalled
from runpod.validation import is_done, validate_song_outputs

logger = logging.getLogger("supervisor")

RUNPOD_REST_URL = "https://rest.runpod.io/v1/pods"
RUNPOD_GRAPHQL_URL = "https://api.runpod.io/graphql"

DEFAULT_GPU_TYPE_ID = "NVIDIA GeForce RTX 4090"
# The image's base is runpod/pytorch:*-cu1300-torch291-* with requirements.txt
# then pulling audio-separator[gpu]==0.44.3's own torch 2.13.0+cu130 wheel —
# the workload needs a host driver reporting CUDA 12.4+ for cu130 forward
# compatibility. allowedCudaVersions filters on the *driver's* reported CUDA
# version, not the workload's build, so this floor is conservative rather
# than an exact match.
DEFAULT_ALLOWED_CUDA_VERSIONS = ["12.4", "12.5", "12.6", "12.7", "12.8", "12.9", "13.0"]
DEFAULT_MIN_DOWNLOAD_MBPS = 500
DEFAULT_MIN_UPLOAD_MBPS = 500
DEFAULT_CONTAINER_DISK_GB = 40
# ~15MB audio.webm + ~150MB peak WAV temps per song (3-model ensemble stems
# at a few minutes of 44.1kHz stereo), rounded up with headroom.
DISK_GB_PER_SONG = 0.2

DEFAULT_PER_POD_TIMEOUT_SECONDS = 4 * 3600
SHARD_WORKSPACE_PATH = "/workspace/shard"
SONG_LIST_FILENAME = "songs.txt"
SHARD_LOG_FILENAME = "shard.log"


# ── Pure / local logic — unit tested in supervisor_test.py ──────────────────


def songs_missing_outputs(library_dir: Path) -> list[str]:
    """Fast pre-shard scan: songs entirely lacking separation outputs. Used
    to build the initial song list for sharding — cheaper than full
    ffprobe-based validation, appropriate for a one-time scan of the whole
    2,646-song catalog before any pod work has happened.
    """
    return [d.name for d in delyric.find_song_dirs(library_dir) if not delyric.is_processed(d)]


def songs_needing_reprocessing(
    library_dir: Path,
    tolerance_seconds: float = 2.0,
) -> list[str]:
    """Full validation scan for the final sweep pass: catches songs with
    missing OR invalid (truncated/corrupt) outputs, not just absent files —
    this is what distinguishes the sweep from the initial fast scan.
    """
    needing = []
    for song_dir in delyric.find_song_dirs(library_dir):
        ok, _ = validate_song_outputs(song_dir, tolerance_seconds)
        if not ok:
            needing.append(song_dir.name)
    return needing


def build_create_pod_payload(
    pod_name: str,
    image: str,
    shard_song_count: int,
    env: dict[str, str],
    gpu_type_id: str = DEFAULT_GPU_TYPE_ID,
    allowed_cuda_versions: list[str] | None = None,
    min_download_mbps: int = DEFAULT_MIN_DOWNLOAD_MBPS,
    min_upload_mbps: int = DEFAULT_MIN_UPLOAD_MBPS,
) -> dict:
    """Pure builder for the RunPod REST create-pod payload — no network call.
    See create_pod() below for the actual POST.
    """
    disk_gb = max(DEFAULT_CONTAINER_DISK_GB, int(shard_song_count * DISK_GB_PER_SONG) + 10)

    return {
        "name": pod_name,
        "imageName": image,
        "cloudType": "COMMUNITY",
        "gpuTypeIds": [gpu_type_id],
        "gpuCount": 1,
        "containerDiskInGb": disk_gb,
        "minDownloadMbps": min_download_mbps,
        "minUploadMbps": min_upload_mbps,
        "allowedCudaVersions": allowed_cuda_versions or DEFAULT_ALLOWED_CUDA_VERSIONS,
        "ports": ["22/tcp"],
        "supportPublicIp": True,
        "env": env,
    }


def write_shard_song_list(list_path: Path, song_names: list[str]) -> None:
    """Write a shard's song list file (shard_runner.read_song_list's input)."""
    list_path.parent.mkdir(parents=True, exist_ok=True)
    list_path.write_text("\n".join(song_names) + "\n" if song_names else "", encoding="utf-8")


def stage_shard_dir(library_dir: Path, staging_dir: Path, song_names: list[str]) -> None:
    """Prepare a local directory tree mirroring the pod's /workspace/shard
    layout: one subdir per song holding a copy of audio.webm. Skips songs
    that already have a local copy — a relaunch calls this again with a
    shrunk song list, and any done-marker from the previous attempt (pulled
    back by rsync_pull) is left in place so the new pod's shard runner still
    sees it and skips re-processing.
    """
    staging_dir.mkdir(parents=True, exist_ok=True)
    for name in song_names:
        dest_dir = staging_dir / name
        dest_dir.mkdir(parents=True, exist_ok=True)
        src_audio = library_dir / name / "audio.webm"
        dest_audio = dest_dir / "audio.webm"
        if src_audio.exists() and not dest_audio.exists():
            shutil.copy2(src_audio, dest_audio)


def remaining_songs_in_shard(staging_dir: Path, song_names: list[str]) -> list[str]:
    """After pulling partial results back from a stalled/timed-out pod,
    return the subset of song_names that still need processing — i.e. have
    no done-marker (written only once validate_song_outputs passed).
    """
    return [name for name in song_names if not is_done(staging_dir / name)]


def run_local_tag_pass(library_dir: Path) -> int:
    """After pulling shard outputs back into the library, add #INSTRUMENTAL:/
    #VOCALS: tags to every song.txt whose separation outputs are now present.
    Returns the count of songs tagged. Idempotent — delyric.update_song_txt
    is itself a no-op once tags already exist.
    """
    tagged = 0
    for song_dir in delyric.find_song_dirs(library_dir):
        if delyric.is_processed(song_dir):
            delyric.update_song_txt(song_dir)
            tagged += 1
    return tagged


# ── Pod-lifecycle plumbing — NOT unit tested; see module docstring ──────────


@dataclass
class PodHandle:
    """Tracks one active pod's shard assignment and progress for the
    monitor loop's stall detection and relaunch decisions."""

    pod_id: str
    shard_index: int
    song_names: list[str]
    started_at: float
    last_progress_at: float
    ssh_host: str | None = None
    ssh_port: int | None = None


def _api_headers(api_key: str) -> dict[str, str]:
    return {"Authorization": f"Bearer {api_key}", "Content-Type": "application/json"}


def create_pod(api_key: str, payload: dict) -> dict:
    """POST a pod-creation request to the RunPod REST API. Mutating —
    never invoked by tests; see module docstring's hard gate.
    """
    req = urllib.request.Request(
        RUNPOD_REST_URL,
        data=json.dumps(payload).encode("utf-8"),
        headers=_api_headers(api_key),
        method="POST",
    )
    with urllib.request.urlopen(req, timeout=30) as resp:
        return json.loads(resp.read())


def terminate_pod(api_key: str, pod_id: str) -> None:
    """DELETE a pod via the RunPod REST API. Mutating — never invoked by
    tests; see module docstring's hard gate.
    """
    req = urllib.request.Request(
        f"{RUNPOD_REST_URL}/{pod_id}",
        headers=_api_headers(api_key),
        method="DELETE",
    )
    urllib.request.urlopen(req, timeout=30)


def get_pod_runtime_ports(api_key: str, pod_id: str) -> dict | None:
    """Poll the RunPod GraphQL API for a pod's public SSH ip/port, mirroring
    backlot's scripts/runpod-sync.sh get_ssh_info. Returns
    {"ip": ..., "port": ...} once the runtime's port 22 mapping is public, or
    None if not ready yet. Not unit tested — real network call.
    """
    query = (
        "{ pod(input: { podId: \"%s\" }) { runtime { ports "
        "{ ip isIpPublic privatePort publicPort } } } }" % pod_id
    )
    req = urllib.request.Request(
        RUNPOD_GRAPHQL_URL,
        data=json.dumps({"query": query}).encode("utf-8"),
        headers=_api_headers(api_key),
        method="POST",
    )
    with urllib.request.urlopen(req, timeout=30) as resp:
        body = json.loads(resp.read())

    runtime = (body.get("data") or {}).get("pod", {}).get("runtime")
    if not runtime:
        return None
    for port in runtime.get("ports") or []:
        if port.get("privatePort") == 22 and port.get("isIpPublic"):
            return {"ip": port["ip"], "port": port["publicPort"]}
    return None


def rsync_push(local_dir: Path, ssh_host: str, ssh_port: int, ssh_key: Path) -> None:
    """Push a shard's audio.webm files up to the pod. Not unit tested —
    shells out to the real rsync/ssh binaries against a live pod.
    """
    ssh_opts = f"ssh -o StrictHostKeyChecking=no -i {ssh_key} -p {ssh_port}"
    subprocess.run(
        ["rsync", "-az", "-e", ssh_opts, f"{local_dir}/", f"root@{ssh_host}:{SHARD_WORKSPACE_PATH}/"],
        check=True,
    )


def rsync_pull(local_dir: Path, ssh_host: str, ssh_port: int, ssh_key: Path) -> None:
    """Pull validated shard outputs back from the pod. Not unit tested —
    shells out to the real rsync/ssh binaries against a live pod.
    """
    ssh_opts = f"ssh -o StrictHostKeyChecking=no -i {ssh_key} -p {ssh_port}"
    local_dir.mkdir(parents=True, exist_ok=True)
    subprocess.run(
        ["rsync", "-az", "-e", ssh_opts, f"root@{ssh_host}:{SHARD_WORKSPACE_PATH}/", f"{local_dir}/"],
        check=True,
    )


def tail_shard_log_last_progress(ssh_host: str, ssh_port: int, ssh_key: Path) -> float | None:
    """SSH-tail the shard runner's stdout log on the pod and return the unix
    timestamp of the most recent "[YYYY-MM-DDTHH:MM:SS] DONE/FAILED <song>"
    progress line, or None if no progress line exists yet. Not unit tested —
    real SSH call against a live pod.
    """
    result = subprocess.run(
        [
            "ssh", "-o", "StrictHostKeyChecking=no", "-i", str(ssh_key), "-p", str(ssh_port),
            f"root@{ssh_host}", f"tail -n 1 {SHARD_WORKSPACE_PATH}/{SHARD_LOG_FILENAME}",
        ],
        capture_output=True, text=True, timeout=30, check=False,
    )
    line = result.stdout.strip()
    if not line or not line.startswith("["):
        return None
    ts_str = line[1:line.index("]")]
    return time.mktime(time.strptime(ts_str, "%Y-%m-%dT%H:%M:%S"))


def wait_for_pod_running(api_key: str, pod_id: str, max_wait_seconds: float = 900) -> None:
    """Poll REST GET /pods/{id} until desiredStatus/status is RUNNING.
    Mirrors backlot's runpod-sync.sh wait_for_running. Not unit tested —
    real network call against a live pod.
    """
    deadline = time.monotonic() + max_wait_seconds
    while time.monotonic() < deadline:
        req = urllib.request.Request(
            f"{RUNPOD_REST_URL}/{pod_id}", headers=_api_headers(api_key), method="GET"
        )
        with urllib.request.urlopen(req, timeout=30) as resp:
            body = json.loads(resp.read())
        if body.get("desiredStatus") == "RUNNING" or body.get("status") == "RUNNING":
            return
        time.sleep(10)
    raise TimeoutError(f"pod {pod_id} did not reach RUNNING within {max_wait_seconds}s")


def wait_for_ssh_ready(ssh_host: str, ssh_port: int, ssh_key: Path, max_wait_seconds: float = 300) -> None:
    """Poll real SSH connectivity. Mirrors backlot's wait_for_ssh. Not unit
    tested — real network call against a live pod."""
    deadline = time.monotonic() + max_wait_seconds
    while time.monotonic() < deadline:
        result = subprocess.run(
            [
                "ssh", "-o", "ConnectTimeout=5", "-o", "BatchMode=yes",
                "-o", "StrictHostKeyChecking=no", "-i", str(ssh_key), "-p", str(ssh_port),
                f"root@{ssh_host}", "true",
            ],
            capture_output=True, timeout=10, check=False,
        )
        if result.returncode == 0:
            return
        time.sleep(10)
    raise TimeoutError(f"SSH to {ssh_host}:{ssh_port} not ready within {max_wait_seconds}s")


def is_pod_finished(api_key: str, pod_id: str) -> bool:
    """True once a RUNPOD_JOB-mode pod has self-exited (billing stopped).
    Not unit tested — real network call against a live pod."""
    req = urllib.request.Request(
        f"{RUNPOD_REST_URL}/{pod_id}", headers=_api_headers(api_key), method="GET"
    )
    with urllib.request.urlopen(req, timeout=30) as resp:
        body = json.loads(resp.read())
    status = body.get("desiredStatus") or body.get("status")
    return status in ("EXITED", "TERMINATED", "DEAD")


def launch_pod_for_shard(
    api_key: str,
    image: str,
    shard_index: int,
    remaining_songs: list[str],
    ssh_key: Path,
    staging_dir: Path,
    per_pod_timeout_seconds: float,
) -> PodHandle:
    """Create a pod for a shard (or its relaunch), wait for it to come up,
    push the staged shard directory, and return a handle for monitoring.
    Not unit tested — orchestrates several real network/SSH/rsync calls
    against a live pod; see module docstring's hard gate.
    """
    pod_name = f"sound-stage-backfill-shard-{shard_index}"
    payload = build_create_pod_payload(
        pod_name=pod_name,
        image=image,
        shard_song_count=len(remaining_songs),
        env={
            "RUNPOD_JOB": (
                f"python -m runpod.shard_runner --shard-dir {SHARD_WORKSPACE_PATH} "
                f"--song-list {SHARD_WORKSPACE_PATH}/{SONG_LIST_FILENAME} "
                f"2>&1 | tee {SHARD_WORKSPACE_PATH}/{SHARD_LOG_FILENAME}"
            ),
            "RUNPOD_TIMEOUT": str(int(per_pod_timeout_seconds)),
        },
    )
    pod = create_pod(api_key, payload)
    pod_id = pod["id"]
    wait_for_pod_running(api_key, pod_id)

    ssh_info = None
    deadline = time.monotonic() + 900
    while ssh_info is None and time.monotonic() < deadline:
        ssh_info = get_pod_runtime_ports(api_key, pod_id)
        if ssh_info is None:
            time.sleep(10)
    if ssh_info is None:
        raise TimeoutError(f"pod {pod_id} never exposed a public SSH port")

    wait_for_ssh_ready(ssh_info["ip"], ssh_info["port"], ssh_key)

    write_shard_song_list(staging_dir / SONG_LIST_FILENAME, remaining_songs)
    rsync_push(staging_dir, ssh_info["ip"], ssh_info["port"], ssh_key)

    now = time.time()
    return PodHandle(
        pod_id=pod_id,
        shard_index=shard_index,
        song_names=remaining_songs,
        started_at=now,
        last_progress_at=now,
        ssh_host=ssh_info["ip"],
        ssh_port=ssh_info["port"],
    )


def monitor_shard_pod(
    api_key: str,
    handle: PodHandle,
    ssh_key: Path,
    per_pod_timeout_seconds: float,
    stall_threshold_seconds: float,
    poll_interval_seconds: float = 30,
) -> str:
    """Poll a shard's pod until it finishes, stalls, or times out. Returns
    "completed", "stalled", or "timed_out". Not unit tested — real SSH/API
    polling against a live pod.
    """
    while True:
        now = time.time()
        if now - handle.started_at > per_pod_timeout_seconds:
            return "timed_out"

        progress_ts = tail_shard_log_last_progress(handle.ssh_host, handle.ssh_port, ssh_key)
        if progress_ts is not None:
            handle.last_progress_at = progress_ts

        if is_pod_finished(api_key, handle.pod_id):
            return "completed"

        if is_stalled(handle.last_progress_at, now, stall_threshold_seconds):
            return "stalled"

        time.sleep(poll_interval_seconds)


# ── Orchestration ────────────────────────────────────────────────────────


def run_backfill(
    api_key: str,
    library_dir: Path,
    image: str,
    n_shards: int,
    max_concurrent_pods: int,
    max_pod_hours: float,
    ssh_key: Path,
    staging_root: Path,
    per_pod_timeout_seconds: float = DEFAULT_PER_POD_TIMEOUT_SECONDS,
    stall_threshold_seconds: float = DEFAULT_STALL_THRESHOLD_SECONDS,
    sweep: bool = False,
) -> None:
    """Run (or sweep-repair) the backfill across n_shards community pods.

    Each shard runs in its own worker (a thread pool caps concurrency at
    max_concurrent_pods) and keeps relaunching on a fresh pod — pulling
    partial results and re-staging only the songs still missing a
    done-marker — until the shard completes, or the spend guard refuses
    another launch.

    Not unit tested: this is the live orchestration loop against the real
    RunPod API and real pods, gated off entirely during development per the
    epic's hard gate. See module docstring.
    """
    guard = SpendGuard(max_concurrent_pods=max_concurrent_pods, max_pod_hours=max_pod_hours)

    song_names = (
        songs_needing_reprocessing(library_dir) if sweep else songs_missing_outputs(library_dir)
    )
    if not song_names:
        logger.info("Nothing to process (sweep=%s).", sweep)
        return

    shards = [s for s in shard_songs(song_names, n_shards) if s]

    spend_lock = threading.Lock()
    spend_state = {"active_pods": 0, "spent_pod_hours": 0.0}

    def process_shard(shard_index: int, songs: list[str]) -> None:
        remaining = list(songs)
        staging_dir = staging_root / f"shard-{shard_index}"

        while remaining:
            with spend_lock:
                refusal = guard.refusal_reason(
                    spend_state["active_pods"], spend_state["spent_pod_hours"]
                )
                if refusal:
                    logger.error(
                        "%s — abandoning shard %d with %d songs remaining",
                        refusal, shard_index, len(remaining),
                    )
                    return
                spend_state["active_pods"] += 1

            stage_shard_dir(library_dir, staging_dir, remaining)
            handle: PodHandle | None = None
            try:
                handle = launch_pod_for_shard(
                    api_key, image, shard_index, remaining, ssh_key,
                    staging_dir, per_pod_timeout_seconds,
                )
                outcome = monitor_shard_pod(
                    api_key, handle, ssh_key, per_pod_timeout_seconds, stall_threshold_seconds
                )
                rsync_pull(staging_dir, handle.ssh_host, handle.ssh_port, ssh_key)
                terminate_pod(api_key, handle.pod_id)
            finally:
                with spend_lock:
                    spend_state["active_pods"] -= 1
                    if handle is not None:
                        spend_state["spent_pod_hours"] += pod_hours_elapsed(
                            handle.started_at, time.time()
                        )

            if outcome == "completed":
                logger.info("shard %d completed", shard_index)
                return

            logger.warning("shard %d pod %s (outcome=%s); relaunching", shard_index, handle.pod_id, outcome)
            remaining = remaining_songs_in_shard(staging_dir, remaining)

    with concurrent.futures.ThreadPoolExecutor(max_workers=max_concurrent_pods) as pool:
        futures = [pool.submit(process_shard, i, s) for i, s in enumerate(shards)]
        for f in futures:
            f.result()

    tagged = run_local_tag_pass(library_dir)
    logger.info("Tag pass complete: %d songs tagged.", tagged)


@click.command()
@click.option("--library-dir", required=True, type=click.Path(exists=True, file_okay=False, path_type=Path))
@click.option("--image", required=True, help="Docker image to run on each pod.")
@click.option("--n-shards", required=True, type=int)
@click.option("--max-concurrent-pods", default=4, show_default=True, type=int)
@click.option("--max-pod-hours", default=200.0, show_default=True, type=float)
@click.option("--ssh-key", required=True, type=click.Path(exists=True, dir_okay=False, path_type=Path))
@click.option(
    "--staging-root", required=True, type=click.Path(file_okay=False, path_type=Path),
    help="Local directory for per-shard staging trees (audio.webm copies + pulled-back outputs).",
)
@click.option("--per-pod-timeout-seconds", default=DEFAULT_PER_POD_TIMEOUT_SECONDS, show_default=True, type=float)
@click.option("--stall-threshold-seconds", default=DEFAULT_STALL_THRESHOLD_SECONDS, show_default=True, type=float)
@click.option("--sweep", is_flag=True, help="Re-run only songs with missing/invalid outputs.")
@click.option("--api-key-env", default="RUNPOD_API_KEY", show_default=True)
def main(
    library_dir: Path,
    image: str,
    n_shards: int,
    max_concurrent_pods: int,
    max_pod_hours: float,
    ssh_key: Path,
    staging_root: Path,
    per_pod_timeout_seconds: float,
    stall_threshold_seconds: float,
    sweep: bool,
    api_key_env: str,
) -> None:
    """Shard the sound-stage library's backfill across N community-cloud
    pods, monitor and relaunch stalled/timed-out pods, and locally tag
    song.txt once outputs are pulled back."""
    api_key = os.environ.get(api_key_env)
    if not api_key:
        raise click.ClickException(f"{api_key_env} is not set")

    logging.basicConfig(level=logging.INFO, format="%(asctime)s %(levelname)s %(message)s")
    run_backfill(
        api_key=api_key,
        library_dir=library_dir,
        image=image,
        n_shards=n_shards,
        max_concurrent_pods=max_concurrent_pods,
        max_pod_hours=max_pod_hours,
        ssh_key=ssh_key,
        staging_root=staging_root,
        per_pod_timeout_seconds=per_pod_timeout_seconds,
        stall_threshold_seconds=stall_threshold_seconds,
        sweep=sweep,
    )


if __name__ == "__main__":
    main()
