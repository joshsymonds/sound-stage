"""FastAPI HTTP wrapper around the delyric vocal separation pipeline.

Exposes POST /process, GET /status/{jobId}, GET /healthz. Jobs are processed
serially by a single background worker thread; state is kept in memory only.
"""

import logging
import os
import queue
import socket
import threading
import uuid
from collections import OrderedDict
from contextlib import asynccontextmanager
from dataclasses import dataclass
from pathlib import Path
from typing import AsyncIterator, Literal

from fastapi import FastAPI, HTTPException
from pydantic import BaseModel

import delyric

logger = logging.getLogger("delyric_worker")

JobStatus = Literal["queued", "processing", "complete", "failed"]

# Cap in-memory job history. Terminal jobs beyond this are evicted oldest-first
# so a long-running worker doesn't accumulate state forever.
MAX_JOB_HISTORY = 1000


@dataclass
class JobState:
    job_id: str
    song_path: str
    status: JobStatus = "queued"
    error: str | None = None


class ProcessRequest(BaseModel):
    songPath: str


class ProcessResponse(BaseModel):
    jobId: str


class StatusResponse(BaseModel):
    status: JobStatus
    error: str | None = None


class HealthResponse(BaseModel):
    ok: bool


_jobs: "OrderedDict[str, JobState]" = OrderedDict()
_jobs_lock = threading.Lock()
_queue: "queue.Queue[str]" = queue.Queue()
_worker_thread: threading.Thread | None = None
_shutdown = threading.Event()


def _reset_for_tests() -> None:
    """Clear module-level state. Test helper — not part of the public API."""
    global _worker_thread
    _shutdown.set()
    if _worker_thread is not None:
        _worker_thread.join(timeout=2)
    _worker_thread = None
    _shutdown.clear()
    with _jobs_lock:
        _jobs.clear()
    while not _queue.empty():
        try:
            _queue.get_nowait()
            _queue.task_done()
        except queue.Empty:
            break


def _evict_old_jobs_locked() -> None:
    """Drop oldest terminal jobs once history exceeds the cap. Caller holds _jobs_lock."""
    if len(_jobs) <= MAX_JOB_HISTORY:
        return
    over = len(_jobs) - MAX_JOB_HISTORY
    # Iterate insertion order; only evict finished jobs so in-flight state is preserved.
    for jid in list(_jobs):
        if over <= 0:
            break
        if _jobs[jid].status in ("complete", "failed"):
            del _jobs[jid]
            over -= 1


def _library_root() -> Path:
    return Path(os.environ.get("DELYRIC_LIBRARY", "/mnt/music/sound-stage")).resolve()


def _validate_song_path(raw: str) -> Path:
    """Resolve and validate a client-supplied song path.

    Error details returned to the client are intentionally generic — the full
    path and reason are logged server-side via the debug logger. The threat
    model assumes a network-trusted caller, but there is no benefit to echoing
    resolved filesystem paths back over the wire.
    """
    if not raw or not raw.startswith("/"):
        logger.debug("rejecting non-absolute songPath: %r", raw)
        raise HTTPException(status_code=400, detail="songPath must be an absolute path")
    p = Path(raw).resolve()
    root = _library_root()
    try:
        p.relative_to(root)
    except ValueError as exc:
        logger.debug("rejecting out-of-library songPath: %s (root=%s)", p, root)
        raise HTTPException(status_code=400, detail="songPath outside library") from exc
    if not p.is_dir():
        logger.debug("rejecting non-directory songPath: %s", p)
        raise HTTPException(status_code=400, detail="songPath is not a directory")
    if not (p / "audio.webm").exists():
        logger.debug("rejecting songPath without audio.webm: %s", p)
        raise HTTPException(status_code=400, detail="songPath missing audio.webm")
    return p


