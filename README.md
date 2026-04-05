# sound-stage

CLI tool for downloading [UltraStar](https://usdx.eu/) karaoke songs from [USDB](https://usdb.animux.de/) to your local machine. Searches the USDB song database, fetches lyrics and metadata, then downloads audio and video from YouTube via [yt-dlp](https://github.com/yt-dlp/yt-dlp).

Built for use with [Melody Mania](https://store.steampowered.com/app/2394070/Melody_Mania/) but compatible with any UltraStar player.

## Features

- Search USDB by artist and title, output as table or JSON
- Download songs by ID with rate-limited YouTube fetching
- VP9/Opus in WebM — YouTube's native codecs, no transcoding
- Multi-stage YouTube URL extraction from USDB comments (embed, iframe, anchor, plain text)
- Download archive prevents re-downloading completed songs
- Batch download from a file of song IDs
- Optional SOCKS5 proxy support for yt-dlp

## Prerequisites

- [Nix](https://nixos.org/) with [devenv](https://devenv.sh/) — manages Go, yt-dlp, ffmpeg, and all dev tools
- A [USDB](https://usdb.animux.de/) account (free registration)

## Setup

```bash
git clone https://github.com/joshsymonds/sound-stage.git
cd sound-stage
direnv allow  # or: devenv shell
```

Create `.env.local` with your USDB credentials (this file is gitignored):

```
USDB_USERNAME=your_username
USDB_PASSWORD=your_password
```

## Usage

All commands use `just`, which automatically loads credentials from `.env.local`.

### Search for songs

```bash
# Search by artist
just search --artist "Queen"

# Search by artist and title
just search --artist "Queen" --title "Bohemian Rhapsody"

# JSON output (useful for scripting or AI-assisted workflows)
just search --artist "Daft Punk" --json

# Limit results
just search --artist "Beatles" --limit 50
```

### Download songs

```bash
# Download by song ID (from search results)
just download 3715

# Download multiple songs
just download 3715 10385 12345

# Download from a file of IDs (one per line, # comments supported)
just download --from my_songs.txt

# Download to a custom directory
just download 3715 --output ~/Music/karaoke
```

The default output directory is `/mnt/music/sound-stage/`. Override with `--output` or the `SOUND_STAGE_OUTPUT` environment variable.

### What you get

Each downloaded song creates a directory like:

```
Queen - Bohemian Rhapsody/
  song.txt      # UltraStar lyrics with corrected media headers
  audio.webm    # Opus audio (YouTube native)
  video.webm    # VP9 video (YouTube native)
  cover.jpg     # Album art from USDB
```

Point your UltraStar player (Melody Mania, USDX, Vocaluxe) at the output directory and the songs will appear.

### Batch workflow

```bash
# 1. Search and save results
just search --artist "Queen" --json > queen.json

# 2. Extract IDs (e.g., with jq)
jq '.[].id' queen.json > queen_ids.txt

# 3. Download all
just download --from queen_ids.txt
```

Re-running a download skips songs that are already in the `.downloaded.txt` archive.

## Rate limiting

Downloads are rate-limited to be respectful of YouTube:

- 10–30 second random sleep between videos
- 5 MB/s bandwidth cap
- Automatic retry with 30 second backoff on HTTP 429
- Sequential downloads (one at a time)

For bulk downloads (hundreds of songs), run in batches of 20–30 per session with breaks in between.

## Development

```bash
just test     # Run tests with race detector
just lint     # Run golangci-lint (strict config)
just check    # Both lint and test
just build    # Build binary to ./sound-stage
just fmt      # Auto-fix formatting
```

## License

MIT
