# sound-stage development commands

# Load credentials from .env.local
set dotenv-load := true
set dotenv-filename := ".env.local"

# Run all tests
test:
    go test -race ./...

# Run linter
lint:
    golangci-lint run ./...

# Format code
fmt:
    golangci-lint run --fix ./...

# Build binary
build:
    go build -o sound-stage .

# Run all checks (lint + test)
check: lint test

# Run tests with verbose output
test-v:
    go test -race -v ./...

# Search USDB for songs (pass any flags after --)
search *ARGS:
    go run . search {{ARGS}}

# Download songs by ID (pass IDs and flags after --)
download *ARGS:
    go run . download {{ARGS}}

# Strip audio tracks from all video.webm files in the output directory
strip-video-audio dir:
    #!/usr/bin/env bash
    set -euo pipefail
    count=0
    while IFS= read -r f; do
      if ffprobe "$f" 2>&1 | grep -q "Audio:"; then
        tmp="${f%.webm}_noaudio.webm"
        ffmpeg -y -i "$f" -an -c:v copy "$tmp" 2>/dev/null && mv "$tmp" "$f"
        echo "Stripped: $f"
        count=$((count + 1))
      fi
    done < <(find "{{dir}}" -name "video.webm")
    echo "Done. Stripped audio from $count video(s)."

# Separate vocals from instrumentals (venv auto-created by devenv)
delyric *ARGS:
    python3 delyric.py {{ARGS}}

# Preview what delyric would process
delyric-dry-run:
    python3 delyric.py --dry-run

# Run any sound-stage subcommand
run *ARGS:
    go run . {{ARGS}}