def _worker_loop() -> None:
    while not _shutdown.is_set():
        try:
            job_id = _queue.get(timeout=1.0)
        except queue.Empty:
            continue
        try:
            with _jobs_lock:
                job = _jobs.get(job_id)
            if job is None:
                continue
            with _jobs_lock:
                job.status = "processing"
            try:
                delyric.process_song(Path(job.song_path))
                with _jobs_lock:
                    job.status = "complete"
                    _evict_old_jobs_locked()
            except Exception as e:
                logger.exception("job %s failed", job_id)
                with _jobs_lock:
                    job.status = "failed"
                    job.error = f"{type(e).__name__}: {e}"
                    _evict_old_jobs_locked()
        finally:
            _queue.task_done()


def _start_worker() -> None:
    global _worker_thread
    _shutdown.clear()
    if _worker_thread is None or not _worker_thread.is_alive():
        _worker_thread = threading.Thread(
            target=_worker_loop, daemon=True, name="delyric-worker"
        )
        _worker_thread.start()


def _stop_worker() -> None:
    """Signal the worker to stop, mark any in-flight job failed, join briefly.

    The worker calls a blocking C extension for GPU separation, so it cannot
    observe _shutdown mid-job. The thread is a daemon and will die with the
    process. Marking a processing job "failed" keeps observable state consistent
    if shutdown races a status poll.
    """
    global _worker_thread
    _shutdown.set()
    with _jobs_lock:
        for job in _jobs.values():
            if job.status == "processing":
                job.status = "failed"
                job.error = "worker shutting down"
    if _worker_thread is not None:
        _worker_thread.join(timeout=5)
        _worker_thread = None


def bind_host_available(host: str) -> bool:
    """Return True if we can actually bind a TCP socket to `host` locally.

    Rejects empty/whitespace hosts explicitly so a misconfigured
    DELYRIC_BIND_HOST="" doesn't fall through to getaddrinfo("") wildcard.
    """
    if not host or not host.strip():
        return False
    try:
        infos = socket.getaddrinfo(host, 0, type=socket.SOCK_STREAM)
    except socket.gaierror:
        return False
    for family, socktype, proto, _canon, sockaddr in infos:
        try:
            s = socket.socket(family, socktype, proto)
        except OSError:
            continue
        try:
            s.bind(sockaddr)
            return True
        except OSError:
            continue
        finally:
            s.close()
    return False


@asynccontextmanager
async def _lifespan(_app: FastAPI) -> AsyncIterator[None]:
    _start_worker()
    try:
        yield
    finally:
        _stop_worker()


def create_app() -> FastAPI:
    # Docs/OpenAPI disabled: this is an internal three-endpoint API with no
    # external consumer of the schema. Reduces surface if the network boundary
    # is ever misconfigured.
    app = FastAPI(
        lifespan=_lifespan,
        docs_url=None,
        redoc_url=None,
        openapi_url=None,
    )

    @app.get("/healthz", response_model=HealthResponse)
    def healthz() -> HealthResponse:
        return HealthResponse(ok=True)

    @app.post("/process", response_model=ProcessResponse)
    def process(req: ProcessRequest) -> ProcessResponse:
        song_dir = _validate_song_path(req.songPath)
        job_id = uuid.uuid4().hex
        with _jobs_lock:
            _jobs[job_id] = JobState(job_id=job_id, song_path=str(song_dir))
        _queue.put(job_id)
        return ProcessResponse(jobId=job_id)

    @app.get("/status/{job_id}", response_model=StatusResponse)
    def status(job_id: str) -> StatusResponse:
        with _jobs_lock:
            job = _jobs.get(job_id)
        if job is None:
            raise HTTPException(status_code=404, detail="unknown job")
        return StatusResponse(status=job.status, error=job.error)

    return app


app = create_app()


def main() -> None:
    logging.basicConfig(level=logging.INFO)
    host = os.environ.get("DELYRIC_BIND_HOST", "127.0.0.1")
    port = int(os.environ.get("DELYRIC_PORT", "9001"))
    if not bind_host_available(host):
        logger.error(
            "DELYRIC_BIND_HOST=%r is not bindable on this host; refusing to start", host
        )
        raise SystemExit(0)
    import uvicorn

    uvicorn.run(app, host=host, port=port)


if __name__ == "__main__":
    main()
