import type { Song } from "./types";

export interface Shelf {
  label: string;
  songs: Song[];
}

const EARLIEST_DECADE = 1950;

/** Songs ordered by addedAt descending; songs missing addedAt sort last. */
export function recentlyAdded(songs: Song[], n = 20): Song[] {
  return [...songs]
    .sort((a, b) => {
      if (a.addedAt === undefined && b.addedAt === undefined) return 0;
      if (a.addedAt === undefined) return 1;
      if (b.addedAt === undefined) return -1;
      return b.addedAt.localeCompare(a.addedAt);
    })
    .slice(0, n);
}

/**
 * Groups songs into decade shelves ("'50s" through "'20s" and beyond),
 * derived from each song's year. Songs from before 1950 or with no year
 * are excluded. Empty decades are omitted; the result is chronological.
 */
export function byDecade(songs: Song[]): Shelf[] {
  const buckets = new Map<number, Song[]>();
  for (const song of songs) {
    if (song.year === undefined || song.year < EARLIEST_DECADE) continue;
    const decade = Math.floor(song.year / 10) * 10;
    const bucket = buckets.get(decade);
    if (bucket) {
      bucket.push(song);
    } else {
      buckets.set(decade, [song]);
    }
  }
  return [...buckets.entries()]
    .sort(([a], [b]) => a - b)
    .map(([decade, decadeSongs]) => ({
      label: `'${String(decade % 100).padStart(2, "0")}s`,
      songs: decadeSongs,
    }));
}

/** Songs marked as duets. */
export function duets(songs: Song[]): Song[] {
  return songs.filter((song) => song.duet === true);
}

export type GenreBucket = "pop" | "rock" | "soundtrack";

const GENRE_PATTERNS: Record<GenreBucket, RegExp> = {
  pop: /pop/i,
  rock: /rock|metal|grunge|punk/i,
  soundtrack: /soundtrack|musical|disney/i,
};

/** Songs whose genre matches the given bucket's keywords (case-insensitive). */
export function genreShelf(songs: Song[], bucket: GenreBucket): Song[] {
  const pattern = GENRE_PATTERNS[bucket];
  return songs.filter((song) => song.genre !== undefined && pattern.test(song.genre));
}
