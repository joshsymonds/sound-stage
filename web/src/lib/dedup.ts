import type { USDBResult } from "./api";
import type { Song } from "./types";

// Characters that USDB editors often use as decorative substitutions for
// ASCII equivalents. Folding them to a plain space (or apostrophe) lets
// "A★Teens" match "A Teens" and "Don't" match "Don't" without forcing
// users to think about Unicode in their search box.
const FOLD_TO_SPACE = /[★☆●◆◇♥♡♪♫]/g;
const FOLD_TO_APOSTROPHE = /[‘’‛′]/g; // ‘ ’ ‛ ′
const FOLD_TO_QUOTE = /[“”‟″]/g; // “ ” ‟ ″
const FOLD_TO_HYPHEN = /[‐-―]/g; // hyphen, dashes, em/en dash
const COLLAPSE_WS = /\s+/g;

/**
 * Returns a normalized "(artist, title)" key suitable for set membership.
 *
 * Two songs that a human would consider "the same" produce the same key
 * even when their formatting differs (case, smart quotes, decorative
 * stars, extra whitespace). Two songs with materially different titles
 * — including parenthetical version markers like "(Live Aid)" — produce
 * different keys, so variants stay distinct.
 */
export function normalizeSongKey(artist: string, title: string): string {
  return `${normalize(artist)}${normalize(title)}`;
}

function normalize(s: string): string {
  return s
    .normalize("NFKD")
    .replace(FOLD_TO_APOSTROPHE, "'")
    .replace(FOLD_TO_QUOTE, '"')
    .replace(FOLD_TO_HYPHEN, "-")
    .replace(FOLD_TO_SPACE, " ")
    .toLowerCase()
    .replace(COLLAPSE_WS, " ")
    .trim();
}

/**
 * Filters USDB results to those NOT already represented in the local
 * library. Matching is by normalized (artist, title); same title in two
 * different versions remains visible. Order of input is preserved.
 */
export function dedupUSDBResults(library: Song[], results: USDBResult[]): USDBResult[] {
  const known = new Set<string>();
  for (const song of library) {
    known.add(normalizeSongKey(song.artist, song.title));
  }
  return results.filter((r) => !known.has(normalizeSongKey(r.artist, r.title)));
}
