import { afterEach, describe, expect, it, vi } from "vitest";

import { averageToHsl, dominantColor, hashHue } from "./color";

function makePixels(colors: [number, number, number][]): Uint8ClampedArray {
  const pixels = new Uint8ClampedArray(colors.length * 4);
  for (const [index, [r, g, b]] of colors.entries()) {
    pixels[index * 4] = r;
    pixels[index * 4 + 1] = g;
    pixels[index * 4 + 2] = b;
    pixels[index * 4 + 3] = 255;
  }
  return pixels;
}

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

describe("averageToHsl", () => {
  it("returns null for an empty pixel buffer", () => {
    expect(averageToHsl(new Uint8ClampedArray(0))).toBeNull();
  });

  it("returns null when every pixel is near-black", () => {
    const pixels = makePixels([
      [0, 0, 0],
      [10, 5, 8],
      [20, 20, 20],
    ]);
    expect(averageToHsl(pixels)).toBeNull();
  });

  it("returns null when every pixel is near-white", () => {
    const pixels = makePixels([
      [255, 255, 255],
      [245, 250, 248],
      [230, 230, 230],
    ]);
    expect(averageToHsl(pixels)).toBeNull();
  });

  it("returns a clamped hsl string for a solid red image", () => {
    const pixels = makePixels([
      [255, 0, 0],
      [255, 0, 0],
    ]);
    // Pure red is 100% saturation, clamped down to the 40-70% band; 50%
    // lightness already sits inside the 35-55% band.
    expect(averageToHsl(pixels)).toBe("hsl(0 70% 50%)");
  });

  it("averages the non-extreme pixels while skipping near-black and near-white ones", () => {
    const pixels = makePixels([
      [0, 0, 0],
      [255, 255, 255],
      [0, 0, 255],
      [0, 0, 255],
    ]);
    // Only the two blue pixels should count toward the average.
    expect(averageToHsl(pixels)).toBe("hsl(240 70% 50%)");
  });
});

describe("dominantColor", () => {
  afterEach(() => {
    vi.restoreAllMocks();
  });

  it("resolves null when canvas 2D context is unavailable (jsdom) and caches the result", async () => {
    const createElementSpy = vi.spyOn(document, "createElement");

    const first = await dominantColor("/api/library/no-canvas-test/thumb");
    expect(first).toBeNull();

    const canvasCallsAfterFirst = createElementSpy.mock.calls.filter(
      ([tag]) => tag === "canvas",
    ).length;
    expect(canvasCallsAfterFirst).toBeGreaterThan(0);

    const second = await dominantColor("/api/library/no-canvas-test/thumb");
    expect(second).toBeNull();

    // The loader should not run again for a URL already cached.
    const canvasCallsAfterSecond = createElementSpy.mock.calls.filter(
      ([tag]) => tag === "canvas",
    ).length;
    expect(canvasCallsAfterSecond).toBe(canvasCallsAfterFirst);
  });
});
