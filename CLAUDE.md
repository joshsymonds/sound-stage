# sound-stage

CLI tool for downloading UltraStar karaoke songs from [usdb.animux.de](https://usdb.animux.de/) to `/mnt/music/sound-stage/`. Target player is Melody Mania (Steam).

## Setup

Requires devenv (Nix). Enter the dev shell with `direnv allow` or `devenv shell`.

USDB credentials must be in `.env.local` (gitignored):
```
USDB_USERNAME=your_username
USDB_PASSWORD=your_password
```

## Commands

All commands use `just` which automatically loads credentials from `.env.local`.

### Search for songs
```bash
just search --artist "Queen" --json
just search --artist "Queen" --title "Bohemian" --json
just search --artist "Daft Punk" --limit 50
```

The `--json` flag outputs machine-readable JSON. Without it, output is a human-readable table.

### Download songs by ID
```bash
just download 3715
just download 3715 10385 12345
just download --from ids.txt
just download 3715 --output /custom/path
```

Default output: `/mnt/music/sound-stage/`. Each song gets its own `Artist - Title/` directory with:
- `song.txt` — UltraStar lyrics with corrected media headers
- `audio.webm` — Opus audio (YouTube native, no transcode)
- `video.webm` — VP9 video (YouTube native, no transcode)
- `cover.jpg` — Album art from USDB

### Typical workflow
```bash
# 1. Search for songs
just search --artist "Queen" --json

# 2. Review results, pick IDs

# 3. Download selected songs
just download 3715 10385

# 4. Re-running download skips already-downloaded songs
just download 3715  # prints "Skipping song #3715 (already downloaded)"
```

### Batch download from file
Create a file with one song ID per line (# comments supported):
```
# Queen songs
3715
10385
```
Then: `just download --from queen_ids.txt`

## Development

```bash
just test     # Run tests with race detector
just lint     # Run golangci-lint
just check    # Both lint and test
just build    # Build binary to ./sound-stage
just fmt      # Auto-fix formatting
```

## Architecture

- `cmd/` — CLI commands (cobra): root, search, download
- `usdb/` — USDB HTTP client: login, search, detail page parsing, txt download
- `ytdlp/` — yt-dlp wrapper: audio/video download with rate limiting and proxy support
- `archive/` — Download archive: tracks completed song IDs in `.downloaded.txt`

## Key design decisions

- VP9/Opus in WebM — YouTube's native format, no transcoding needed. Melody Mania uses libVLC.
- Rate limiting built in — 10-30s sleep between downloads, 5MB/s bandwidth cap, retries on 429.
- No YouTube search fallback — only downloads songs with explicit YouTube URLs in USDB comments to avoid wrong-video risk.
- Download archive in output dir — `.downloaded.txt` with one song ID per line, checked before each download.
