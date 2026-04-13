"""Tests for delyric_worker.py — FastAPI HTTP wrapper around delyric.process_song."""

import threading
import time
from pathlib import Path
from typing import Iterator

import pytest
from fastapi.testclient import TestClient


@pytest.fixture(autouse=True)
def reset_state(tmp_path: Path, monkeypatch: pytest.MonkeyPatch) -> Iterator[Path]:
    """Reset module-level state and point DELYRIC_LIBRARY at an isolated tmp dir."""
    monkeypatch.setenv("DELYRIC_LIBRARY", str(tmp_path))
    import delyric_worker as dw

    with dw._jobs_lock:
        dw._jobs.clear()
    while not dw._queue.empty():
        try:
            dw._queue.get_nowait()
            dw._queue.task_done()
        except Exception:
            break
    yield tmp_path


@pytest.fixture
def client() -> Iterator[TestClient]:
    """Yield a TestClient with lifespan events active (worker thread running)."""
    import delyric_worker as dw

    with TestClient(dw.app) as c:
        yield c


def _make_song_dir(root: Path, name: str = "Artist - Title") -> Path:
    d = root / name
    d.mkdir(parents=True, exist_ok=True)
    (d / "audio.webm").write_bytes(b"fake audio")
    return d


def _wait_status(client: TestClient, job_id: str, terminal: set[str], timeout: float = 3.0) -> dict:
    deadline = time.monotonic() + timeout
    last: dict = {}
    while time.monotonic() < deadline:
        resp = client.get(f"/status/{job_id}")
        assert resp.status_code == 200
        last = resp.json()
        if last["status"] in terminal:
            return last
        time.sleep(0.02)
    raise AssertionError(f"job {job_id} did not reach {terminal}; last={last}")


class TestHealthz:
    def test_healthz_returns_ok(self, client: TestClient) -> None:
        resp = client.get("/healthz")
        assert resp.status_code == 200
        assert resp.json() == {"ok": True}


class TestProcessValidation:
    def test_process_accepts_valid_song_path(
        self, client: TestClient, reset_state: Path, monkeypatch: pytest.MonkeyPatch
    ) -> None:
        import delyric

        monkeypatch.setattr(delyric, "process_song", lambda _p: None)
        song_dir = _make_song_dir(reset_state)
        resp = client.post("/process", json={"songPath": str(song_dir)})
        assert resp.status_code == 200, resp.text
        body = resp.json()
        assert "jobId" in body
        assert isinstance(body["jobId"], str) and len(body["jobId"]) > 0

    def test_process_rejects_missing_audio_webm(self, client: TestClient, reset_state: Path) -> None:
        d = reset_state / "NoAudio"
        d.mkdir()
        resp = client.post("/process", json={"songPath": str(d)})
        assert resp.status_code == 400
        assert "audio.webm" in resp.text

    def test_process_rejects_path_outside_library(self, client: TestClient) -> None:
        resp = client.post("/process", json={"songPath": "/etc"})
        assert resp.status_code == 400

    def test_process_rejects_path_traversal(self, client: TestClient, reset_state: Path) -> None:
        sub = reset_state / "sub"
        sub.mkdir()
        traversal = f"{sub}/../../etc"
        resp = client.post("/process", json={"songPath": traversal})
        assert resp.status_code == 400

    def test_process_rejects_relative_path(self, client: TestClient) -> None:
        resp = client.post("/process", json={"songPath": "Artist - Title"})
        assert resp.status_code == 400

    def test_process_rejects_nonexistent_dir(self, client: TestClient, reset_state: Path) -> None:
        resp = client.post("/process", json={"songPath": str(reset_state / "ghost")})
        assert resp.status_code == 400


class TestStatus:
    def test_status_unknown_job_returns_404(self, client: TestClient) -> None:
        resp = client.get("/status/nonexistent")
        assert resp.status_code == 404

    def test_status_transitions_to_complete(
        self, client: TestClient, reset_state: Path, monkeypatch: pytest.MonkeyPatch
    ) -> None:
        import delyric

        def fake(_p: Path) -> None:
            time.sleep(0.05)

        monkeypatch.setattr(delyric, "process_song", fake)
        song_dir = _make_song_dir(reset_state)
        job_id = client.post("/process", json={"songPath": str(song_dir)}).json()["jobId"]
        final = _wait_status(client, job_id, {"complete", "failed"})
        assert final["status"] == "complete"
        assert final["error"] is None

    def test_status_failed_carries_error(
        self, client: TestClient, reset_state: Path, monkeypatch: pytest.MonkeyPatch
    ) -> None:
        import delyric

        def boom(_p: Path) -> None:
            raise RuntimeError("boom")

        monkeypatch.setattr(delyric, "process_song", boom)
        song_dir = _make_song_dir(reset_state)
        job_id = client.post("/process", json={"songPath": str(song_dir)}).json()["jobId"]
        final = _wait_status(client, job_id, {"complete", "failed"})
        assert final["status"] == "failed"
        assert "boom" in (final["error"] or "")


class TestSerialQueue:
    def test_serial_queue_processes_one_at_a_time(
        self, client: TestClient, reset_state: Path, monkeypatch: pytest.MonkeyPatch
    ) -> None:
        import delyric

        state = {"active": 0, "max": 0}
        state_lock = threading.Lock()

        def fake(_p: Path) -> None:
            with state_lock:
                state["active"] += 1
                if state["active"] > state["max"]:
                    state["max"] = state["active"]
            time.sleep(0.05)
            with state_lock:
                state["active"] -= 1

        monkeypatch.setattr(delyric, "process_song", fake)

        song_dirs = [_make_song_dir(reset_state, f"Song {i}") for i in range(3)]
        job_ids = []
        for sd in song_dirs:
            jid = client.post("/process", json={"songPath": str(sd)}).json()["jobId"]
            job_ids.append(jid)

        for jid in job_ids:
            final = _wait_status(client, jid, {"complete", "failed"}, timeout=5.0)
            assert final["status"] == "complete", final

        assert state["max"] == 1, f"expected strict serial execution, saw max concurrent={state['max']}"


class TestBindHostPrecheck:
    def test_passes_for_loopback(self) -> None:
        from delyric_worker import bind_host_available

        assert bind_host_available("127.0.0.1") is True

    def test_fails_for_foreign_ip(self) -> None:
        from delyric_worker import bind_host_available

        assert bind_host_available("203.0.113.99") is False

    def test_fails_for_unresolvable_hostname(self) -> None:
        from delyric_worker import bind_host_available

        assert bind_host_available("nonexistent-host-xyz.invalid") is False
