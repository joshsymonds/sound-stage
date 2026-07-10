import type { QueueEntry } from "./types";

// Rough per-song runtime used to project how long a queued guest will wait.
// Not measured from real song durations — a flat average keeps the estimate
// simple and good enough for a party queue.
const AVERAGE_SONG_SECONDS = 3.5 * 60;

/**
 * Estimate how long a guest at `position` in the queue will wait, given the
 * time remaining in the currently playing song (if any).
 *
 * Position 1 is always "up next" — it plays as soon as the current song
 * ends, so there's nothing to estimate. Later positions sum the average
 * runtime of every song ahead of them plus whatever time is left on the
 * song currently playing.
 */
export function waitEstimate(
  position: number,
  nowPlayingRemaining?: number,
): string {
  if (position === 1) return "up next";
  const minutes = Math.round(
    ((position - 1) * AVERAGE_SONG_SECONDS + (nowPlayingRemaining ?? 0)) / 60,
  );
  return `~${String(minutes)} min`;
}

/**
 * Determine whether `guestName`'s song just started playing, so the UI can
 * fire a one-time celebration.
 *
 * True only when the song that was at the head of the queue: belonged to
 * `guestName`, is no longer in the queue, and is now the song playing. This
 * rules out a different guest's song starting (guest mismatch) and a guest
 * removing their own song without it ever playing (nowPlayingId won't match
 * the removed song's id).
 */
export function celebrationFor(
  previousQueue: QueueEntry[],
  queue: QueueEntry[],
  nowPlayingId: string | undefined,
  guestName: string,
): boolean {
  const previousHead = previousQueue[0];
  if (previousHead?.guest !== guestName) return false;
  const stillQueued = queue.some(
    (entry) => entry.song.id === previousHead.song.id,
  );
  if (stillQueued) return false;
  return nowPlayingId === previousHead.song.id;
}
