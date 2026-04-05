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

# Run any sound-stage subcommand
run *ARGS:
    go run . {{ARGS}}
