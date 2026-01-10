# beads-lite

Minimal dependency-aware task tracker for coding agents. A stripped-down clone of [steveyegge/beads](https://github.com/steveyegge/beads) without the enterprise bloat.

## Context

@.agent-history/context-packet-beads-lite.md

## Reference Implementation

@.cloned-sources/beads/ - Original beads repo for algorithm reference

Key files:
- `internal/types/types.go` - Issue/Dependency structs
- `internal/storage/sqlite/blocked_cache.go` - Blocking algorithm
- `internal/idgen/hash.go` - Hash-based ID generation

## Commands

```bash
# Run tests
go test ./...

# Build
go build -o bl .

# Run
./bl <command>
```

## Development Rules

1. **TDD**: Write failing test first, then implement
2. **Stdlib preferred**: Avoid external dependencies beyond go-sqlite3
3. **Simple CLI**: Minimal flags, easy for AI agents to use correctly
4. **Tests > Docs > Code**: When in doubt, the test is the spec

## Architecture

```
beads-lite/
├── main.go           # CLI entry point
├── issue.go          # Issue struct + validation
├── dependency.go     # Dependency types + blocking calculation
├── storage.go        # SQLite implementation
├── jsonl.go          # Import/export for git backup
└── commands/         # CLI command handlers
```

## Core Algorithm

The money shot is the blocking calculation - a recursive CTE that finds:
1. Issues directly blocked by open issues (via `blocks` dependency)
2. Issues transitively blocked via parent-child relationships

Everything else is CRUD.
