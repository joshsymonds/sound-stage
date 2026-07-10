/**
 * Deterministically maps a string to a hue (0-359) for the fallback cover
 * generated when a song has no thumbnail. FNV-1a keeps the same artist
 * name mapping to the same color across renders without needing a
 * lookup table.
 */
export function hashHue(input: string): number {
  let hash = 0x81_1c_9d_c5;
  for (let index = 0; index < input.length; index++) {
    hash ^= input.codePointAt(index) ?? 0;
    hash = Math.imul(hash, 0x01_00_01_93);
  }
  return (hash >>> 0) % 360;
}

const NEAR_BLACK_MAX_CHANNEL = 32;
const NEAR_WHITE_MIN_CHANNEL = 224;
const SATURATION_MIN = 40;
const SATURATION_MAX = 70;
const LIGHTNESS_MIN = 35;
const LIGHTNESS_MAX = 55;

function rgbToHsl(r: number, g: number, b: number): { h: number; s: number; l: number } {
  const rNorm = r / 255;
  const gNorm = g / 255;
  const bNorm = b / 255;
  const max = Math.max(rNorm, gNorm, bNorm);
  const min = Math.min(rNorm, gNorm, bNorm);
  const l = (max + min) / 2;

  if (max === min) {
    return { h: 0, s: 0, l: l * 100 };
  }

  const d = max - min;
  const s = l > 0.5 ? d / (2 - max - min) : d / (max + min);
  let h: number;
  switch (max) {
    case rNorm: {
      h = (gNorm - bNorm) / d + (gNorm < bNorm ? 6 : 0);
      break;
    }
    case gNorm: {
      h = (bNorm - rNorm) / d + 2;
      break;
    }
    default: {
      h = (rNorm - gNorm) / d + 4;
      break;
    }
  }
  h /= 6;

  return { h: h * 360, s: s * 100, l: l * 100 };
}

/**
 * Averages the non-extreme pixels of an RGBA buffer into a single clamped
 * HSL color string, for use as a glow tint derived from cover art. Returns
 * null when there is nothing usable to average (empty buffer, or every
 * pixel is near-black/near-white).
 */
export function averageToHsl(pixels: Uint8ClampedArray): string | null {
  let rSum = 0;
  let gSum = 0;
  let bSum = 0;
  let count = 0;

  for (let index = 0; index < pixels.length; index += 4) {
    const r = pixels[index] ?? 0;
    const g = pixels[index + 1] ?? 0;
    const b = pixels[index + 2] ?? 0;
    const max = Math.max(r, g, b);
    const min = Math.min(r, g, b);
    if (max < NEAR_BLACK_MAX_CHANNEL || min > NEAR_WHITE_MIN_CHANNEL) {
      continue;
    }
    rSum += r;
    gSum += g;
    bSum += b;
    count++;
  }

  if (count === 0) {
    return null;
  }

  const { h, s, l } = rgbToHsl(rSum / count, gSum / count, bSum / count);
  const clampedS = Math.min(SATURATION_MAX, Math.max(SATURATION_MIN, s));
  const clampedL = Math.min(LIGHTNESS_MAX, Math.max(LIGHTNESS_MIN, l));

  return `hsl(${String(Math.round(h))} ${String(Math.round(clampedS))}% ${String(Math.round(clampedL))}%)`;
}

const THUMB_SAMPLE_SIZE = 8;
const dominantColorCache = new Map<string, string | null>();

function loadDominantColor(imgUrl: string): Promise<string | null> {
  return new Promise((resolve) => {
    const canvas = document.createElement("canvas");
    canvas.width = THUMB_SAMPLE_SIZE;
    canvas.height = THUMB_SAMPLE_SIZE;
    const ctx = canvas.getContext("2d");
    if (!ctx) {
      resolve(null);
      return;
    }

    const img = new Image();
    img.addEventListener("load", () => {
      try {
        ctx.drawImage(img, 0, 0, THUMB_SAMPLE_SIZE, THUMB_SAMPLE_SIZE);
        const { data } = ctx.getImageData(0, 0, THUMB_SAMPLE_SIZE, THUMB_SAMPLE_SIZE);
        resolve(averageToHsl(data));
      } catch {
        resolve(null);
      }
    });
    img.addEventListener("error", () => {
      resolve(null);
    });
    img.src = imgUrl;
  });
}

/**
 * Loads an image, samples it down to an 8x8 canvas, and derives a clamped
 * HSL glow color from its average color. Results (including failures) are
 * cached per URL so repeated calls for the same image never re-run the
 * loader.
 */
export async function dominantColor(imgUrl: string): Promise<string | null> {
  if (dominantColorCache.has(imgUrl)) {
    return dominantColorCache.get(imgUrl) ?? null;
  }
  const result = await loadDominantColor(imgUrl);
  dominantColorCache.set(imgUrl, result);
  return result;
}
