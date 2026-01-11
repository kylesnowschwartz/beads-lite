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

### CLI Reference

```
Usage: bl <command> [args]

Commands:
  init                  Initialize .beads-lite/ directory and database
  create <title>        Create a new issue, prints ID
  list                  List all issues
  show <id>             Show issue details
  update <id>           Update an issue (including blockers)
  delete <id>           Delete an issue permanently (requires --confirm)
  close <id>            Close an issue
  ready                 List unblocked work
  export [file]         Export all issues to JSONL (stdout or file)
  import <file>         Import issues from JSONL file
  onboard               Print Claude Code integration instructions
  version               Show version
  upgrade               Upgrade to latest release

List/Ready Flags:
  --json                Output as JSONL (one JSON object per line)
  --tree                Show dependency tree
  --priority <int>      Filter by priority (0-4)
  --type <string>       Filter by type (task, bug, feature, epic)

List-Only Flags:
  --status <string>     Filter by status (open, in_progress, closed)
  --resolution <string> Filter by resolution (done, wontfix, duplicate)

Show Flags:
  --json                Output as JSON

Create Flags:
  --description <text>  Issue description
  --priority <int>      Priority (0-4), default 2
  --type <string>       Type (task, bug, feature, epic), default task
  --blocked-by <id>     Issue ID that blocks this (repeatable)

Update Flags:
  --title <string>      New title
  --status <string>     New status (open, in_progress, closed)
  --priority <int>      New priority (0-4)
  --type <string>       New type (task, bug, feature, epic)
  --description <text>  New description
  --blocked-by <id>     Add blocker (repeatable)
  --unblock <id>        Remove blocker (repeatable)

Close Flags:
  --resolution <string> Resolution (done, wontfix, duplicate), default done

Delete Flags:
  --confirm             Required to confirm permanent deletion
```

## Development

```bash
just test   # run tests
just build  # build ./bl binary
```

## License

MIT
