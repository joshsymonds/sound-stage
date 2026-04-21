import type { NowPlayingState, QueueEntry, Song } from "./types";

export interface USDBResult {
  id: number;
  artist: string;
  title: string;
  language: string;
}

export async function fetchSongs(): Promise<Song[]> {
  const response = await fetch("/api/songs");
  if (!response.ok) {
    throw new Error(`Failed to fetch songs: ${String(response.status)}`);
  }
  return response.json() as Promise<Song[]>;
}

export async function fetchQueue(): Promise<QueueEntry[]> {
  const response = await fetch("/api/queue");
  if (!response.ok) {
    throw new Error(`Failed to fetch queue: ${String(response.status)}`);
  }
  return response.json() as Promise<QueueEntry[]>;
}

export async function addToQueue(song: Song, guest: string): Promise<void> {
  const response = await fetch("/api/queue", {
    method: "POST",
    headers: { "Content-Type": "application/json" },
    body: JSON.stringify({
      songId: song.id,
      title: song.title,
      artist: song.artist,
      duet: song.duet,
      edition: song.edition,
      year: song.year,
      guest,
    }),
  });
  if (!response.ok) {
    throw new Error(`Failed to add to queue: ${String(response.status)}`);
  }
}

export async function searchUSDB(params: { artist?: string; title?: string; edition?: string }): Promise<USDBResult[]> {
  const query = new URLSearchParams();
  if (params.artist) query.set("artist", params.artist);
  if (params.title) query.set("title", params.title);
  if (params.edition) query.set("edition", params.edition);

  const response = await fetch(`/api/usdb/search?${query.toString()}`);
  if (!response.ok) {
    throw new Error(`Failed to search USDB: ${String(response.status)}`);
  }
  return response.json() as Promise<USDBResult[]>;
}

export async function triggerDownload(songId: number, guest: string): Promise<string> {
  const response = await fetch("/api/download", {
    method: "POST",
    headers: { "Content-Type": "application/json" },
    body: JSON.stringify({ songId, guest }),
  });
  if (!response.ok) {
    throw new Error(`Failed to trigger download: ${String(response.status)}`);
  }
  const result = await response.json() as { status: string };
  return result.status;
}

export async function fetchNowPlaying(): Promise<NowPlayingState | null> {
  const response = await fetch("/api/now-playing");
  if (!response.ok) {
    return null;
  }
  return response.json() as Promise<NowPlayingState | null>;
}

export async function pausePlayback(): Promise<void> {
  await fetch("/api/playback/pause", { method: "POST" });
}

export async function resumePlayback(): Promise<void> {
  await fetch("/api/playback/resume", { method: "POST" });
}

export async function removeFromQueue(position: number, guest: string): Promise<void> {
  const response = await fetch(`/api/queue/${String(position)}?guest=${encodeURIComponent(guest)}`, {
    method: "DELETE",
  });
  if (!response.ok) {
    throw new Error(`Failed to remove from queue: ${String(response.status)}`);
  }
}

