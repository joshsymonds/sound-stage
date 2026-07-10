import { describe, expect, it } from "vitest";

import { byDecade, duets, genreShelf, recentlyAdded } from "./shelves";
import type { Song } from "./types";

const song = (overrides: Partial<Song> & Pick<Song, "id" | "title" | "artist">): Song => ({
  ...overrides,
});

describe("recentlyAdded", () => {
  it("sorts by addedAt descending", () => {
    const a = song({ id: "1", title: "A", artist: "A", addedAt: "2024-01-01" });
    const b = song({ id: "2", title: "B", artist: "B", addedAt: "2025-06-01" });
    const c = song({ id: "3", title: "C", artist: "C", addedAt: "2020-01-01" });
    expect(recentlyAdded([a, b, c]).map((s) => s.id)).toEqual(["2", "1", "3"]);
  });

  it("sorts songs missing addedAt to the end", () => {
    const a = song({ id: "1", title: "A", artist: "A", addedAt: "2024-01-01" });
    const b = song({ id: "2", title: "B", artist: "B" });
    const c = song({ id: "3", title: "C", artist: "C", addedAt: "2025-01-01" });
    expect(recentlyAdded([a, b, c]).map((s) => s.id)).toEqual(["3", "1", "2"]);
  });

  it("defaults to at most 20 songs", () => {
    const songs = Array.from({ length: 30 }, (_, index) =>
      song({
        id: String(index),
        title: String(index),
        artist: String(index),
        addedAt: `2024-01-${String(index).padStart(2, "0")}`,
      }),
    );
    expect(recentlyAdded(songs)).toHaveLength(20);
  });

  it("respects a custom limit", () => {
    const a = song({ id: "1", title: "A", artist: "A", addedAt: "2024-01-01" });
    const b = song({ id: "2", title: "B", artist: "B", addedAt: "2025-01-01" });
    expect(recentlyAdded([a, b], 1).map((s) => s.id)).toEqual(["2"]);
  });
});

describe("byDecade", () => {
  it("excludes songs from before 1950", () => {
    const a = song({ id: "1", title: "A", artist: "A", year: 1949 });
    expect(byDecade([a])).toEqual([]);
  });

  it("buckets 1950 into the '50s", () => {
    const a = song({ id: "1", title: "A", artist: "A", year: 1950 });
    expect(byDecade([a])).toEqual([{ label: "'50s", songs: [a] }]);
  });

  it("buckets 2026 into the '20s", () => {
    const a = song({ id: "1", title: "A", artist: "A", year: 2026 });
    expect(byDecade([a])).toEqual([{ label: "'20s", songs: [a] }]);
  });

  it("excludes songs with no year", () => {
    const a = song({ id: "1", title: "A", artist: "A" });
    expect(byDecade([a])).toEqual([]);
  });

  it("omits empty decades and orders chronologically", () => {
    const seventies = song({ id: "1", title: "A", artist: "A", year: 1975 });
    const twenties = song({ id: "2", title: "B", artist: "B", year: 2021 });
    const eighties = song({ id: "3", title: "C", artist: "C", year: 1985 });
    expect(byDecade([twenties, seventies, eighties]).map((d) => d.label)).toEqual([
      "'70s",
      "'80s",
      "'20s",
    ]);
  });

  it("groups multiple songs from the same decade together", () => {
    const a = song({ id: "1", title: "A", artist: "A", year: 1991 });
    const b = song({ id: "2", title: "B", artist: "B", year: 1999 });
    expect(byDecade([a, b])).toEqual([{ label: "'90s", songs: [a, b] }]);
  });
});

describe("duets", () => {
  it("returns only songs marked as duets", () => {
    const a = song({ id: "1", title: "A", artist: "A", duet: true });
    const b = song({ id: "2", title: "B", artist: "B", duet: false });
    const c = song({ id: "3", title: "C", artist: "C" });
    expect(duets([a, b, c])).toEqual([a]);
  });
});

describe("genreShelf", () => {
  it("matches pop genres case-insensitively, including compound genres", () => {
    const a = song({ id: "1", title: "A", artist: "A", genre: "Synth-Pop" });
    const b = song({ id: "2", title: "B", artist: "B", genre: "Rock" });
    expect(genreShelf([a, b], "pop")).toEqual([a]);
  });

  it("matches rock, metal, grunge, and punk into the rock bucket", () => {
    const rock = song({ id: "1", title: "A", artist: "A", genre: "Alternative Rock" });
    const metal = song({ id: "2", title: "B", artist: "B", genre: "Heavy Metal" });
    const grunge = song({ id: "3", title: "C", artist: "C", genre: "Grunge" });
    const punk = song({ id: "4", title: "D", artist: "D", genre: "Pop Punk" });
    const pop = song({ id: "5", title: "E", artist: "E", genre: "Pop" });
    expect(genreShelf([rock, metal, grunge, punk, pop], "rock")).toEqual([
      rock,
      metal,
      grunge,
      punk,
    ]);
  });

  it("matches soundtrack, musical, and disney into the soundtrack bucket", () => {
    const soundtrack = song({ id: "1", title: "A", artist: "A", genre: "Soundtrack" });
    const musical = song({ id: "2", title: "B", artist: "B", genre: "Musical" });
    const disney = song({ id: "3", title: "C", artist: "C", genre: "Disney" });
    const pop = song({ id: "4", title: "D", artist: "D", genre: "Pop" });
    expect(genreShelf([soundtrack, musical, disney, pop], "soundtrack")).toEqual([
      soundtrack,
      musical,
      disney,
    ]);
  });

  it("excludes songs with no genre", () => {
    const a = song({ id: "1", title: "A", artist: "A" });
    expect(genreShelf([a], "pop")).toEqual([]);
  });
});
