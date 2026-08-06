"""Pure spend-guard math for the RunPod backfill supervisor.

Two independent caps — concurrent pods and total pod-hour budget — both must
be satisfied before the supervisor is allowed to launch another pod. Kept
free of subprocess/API calls so the guard logic is unit-testable without
touching RunPod.
"""

from dataclasses import dataclass


@dataclass(frozen=True)
class SpendGuard:
    max_concurrent_pods: int
    max_pod_hours: float

    def can_launch(self, active_pods: int, spent_pod_hours: float) -> bool:
        return self.refusal_reason(active_pods, spent_pod_hours) is None

    def refusal_reason(self, active_pods: int, spent_pod_hours: float) -> str | None:
        """Return why a launch would be refused, or None if it's allowed.

        Concurrent-pod cap is checked first — it's the more immediately
        actionable constraint (wait for a pod to free up) versus the
        pod-hour budget, which is a terminal stop.
        """
        if active_pods >= self.max_concurrent_pods:
            return (
                f"refusing to launch: {active_pods} active pods >= "
                f"max_concurrent_pods={self.max_concurrent_pods}"
            )
        if spent_pod_hours >= self.max_pod_hours:
            return (
                f"refusing to launch: {spent_pod_hours:.2f} spent pod-hours >= "
                f"max_pod_hours={self.max_pod_hours}"
            )
        return None


def pod_hours_elapsed(started_at: float, now: float) -> float:
    """Convert a wall-clock span (unix seconds) into pod-hours."""
    if now < started_at:
        raise ValueError("now must be >= started_at")
    return (now - started_at) / 3600.0
