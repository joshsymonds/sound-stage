"""Tests for runpod/stall.py — stall-detection timer."""

import pytest

from runpod.stall import DEFAULT_STALL_THRESHOLD_SECONDS, is_stalled


class TestIsStalled:
    def test_not_stalled_immediately_after_progress(self) -> None:
        assert is_stalled(last_progress_at=1000.0, now=1000.0) is False

    def test_not_stalled_just_under_threshold(self) -> None:
        now = 1000.0 + DEFAULT_STALL_THRESHOLD_SECONDS - 1
        assert is_stalled(last_progress_at=1000.0, now=now) is False

    def test_stalled_just_over_threshold(self) -> None:
        now = 1000.0 + DEFAULT_STALL_THRESHOLD_SECONDS + 1
        assert is_stalled(last_progress_at=1000.0, now=now) is True

    def test_custom_threshold_shorter(self) -> None:
        assert is_stalled(last_progress_at=0.0, now=61.0, stall_threshold_seconds=60.0) is True
        assert is_stalled(last_progress_at=0.0, now=59.0, stall_threshold_seconds=60.0) is False

    def test_rejects_now_before_last_progress(self) -> None:
        with pytest.raises(ValueError):
            is_stalled(last_progress_at=500.0, now=400.0)

    def test_default_threshold_is_fifteen_minutes(self) -> None:
        assert DEFAULT_STALL_THRESHOLD_SECONDS == 15 * 60
