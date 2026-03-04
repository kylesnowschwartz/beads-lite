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

# Bump version (patch, minor, or major)
bump version:
    #!/usr/bin/env zsh
    set -e

    v=$(cat VERSION)
    IFS='.' read -r M m p <<< "$v"

    case {{version}} in
        patch) new="$M.$m.$((p+1))" ;;
        minor) new="$M.$((m+1)).0" ;;
        major) new="$((M+1)).0.0" ;;
        *) echo "Usage: just bump patch|minor|major" && exit 1 ;;
    esac

    echo "Bumping $v → $new"
    echo "$new" > VERSION
    git add VERSION
    echo "Version bumped to $new. Run 'just release' to commit, tag, and push."

# Commit, tag, and push the release
release notes="":
    #!/usr/bin/env zsh
    set -e

    v=$(cat VERSION)

    # Sanity checks
    git fetch --tags
    if git tag -l "v$v" | grep -q .; then
        echo "Error: tag v$v already exists"
        exit 1
    fi

    behind=$(git rev-list HEAD..origin/main --count 2>/dev/null || echo 0)
    if [[ "$behind" -gt 0 ]]; then
        echo "Error: $behind commit(s) behind origin/main"
        echo "Run 'git pull --rebase' first"
        exit 1
    fi

    if git diff --cached --quiet; then
        echo "Error: nothing staged. Run 'just bump' first."
        exit 1
    fi

    # Commit, tag, push
    git commit -m "chore: bump version to $v"
    git tag "v$v"
    git push && git push --tags

    # Create GitHub Release
    notes="{{notes}}"
    if [[ -n "$notes" && -f "$notes" ]]; then
        gh release create "v$v" --title "v$v" --notes-file "$notes" --latest
    else
        gh release create "v$v" --title "v$v" --generate-notes --latest
    fi

    # Prime Go module proxy cache
    GOPROXY=https://proxy.golang.org go list -m "github.com/kylesnowschwartz/beads-lite@v$v" || true
