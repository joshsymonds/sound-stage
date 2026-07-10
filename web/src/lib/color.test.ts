import { describe, expect, it } from "vitest";

import { hashHue } from "./color";

describe("hashHue", () => {
  it("is deterministic for the same input", () => {
    expect(hashHue("Queen")).toBe(hashHue("Queen"));
  });

  it("returns a value in [0, 359]", () => {
    for (const input of ["Queen", "ABBA", "", "a-ha", "Tears for Fears"]) {
      const hue = hashHue(input);
      expect(hue).toBeGreaterThanOrEqual(0);
      expect(hue).toBeLessThanOrEqual(359);
    }
  });

  it("returns an integer", () => {
    expect(Number.isInteger(hashHue("Queen"))).toBe(true);
  });

  it("generally spreads different inputs across different hues", () => {
    const artists = [
      "Queen",
      "ABBA",
      "a-ha",
      "Tears for Fears",
      "Gotye",
      "Daft Punk",
      "Radiohead",
      "Nirvana",
      "Madonna",
      "Prince",
    ];
    const hues = new Set(artists.map((artist) => hashHue(artist)));
    // Not asserting perfect uniqueness (a hash can collide), but distinct
    // inputs should overwhelmingly land on distinct hues.
    expect(hues.size).toBeGreaterThan(artists.length / 2);
  });
});
