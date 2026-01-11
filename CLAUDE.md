# beads-lite

Minimal dependency-aware task tracker for coding agents. Tracks tasks with blocking dependencies and answers "what's ready to work on?"

## Commands

```bash
just test     # run tests
just build    # build ./bl binary

bl ready              # what can I work on?
bl list --tree        # see all tasks with dependencies
bl create "title"     # new task
bl close <id>         # complete task
bl update <a> --blocked-by <b>  # a blocked by b
```

## Development Rules

1. **TDD**: Write failing test first, then implement
2. **Stdlib preferred**: Only deps are go-sqlite3 and pflag
3. **No short flags**: Use `--json` not `-j` (AI agents parse long flags better)
4. **Tests are the spec**: When in doubt, check the tests

## Architecture

```
main.go         # CLI entry point + command handlers
issue.go        # Issue struct + validation
dependency.go   # Dependency types
storage.go      # SQLite implementation
jsonl.go        # Import/export for git backup
```

## Core Algorithm

The blocking calculation uses a recursive CTE that finds:
1. Issues directly blocked by open issues (via `blocks` dependency)
2. Issues transitively blocked via parent-child relationships

Everything else is CRUD.
