"""Tests for runpod/budget.py — spend-guard math (concurrent-pod cap + pod-hour budget)."""

import pytest

from runpod.budget import SpendGuard, pod_hours_elapsed


class TestSpendGuardCanLaunch:
    def test_allows_launch_under_both_caps(self) -> None:
        guard = SpendGuard(max_concurrent_pods=4, max_pod_hours=100.0)
        assert guard.can_launch(active_pods=2, spent_pod_hours=10.0) is True

    def test_refuses_at_concurrent_cap(self) -> None:
        guard = SpendGuard(max_concurrent_pods=4, max_pod_hours=100.0)
        assert guard.can_launch(active_pods=4, spent_pod_hours=0.0) is False

    def test_refuses_above_concurrent_cap(self) -> None:
        guard = SpendGuard(max_concurrent_pods=4, max_pod_hours=100.0)
        assert guard.can_launch(active_pods=5, spent_pod_hours=0.0) is False

    def test_refuses_at_pod_hour_budget(self) -> None:
        guard = SpendGuard(max_concurrent_pods=4, max_pod_hours=100.0)
        assert guard.can_launch(active_pods=0, spent_pod_hours=100.0) is False

    def test_refuses_above_pod_hour_budget(self) -> None:
        guard = SpendGuard(max_concurrent_pods=4, max_pod_hours=100.0)
        assert guard.can_launch(active_pods=0, spent_pod_hours=150.0) is False

    def test_allows_just_under_pod_hour_budget(self) -> None:
        guard = SpendGuard(max_concurrent_pods=4, max_pod_hours=100.0)
        assert guard.can_launch(active_pods=0, spent_pod_hours=99.99) is True


class TestSpendGuardRefusalReason:
    def test_none_when_launch_allowed(self) -> None:
        guard = SpendGuard(max_concurrent_pods=4, max_pod_hours=100.0)
        assert guard.refusal_reason(active_pods=1, spent_pod_hours=1.0) is None

    def test_names_concurrent_cap_when_that_is_the_blocker(self) -> None:
        guard = SpendGuard(max_concurrent_pods=4, max_pod_hours=100.0)
        reason = guard.refusal_reason(active_pods=4, spent_pod_hours=0.0)
        assert reason is not None
        assert "4" in reason
        assert "concurrent" in reason.lower()

    def test_names_pod_hour_budget_when_that_is_the_blocker(self) -> None:
        guard = SpendGuard(max_concurrent_pods=4, max_pod_hours=100.0)
        reason = guard.refusal_reason(active_pods=0, spent_pod_hours=100.0)
        assert reason is not None
        assert "100" in reason
        assert "pod-hour" in reason.lower() or "pod_hours" in reason.lower()

    def test_concurrent_cap_checked_before_pod_hour_budget(self) -> None:
        """When both caps are blown, the concurrent-pod reason should surface
        first — it's the more actionable/immediate constraint."""
        guard = SpendGuard(max_concurrent_pods=4, max_pod_hours=100.0)
        reason = guard.refusal_reason(active_pods=10, spent_pod_hours=200.0)
        assert reason is not None
        assert "concurrent" in reason.lower()


class TestPodHoursElapsed:
    def test_converts_seconds_to_hours(self) -> None:
        assert pod_hours_elapsed(started_at=0.0, now=3600.0) == pytest.approx(1.0)

    def test_zero_elapsed(self) -> None:
        assert pod_hours_elapsed(started_at=1000.0, now=1000.0) == 0.0

    def test_fractional_hours(self) -> None:
        assert pod_hours_elapsed(started_at=0.0, now=1800.0) == pytest.approx(0.5)

    def test_rejects_negative_span(self) -> None:
        with pytest.raises(ValueError):
            pod_hours_elapsed(started_at=100.0, now=50.0)
