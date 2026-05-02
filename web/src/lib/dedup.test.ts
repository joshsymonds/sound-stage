import { describe, expect, it } from "vitest";

import { dedupUSDBResults, normalizeSongKey } from "./dedup";
import type { USDBResult } from "./api";
import type { Song } from "./types";

const lib = (artist: string, title: string): Song => ({
  id: `${artist}-${title}`,
  artist,
  title,
});

const usdb = (id: number, artist: string, title: string): USDBResult => ({
  id,
  artist,
  title,
  language: "",
});

describe("normalizeSongKey", () => {
  it("is case-insensitive on artist and title", () => {
    expect(normalizeSongKey("ABBA", "Dancing Queen")).toBe(
      normalizeSongKey("abba", "dancing queen"),
    );
  });

  it("folds the star/special characters used in USDB artist names", () => {
    // A★Teens is a real USDB convention; CLAUDE.md calls it out as a search gotcha.
    expect(normalizeSongKey("A★Teens", "Mamma Mia")).toBe(
      normalizeSongKey("A Teens", "Mamma Mia"),
    );
  });

  it("folds smart quotes and apostrophes", () => {
    expect(normalizeSongKey("Queen", "Don’t Stop Me Now")).toBe(
      normalizeSongKey("Queen", "Don't Stop Me Now"),
    );
  });

  it("collapses whitespace and trims", () => {
    expect(normalizeSongKey("  Queen  ", "Bohemian   Rhapsody")).toBe(
      normalizeSongKey("Queen", "Bohemian Rhapsody"),
    );
  });

  it("treats different titles as different keys (variant-preserving)", () => {
    // The studio version and the Live Aid version must NOT dedup against each
    // other — different titles, different songs from the user's perspective.
    expect(normalizeSongKey("Queen", "Bohemian Rhapsody")).not.toBe(
      normalizeSongKey("Queen", "Bohemian Rhapsody (Live Aid)"),
    );
  });

  it("treats different artists as different keys", () => {
    expect(normalizeSongKey("Queen", "Killer Queen")).not.toBe(
      normalizeSongKey("ABBA", "Killer Queen"),
    );
  });
});

describe("dedupUSDBResults", () => {
  it("filters USDB results that match library by (artist, title)", () => {
    const library = [lib("Queen", "Bohemian Rhapsody"), lib("ABBA", "Dancing Queen")];
    const results = [
      usdb(1, "Queen", "Bohemian Rhapsody"), // dup of library
      usdb(2, "Queen", "Don't Stop Me Now"), // not in library
      usdb(3, "ABBA", "Dancing Queen"), // dup of library
      usdb(4, "Queen", "Killer Queen"), // not in library
    ];
    const out = dedupUSDBResults(library, results);
    expect(out.map((r) => r.id)).toEqual([2, 4]);
  });

  it("returns all results when library is empty", () => {
    const results = [usdb(1, "Queen", "Bohemian Rhapsody")];
    expect(dedupUSDBResults([], results)).toEqual(results);
  });

  it("returns empty when all results match the library", () => {
    const library = [lib("Queen", "Bohemian Rhapsody")];
    const results = [usdb(1, "Queen", "Bohemian Rhapsody")];
    expect(dedupUSDBResults(library, results)).toEqual([]);
  });

  it("preserves order of remaining USDB results", () => {
    const library = [lib("Queen", "B")];
    const results = [
      usdb(1, "Queen", "A"),
      usdb(2, "Queen", "B"), // dropped
      usdb(3, "Queen", "C"),
    ];
    const out = dedupUSDBResults(library, results);
    expect(out.map((r) => r.id)).toEqual([1, 3]);
  });

  it("dedups across formatting variation (case, whitespace, smart quotes)", () => {
    const library = [lib("queen", "don't stop me now")];
    const results = [usdb(1, "Queen", "Don’t  Stop Me Now")];
    expect(dedupUSDBResults(library, results)).toEqual([]);
  });
});
