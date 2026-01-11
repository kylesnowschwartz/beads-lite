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
5. **Conventional commits**: Required for auto-release (see below)

## Commit Conventions (Auto-Release)

Pushes to main auto-tag releases based on commit prefix:

| Prefix | Version Bump | Example |
|--------|--------------|---------|
| `fix:` | Patch (0.0.X) | `fix: handle nil pointer in export` |
| `perf:` | Patch (0.0.X) | `perf: batch dependency queries` |
| `feat:` | Minor (0.X.0) | `feat: add --filter flag to list` |
| `feat!:` or `BREAKING CHANGE` | Major (X.0.0) | `feat!: change ID format` |

**No release** (docs, tests, chore):
- `docs:`, `test:`, `chore:`, `style:`, `ci:` - no version bump
- Add `[skip release]` to any commit to skip tagging

**Rules:**
- Every functional change (fix/feat/perf) triggers a release
- Keep commits atomic - one logical change per commit
- Reference issue IDs in commit body: `Closes: bl-xxxx`

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
