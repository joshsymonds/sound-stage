# sound-stage development commands

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
