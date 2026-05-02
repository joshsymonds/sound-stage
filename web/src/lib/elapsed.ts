/**
 * Project the current elapsed time from the last polled value.
 *
 * Polling runs every 5s but the user-visible clock should advance every
 * second. This helper extrapolates using `Date.now()` as the wall-clock
 * anchor. Each poll re-anchors so the projection never drifts more than
 * one poll interval from the server's truth.
 *
 * Inputs:
 *   serverElapsed — the value /api/now-playing returned at anchorMs.
 *   anchorMs      — Date.now() at the moment that value was received.
 *   nowMs         — Date.now() at the moment of render.
 *   duration      — the song's duration; the projection is clamped to this.
 *   paused        — when true, returns serverElapsed unchanged.
 *
 * The projection never runs backwards (anchor > now is treated as "0
 * seconds elapsed since anchor") so transient clock weirdness can't make
 * the displayed time tick down.
 */
export function displayElapsed(
  serverElapsed: number,
  anchorMs: number,
  nowMs: number,
  duration: number,
  paused: boolean,
): number {
  if (paused) return serverElapsed;
  const driftSeconds = Math.max(0, (nowMs - anchorMs) / 1000);
  return Math.min(duration, serverElapsed + driftSeconds);
}
