# beads-lite

Minimal dependency-aware task tracker for coding agents.

## Install

```bash
# macOS/Linux
curl -sSL https://raw.githubusercontent.com/kylesnowschwartz/beads-lite/main/install.sh | sh

# or with Go
go install github.com/kylesnowschwartz/beads-lite/cmd/bl@latest
```

## Usage

```bash
bl init                    # initialize in current directory
bl create "Fix login bug"  # create a task
bl ready                   # what can I work on?
bl close <id>              # complete a task
```

### Dependencies

```bash
bl create "Deploy"
bl create "Write tests"
bl update <deploy-id> --blocked-by <tests-id>  # deploy blocked until tests done
bl ready                                        # only "Write tests" shows
bl close <tests-id>
bl ready                                        # now "Deploy" shows
```

### All Commands

```
bl init                          Initialize database
bl create "title"                Create task
bl list                          List all tasks
bl list --tree                   Show dependency tree
bl ready                         Show unblocked tasks
bl show <id>                     Show task details
bl update <id> --title "new"     Update task
bl update <id> --blocked-by <b>  Add blocker
bl close <id>                    Complete task
bl delete <id> --confirm         Delete task
bl export backup.jsonl           Export for git backup
bl import backup.jsonl           Restore from backup
```

## Development

```bash
just test   # run tests
just build  # build ./bl binary
```

## License

MIT
