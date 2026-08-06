"""Pure stall-detection timer for the RunPod backfill supervisor.

A shard is considered stalled if no new done-marker / progress line has
appeared within `stall_threshold_seconds` of the last observed progress —
this catches a pod whose GPU has silently wedged mid-shard (community-cloud
hardware is unreliable) so the supervisor can terminate and relaunch it on a
fresh pod rather than waiting out the full per-pod timeout.
"""

DEFAULT_STALL_THRESHOLD_SECONDS = 15 * 60


def is_stalled(
    last_progress_at: float,
    now: float,
    stall_threshold_seconds: float = DEFAULT_STALL_THRESHOLD_SECONDS,
) -> bool:
    if now < last_progress_at:
        raise ValueError("now must be >= last_progress_at")
    return (now - last_progress_at) > stall_threshold_seconds
