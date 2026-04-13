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
from contextlib import asynccontextmanager
from dataclasses import dataclass
from pathlib import Path
from typing import AsyncIterator, Literal

from fastapi import FastAPI, HTTPException
from pydantic import BaseModel

import delyric

logger = logging.getLogger("delyric_worker")

JobStatus = Literal["queued", "processing", "complete", "failed"]


@dataclass
class JobState:
    job_id: str
    song_path: str
    status: JobStatus = "queued"
    error: str | None = None


class ProcessRequest(BaseModel):
    songPath: str


_jobs: dict[str, JobState] = {}
_jobs_lock = threading.Lock()
_queue: "queue.Queue[str]" = queue.Queue()
_worker_thread: threading.Thread | None = None
_shutdown = threading.Event()


def _library_root() -> Path:
    return Path(os.environ.get("DELYRIC_LIBRARY", "/mnt/music/sound-stage")).resolve()


def _validate_song_path(raw: str) -> Path:
    if not raw or not raw.startswith("/"):
        raise HTTPException(status_code=400, detail="songPath must be an absolute path")
    p = Path(raw).resolve()
    root = _library_root()
    try:
        p.relative_to(root)
    except ValueError as exc:
        raise HTTPException(status_code=400, detail=f"songPath must be inside {root}") from exc
    if not p.is_dir():
        raise HTTPException(status_code=400, detail=f"songPath is not a directory: {p}")
    if not (p / "audio.webm").exists():
        raise HTTPException(status_code=400, detail=f"no audio.webm in {p}")
    return p


def _worker_loop() -> None:
    while not _shutdown.is_set():
        try:
            job_id = _queue.get(timeout=0.1)
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
            except Exception as e:
                logger.exception("job %s failed", job_id)
                with _jobs_lock:
                    job.status = "failed"
                    job.error = str(e)
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
    global _worker_thread
    _shutdown.set()
    if _worker_thread is not None:
        _worker_thread.join(timeout=5)
        _worker_thread = None


def bind_host_available(host: str) -> bool:
    """Return True if we can actually bind a TCP socket to `host` locally."""
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
    app = FastAPI(lifespan=_lifespan)

    @app.get("/healthz")
    def healthz() -> dict[str, bool]:
        return {"ok": True}

    @app.post("/process")
    def process(req: ProcessRequest) -> dict[str, str]:
        song_dir = _validate_song_path(req.songPath)
        job_id = uuid.uuid4().hex
        with _jobs_lock:
            _jobs[job_id] = JobState(job_id=job_id, song_path=str(song_dir))
        _queue.put(job_id)
        return {"jobId": job_id}

    @app.get("/status/{job_id}")
    def status(job_id: str) -> dict[str, object]:
        with _jobs_lock:
            job = _jobs.get(job_id)
        if job is None:
            raise HTTPException(status_code=404, detail=f"unknown job: {job_id}")
        return {"status": job.status, "error": job.error}

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
