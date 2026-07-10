import { describe, expect, it } from "vitest";

import { celebrationFor, waitEstimate } from "./party";
import type { QueueEntry } from "./types";

function makeEntry(overrides: Partial<QueueEntry> = {}): QueueEntry {
  return {
    position: 1,
    song: { id: "1", title: "Dancing Queen", artist: "ABBA" },
    guest: "Alice",
    isNext: true,
    ...overrides,
  };
}

describe("waitEstimate", () => {
  it("returns 'up next' for position 1 regardless of remaining time", () => {
    expect(waitEstimate(1)).toBe("up next");
    expect(waitEstimate(1, 90)).toBe("up next");
  });

  it("estimates minutes for position 2 with no remaining time", () => {
    // (2-1) * 3.5 * 60 = 210s -> 3.5min -> rounds to 4
    expect(waitEstimate(2, 0)).toBe("~4 min");
  });

  it("estimates minutes for position 2 with 90s remaining", () => {
    // 210 + 90 = 300s -> 5min
    expect(waitEstimate(2, 90)).toBe("~5 min");
  });

  it("estimates minutes for position 5 with no remaining time", () => {
    // (5-1) * 3.5 * 60 = 840s -> 14min
    expect(waitEstimate(5, 0)).toBe("~14 min");
  });

  it("estimates minutes for position 5 with 90s remaining", () => {
    // 840 + 90 = 930s -> 15.5min -> rounds to 16
    expect(waitEstimate(5, 90)).toBe("~16 min");
  });

  it("treats missing remaining time as zero", () => {
    expect(waitEstimate(2)).toBe("~4 min");
  });
});

describe("celebrationFor", () => {
  it("is true when the guest's song at the head of the queue starts playing", () => {
    const previousQueue = [
      makeEntry({ guest: "Alice", song: { id: "1", title: "A", artist: "A" } }),
    ];
    const queue: QueueEntry[] = [];
    expect(celebrationFor(previousQueue, queue, "1", "Alice")).toBe(true);
  });

  it("is false when a different guest's song starts playing", () => {
    const previousQueue = [
      makeEntry({ guest: "Bob", song: { id: "1", title: "A", artist: "A" } }),
    ];
    const queue: QueueEntry[] = [];
    expect(celebrationFor(previousQueue, queue, "1", "Alice")).toBe(false);
  });

  it("is false on self-removal (song removed without playback starting)", () => {
    const previousQueue = [
      makeEntry({ guest: "Alice", song: { id: "1", title: "A", artist: "A" } }),
    ];
    const queue: QueueEntry[] = [];
    expect(celebrationFor(previousQueue, queue, undefined, "Alice")).toBe(
      false,
    );
  });

  it("is false when the previous queue was empty", () => {
    const queue: QueueEntry[] = [];
    expect(celebrationFor([], queue, "1", "Alice")).toBe(false);
  });
});
