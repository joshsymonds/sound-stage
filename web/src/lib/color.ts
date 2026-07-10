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
