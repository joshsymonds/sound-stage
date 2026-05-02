import { describe, expect, it } from "vitest";

import { displayElapsed } from "./elapsed";

describe("displayElapsed", () => {
  it("returns the server-anchored value at the moment of poll", () => {
    expect(displayElapsed(42, 1_000_000, 1_000_000, 200, false)).toBe(42);
  });

  it("advances by (now - anchor) seconds between polls", () => {
    expect(displayElapsed(42, 1_000_000, 1_002_000, 200, false)).toBe(44);
  });

  it("freezes at the anchored value while paused", () => {
    expect(displayElapsed(42, 1_000_000, 1_010_000, 200, true)).toBe(42);
  });

  it("clamps at duration so the UI never reports past the end", () => {
    // 198 + 10s would be 208, but duration is 200.
    expect(displayElapsed(198, 1_000_000, 1_010_000, 200, false)).toBe(200);
  });

  it("does not run backwards if anchor somehow lands in the future", () => {
    // Defensive: clock skew or test timing shouldn't produce negative drift.
    expect(displayElapsed(42, 1_010_000, 1_000_000, 200, false)).toBe(42);
  });

  it("returns 0 when serverElapsed is 0 and no time has passed", () => {
    expect(displayElapsed(0, 1_000_000, 1_000_000, 200, false)).toBe(0);
  });
});
