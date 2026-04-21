export interface Song {
  id: string;
  title: string;
  artist: string;
  duet?: boolean;
  edition?: string;
  year?: number;
  coverUrl?: string;
}

export interface QueueEntry {
  position: number;
  song: Song;
  guest: string;
  isNext: boolean;
}

// NowPlayingState matches the USDX API.md shape for GET /now-playing.
// When not on ScreenSing with active audio, the endpoint returns null.
export interface NowPlayingState {
  id: string;
  title: string;
  artist: string;
  elapsed: number;
  duration: number;
}
