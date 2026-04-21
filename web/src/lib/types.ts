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

export interface NowPlayingState {
  song: Song;
  singer: string;
  elapsed: number;
  duration: number;
}
