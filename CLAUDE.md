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
just test     # run tests
just build    # build ./bl binary
just dev      # test then build

./bl <command>
```

## Task Tracking

This project uses beads-lite to track its own development.

```bash
bl ready              # what can I work on?
bl list --tree        # see all tasks with dependencies
bl create "title"     # new task
bl close <id>         # complete task
bl dep add <a> <b>    # a blocked by b
```

## Development Rules

1. **TDD**: Write failing test first, then implement to make it pass
2. **Stdlib preferred**: Avoid external dependencies beyond go-sqlite3
3. **No short flags**: Use `--json` not `-j` (AI agents parse long flags better)
4. **Tests > Docs > Code**: When in doubt, the test is the spec
5. **Run tests frequently**: `just test` after each meaningful change

### TDD Workflow

1. Write test in `*_test.go` that exercises the new behavior
2. Run `just test` - confirm test fails (red)
3. Implement minimum code to pass
4. Run `just test` - confirm test passes (green)
5. Refactor if needed, keeping tests green

### Critical Constraints

- **NO SHORT FLAGS**: This CLI is for AI agents. Use `fs.String("name", ...)` not `fs.StringP("name", "n", ...)`
- **Minimal deps**: go-sqlite3 for DB, pflag for CLI parsing. No cobra, no viper.
- **Keep it simple**: If a feature needs a diagram to explain, it's too complex

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
