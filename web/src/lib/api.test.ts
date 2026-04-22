import { afterEach, beforeEach, describe, expect, it, vi } from "vitest";

import {
  addToQueue,
  fetchDeckStatus,
  fetchQueue,
  fetchSongs,
  removeFromQueue,
  searchUSDB,
  triggerDownload,
} from "./api";
import type { QueueEntry, Song } from "./types";

const mockSongs: Song[] = [
  { id: "deadbeef00000001", title: "Bohemian Rhapsody", artist: "Queen", year: 1975 },
  { id: "deadbeef00000002", title: "Dancing Queen", artist: "ABBA", edition: "ESC 1974" },
];

const mockQueue: QueueEntry[] = [
  { position: 1, song: mockSongs[0]!, guest: "Alice", isNext: true },
  { position: 2, song: mockSongs[1]!, guest: "Bob", isNext: false },
];

describe("API client", () => {
  beforeEach(() => {
    vi.stubGlobal("fetch", vi.fn());
  });

  afterEach(() => {
    vi.restoreAllMocks();
  });

  it("fetchSongs calls GET /api/songs and returns Song[]", async () => {
    vi.mocked(fetch).mockResolvedValueOnce(
      new Response(JSON.stringify(mockSongs), { status: 200 }),
    );

    const songs = await fetchSongs();
    expect(fetch).toHaveBeenCalledWith("/api/songs");
    expect(songs).toHaveLength(2);
    expect(songs[0]?.title).toBe("Bohemian Rhapsody");
  });

  it("fetchSongs throws on non-OK response", async () => {
    vi.mocked(fetch).mockResolvedValueOnce(
      new Response("error", { status: 500 }),
    );

    await expect(fetchSongs()).rejects.toThrow("fetch songs");
  });

  it("fetchQueue calls GET /api/queue and returns QueueEntry[]", async () => {
    vi.mocked(fetch).mockResolvedValueOnce(
      new Response(JSON.stringify(mockQueue), { status: 200 }),
    );

    const queue = await fetchQueue();
    expect(fetch).toHaveBeenCalledWith("/api/queue");
    expect(queue).toHaveLength(2);
    expect(queue[0]?.guest).toBe("Alice");
  });

  it("addToQueue calls POST /api/queue with song and guest", async () => {
    vi.mocked(fetch).mockResolvedValueOnce(
      new Response(JSON.stringify({ status: "queued" }), { status: 201 }),
    );

    await addToQueue(mockSongs[0]!, "Alice");
    expect(fetch).toHaveBeenCalledWith("/api/queue", {
      method: "POST",
      headers: { "Content-Type": "application/json" },
      body: JSON.stringify({
        songId: "deadbeef00000001",
        title: "Bohemian Rhapsody",
        artist: "Queen",
        duet: undefined,
        edition: undefined,
        year: 1975,
        guest: "Alice",
      }),
    });
  });

  it("addToQueue throws on non-OK response", async () => {
    vi.mocked(fetch).mockResolvedValueOnce(
      new Response("error", { status: 400 }),
    );

    await expect(addToQueue(mockSongs[0]!, "Alice")).rejects.toThrow("add to queue");
  });

  it("searchUSDB calls GET /api/usdb/search with query params", async () => {
    const mockResults = [
      { id: 100, artist: "Queen", title: "Bohemian Rhapsody", language: "English" },
    ];
    vi.mocked(fetch).mockResolvedValueOnce(
      new Response(JSON.stringify(mockResults), { status: 200 }),
    );

    const results = await searchUSDB({ artist: "Queen" });
    expect(fetch).toHaveBeenCalledWith("/api/usdb/search?artist=Queen");
    expect(results).toHaveLength(1);
    expect(results[0]?.title).toBe("Bohemian Rhapsody");
  });

  it("searchUSDB throws on non-OK response", async () => {
    vi.mocked(fetch).mockResolvedValueOnce(
      new Response("error", { status: 502 }),
    );

    await expect(searchUSDB({ title: "test" })).rejects.toThrow("search USDB");
  });

  it("triggerDownload calls POST /api/download with songId and guest", async () => {
    vi.mocked(fetch).mockResolvedValueOnce(
      new Response(JSON.stringify({ status: "downloading" }), { status: 202 }),
    );

    const status = await triggerDownload(12_345, "Alice");
    expect(fetch).toHaveBeenCalledWith("/api/download", {
      method: "POST",
      headers: { "Content-Type": "application/json" },
      body: JSON.stringify({ songId: 12_345, guest: "Alice" }),
    });
    expect(status).toBe("downloading");
  });

  it("triggerDownload throws on non-OK response", async () => {
    vi.mocked(fetch).mockResolvedValueOnce(
      new Response("error", { status: 400 }),
    );

    await expect(triggerDownload(0, "Alice")).rejects.toThrow("trigger download");
  });

  it("removeFromQueue calls DELETE /api/queue/N?guest=NAME", async () => {
    vi.mocked(fetch).mockResolvedValueOnce(
      new Response(null, { status: 204 }),
    );

    await removeFromQueue(2, "Alice");
    expect(fetch).toHaveBeenCalledWith("/api/queue/2?guest=Alice", { method: "DELETE" });
  });

  it("removeFromQueue url-encodes the guest name", async () => {
    vi.mocked(fetch).mockResolvedValueOnce(
      new Response(null, { status: 204 }),
    );

    await removeFromQueue(1, "Alice & Bob");
    expect(fetch).toHaveBeenCalledWith("/api/queue/1?guest=Alice%20%26%20Bob", { method: "DELETE" });
  });

  it("fetchDeckStatus calls GET /api/deck-status and returns the body", async () => {
    vi.mocked(fetch).mockResolvedValueOnce(
      new Response(JSON.stringify({ online: true, lastSeenSecondsAgo: 1.2 }), { status: 200 }),
    );

    const status = await fetchDeckStatus();
    expect(fetch).toHaveBeenCalledWith("/api/deck-status");
    expect(status).toEqual({ online: true, lastSeenSecondsAgo: 1.2 });
  });

  it("fetchDeckStatus returns offline on network error", async () => {
    vi.mocked(fetch).mockRejectedValueOnce(new TypeError("network"));
    const status = await fetchDeckStatus();
    expect(status).toEqual({ online: false, lastSeenSecondsAgo: null });
  });

  it("removeFromQueue throws on non-OK response", async () => {
    vi.mocked(fetch).mockResolvedValueOnce(
      new Response("forbidden", { status: 403 }),
    );

    await expect(removeFromQueue(1, "Bob")).rejects.toThrow("remove from queue");
  });
});
