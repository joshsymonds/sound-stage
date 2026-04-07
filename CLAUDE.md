# sound-stage

CLI tool for downloading UltraStar karaoke songs from [usdb.animux.de](https://usdb.animux.de/) to `/mnt/music/sound-stage/`. Target player is USDX (UltraStar Deluxe).

## Setup

Requires devenv (Nix). Enter the dev shell with `direnv allow` or `devenv shell`.

USDB credentials must be in `.env.local` (gitignored):
```
USDB_USERNAME=your_username
USDB_PASSWORD=your_password
```

Vocal separation (delyric) deps are installed automatically on first `devenv shell` — audio-separator[gpu] is pip-installed into a venv (CUDA via pip wheels, no Nix CUDA rebuild).

## Commands

All commands use `just` which automatically loads credentials from `.env.local`.

### Search for songs
```bash
just search --artist "Queen" --json
just search --artist "Queen" --title "Bohemian" --json
just search --artist "Daft Punk" --limit 50
just search --edition "Melodifestivalen 2026" --limit 50 --json
```

The `--json` flag outputs machine-readable JSON. Without it, output is a human-readable table.

Search supports `--artist`, `--title`, `--edition`, and `--limit` flags. All filters are combined (AND logic).

### Searching for Eurovision / contest songs

USDB tags contest songs in **two different ways** — check both:

1. **Edition field** (`--edition`): Used for national selections. Without quotes the match is fuzzy/substring — wrap in literal quotes for exact matching:
   ```bash
   # Fuzzy (may include older Melfest songs):
   just search --edition "Melodifestivalen 2026" --limit 50 --json
   # Exact (only Melfest 2026):
   just search --edition '"Melodifestivalen 2026"' --limit 50 --json
   ```

2. **Title suffix**: Many ESC entries have `(ESC YYYY Country)` appended to the title.
   ```bash
   just search --title "ESC 2026" --limit 50 --json
   ```

For best coverage, search both ways — a song may use one convention or the other, rarely both.

**Known edition names for Eurovision:**
- `Melodifestivalen YYYY` — Swedish national selection (Melfest)
- `ESC YYYY` — Eurovision Song Contest (but note: most ESC songs use title suffixes like `(ESC 2026 Sweden)` instead of editions)

**Gotcha — special characters in artist names:** USDB uses Unicode characters like `A★Teens` (star ★, U+2605). Searching `A*Teens` with an ASCII asterisk won't match. When an artist search returns no results, try searching by title instead, or use a partial/simplified artist name.

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

### Separate vocals from instrumentals
```bash
just delyric                          # Process all unprocessed songs
just delyric --song "ABBA - Dancing Queen"  # Process a single song
just delyric --dry-run                # Preview what would be processed
just delyric --force                  # Reprocess even if outputs exist
```

Requires Python with `pip install audio-separator[gpu] click tqdm`. Runs on a GPU workstation (not part of the download pipeline). Produces `instrumental.webm` and `vocals.webm` alongside existing `audio.webm` in each song directory, and adds `#INSTRUMENTAL:` and `#VOCALS:` tags to `song.txt`.

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
- `ytdlp/` — yt-dlp wrapper: parallel audio/video download with retry and proxy support
- `archive/` — Download archive: tracks completed song IDs in `.downloaded.txt`
- `delyric.py` — Vocal separation pipeline: ensemble AI separation using audio-separator (Mel-Band Roformer + HTDemucs_ft)

## Key design decisions

- VP9/Opus in WebM — YouTube's native format, no transcoding needed. USDX uses FFmpeg for playback.
- USDB rate-limit handling — detects "Please wait N seconds" responses and automatically retries after the countdown.
- Parallel media downloads — audio and video are fetched concurrently from YouTube.
- yt-dlp retries on transient errors (HTTP 429, network issues).
- No YouTube search fallback — only downloads songs with explicit YouTube URLs in USDB comments to avoid wrong-video risk.
- Download archive in output dir — `.downloaded.txt` with one song ID per line, checked before each download.
- Vocal separation uses Mel-Band Roformer + HTDemucs_ft ensemble (~10.8 dB SDR) via audio-separator — best available quality for instrumental isolation. Outputs Opus/WebM at 128kbps to match source format. USDX `DefaultSingMode=Instrumental` makes instrumentals the default for all songs with `#INSTRUMENTAL:` tags.
