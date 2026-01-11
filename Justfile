# Default recipe - show available commands
default:
    @just --list

# Run all tests
test:
    go test ./...

# Build the CLI binary
build:
    go build -o bl ./cmd/bl

# Run tests in verbose mode
test-v:
    go test -v ./...

# Quick dev cycle: test then build
dev: test build
