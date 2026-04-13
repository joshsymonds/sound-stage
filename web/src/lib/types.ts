export interface Song {
  id: number;
  title: string;
  artist: string;
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
