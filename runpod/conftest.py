"""Ensure the repo root (where delyric.py lives) is importable from tests
under runpod/, regardless of pytest's collection order.

The Docker image mirrors this relative layout (delyric.py and runpod/ as
siblings under /app, run via `python -m runpod.shard_runner`), so production
code never needs this shim — only the test session does, since pytest would
otherwise only add runpod/ itself to sys.path when this directory has no
__init__.py-linked ancestor already imported yet.
"""

import sys
from pathlib import Path

sys.path.insert(0, str(Path(__file__).resolve().parent.parent))
