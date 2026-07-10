#!/usr/bin/env python3
"""Startup GPU probe for RunPod backfill pods.

Runs before any shard work (invoked from runpod-start.sh). Community-cloud
GPUs are unreliable: a host can silently fall back to CPU (which
delyric.verify_cuda's torch.cuda.is_available() probe would already catch)
or report CUDA available yet be throttled/broken hardware that still runs at
a crawl. This probe adds a second check that catches the latter: a full
separation pass on a bundled ~10s test clip must complete within
PROBE_TIMEOUT_SECONDS (default 120s). A pod that fails either check is
useless for the backfill and should never start processing a shard.
"""

import logging
import os
import sys
import tempfile
import time
from pathlib import Path

import delyric

logger = logging.getLogger("startup_probe")

TEST_CLIP_DIR = Path(__file__).resolve().parent / "testclip"
DEFAULT_PROBE_TIMEOUT_SECONDS = 120.0


def run_probe(timeout_seconds: float = DEFAULT_PROBE_TIMEOUT_SECONDS) -> None:
    """Run the startup probe. Raises RuntimeError on any failure."""
    delyric.AUDIO_SEPARATOR = delyric.resolve_audio_separator()
    delyric.verify_cuda()

    clip_path = TEST_CLIP_DIR / "audio.webm"
    if not clip_path.exists():
        raise RuntimeError(f"bundled GPU probe test clip missing at {clip_path}")

    start = time.monotonic()
    with tempfile.TemporaryDirectory(prefix="probe_") as tmp:
        delyric.separate_song(TEST_CLIP_DIR, Path(tmp))
    elapsed = time.monotonic() - start

    if elapsed > timeout_seconds:
        raise RuntimeError(
            f"GPU probe separation took {elapsed:.1f}s, exceeding the "
            f"{timeout_seconds:.0f}s threshold — likely CPU fallback or a "
            "throttled/broken community GPU"
        )

    logger.info(
        "GPU probe passed: separation completed in %.1fs (threshold %.0fs)",
        elapsed, timeout_seconds,
    )


def main() -> None:
    logging.basicConfig(level=logging.INFO, format="%(asctime)s %(levelname)s %(message)s")
    timeout = float(os.environ.get("PROBE_TIMEOUT_SECONDS", DEFAULT_PROBE_TIMEOUT_SECONDS))
    try:
        run_probe(timeout)
    except Exception as exc:
        logger.error("STARTUP GPU PROBE FAILED: %s", exc)
        sys.exit(1)
    sys.exit(0)


if __name__ == "__main__":
    main()
