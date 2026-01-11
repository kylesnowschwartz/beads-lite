package beadslite

import (
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"sort"
	"strings"

	flag "github.com/spf13/pflag"
)

const (
	beadsDir = ".beads-lite"
	dbName   = "beads.db"
)

// Version is set at build time via ldflags
var Version = "dev"

// Run executes the CLI with the given arguments and writes output to w.
// This is the main entry point for the CLI, separated from main() for testing.
func Run(args []string, w io.Writer) error {
	if len(args) == 0 {
		printHelp(w)
		return nil
	}

	cmd := args[0]
	cmdArgs := args[1:]

	switch cmd {
	case "init":
		return cmdInit(w)
	case "create":
		return cmdCreate(cmdArgs, w)
	case "list":
		return cmdList(cmdArgs, w)
	case "show":
		return cmdShow(cmdArgs, w)
	case "update":
		return cmdUpdate(cmdArgs, w)
	case "delete":
		return cmdDelete(cmdArgs, w)
	case "close":
		return cmdClose(cmdArgs, w)
	case "ready":
		return cmdReady(cmdArgs, w)
	case "export":
		return cmdExport(cmdArgs, w)
	case "import":
		return cmdImport(cmdArgs, w)
	case "onboard":
		return cmdOnboard(w)
	case "version", "-v", "--version":
		return cmdVersion(w)
	case "upgrade":
		return cmdUpgrade(w)
	case "help", "-h", "--help":
		printHelp(w)
		return nil
	default:
		return fmt.Errorf("unknown command: %s", cmd)
	}
}

func printHelp(w io.Writer) {
	fmt.Fprintln(w, `Usage: bl <command> [args]

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
  --confirm             Required to confirm permanent deletion`)
}

func getDBPath() string {
	return filepath.Join(beadsDir, dbName)
}

func openStore() (*Store, error) {
	dbPath := getDBPath()
	if _, err := os.Stat(dbPath); os.IsNotExist(err) {
		return nil, errors.New("not initialized: run 'bl init' first")
	}
	return NewStore(dbPath)
}

// cmdInit creates the .beads-lite directory and initializes the database
func cmdInit(w io.Writer) error {
	if err := os.MkdirAll(beadsDir, 0755); err != nil {
		return fmt.Errorf("failed to create %s: %w", beadsDir, err)
	}

	store, err := NewStore(getDBPath())
	if err != nil {
		return fmt.Errorf("failed to initialize database: %w", err)
	}
	defer store.Close()

	fmt.Fprintln(w, "Initialized beads-lite in", beadsDir)
	fmt.Fprintln(w)
	fmt.Fprintln(w, "Tip: Run 'bl onboard > .claude/CLAUDE.md' to set up Claude Code integration")
	return nil
}

// cmdCreate creates a new issue
func cmdCreate(args []string, w io.Writer) error {
	fs := flag.NewFlagSet("create", flag.ContinueOnError)
	fs.SetOutput(w)
	description := fs.String("description", "", "Issue description")
	priority := fs.Int("priority", 2, "Priority (0-4)")
	issueType := fs.String("type", "task", "Type (task, bug, feature, epic)")
	blockedBy := fs.StringSlice("blocked-by", nil, "Issue ID that blocks this (repeatable)")

	if err := fs.Parse(args); err != nil {
		return err
	}

	remaining := fs.Args()
	if len(remaining) == 0 {
		return errors.New("usage: bl create <title> [--description <text>] [--priority <0-4>] [--type <task|bug|feature|epic>] [--blocked-by <id>]")
	}

	title := strings.Join(remaining, " ")

	store, err := openStore()
	if err != nil {
		return err
	}
	defer store.Close()

	issue := NewIssue(title)
	issue.Description = *description
	issue.Priority = *priority
	issue.Type = IssueType(*issueType)

	if err := store.CreateIssue(issue); err != nil {
		return fmt.Errorf("failed to create issue: %w", err)
	}

	// Add dependencies if specified
	if err := addBlockers(store, issue.ID, *blockedBy); err != nil {
		return err
	}

	fmt.Fprintf(w, "Created %s: %s\n", issue.ID, issue.Title)
	return nil
}

// cmdList lists all issues
func cmdList(args []string, w io.Writer) error {
	fs := flag.NewFlagSet("list", flag.ContinueOnError)
	fs.SetOutput(w)
	jsonFlag := fs.Bool("json", false, "Output as JSONL")
	treeFlag := fs.Bool("tree", false, "Show dependency tree")
	statusFilter := fs.String("status", "", "Filter by status (open, in_progress, closed)")
	priorityFilter := fs.Int("priority", -1, "Filter by priority (0-4)")
	typeFilter := fs.String("type", "", "Filter by type (task, bug, feature, epic)")
	resolutionFilter := fs.String("resolution", "", "Filter by resolution (done, wontfix, duplicate)")

	if err := fs.Parse(args); err != nil {
		return err
	}

	// Validate filter values before opening store
	if err := validateFilters(*statusFilter, *priorityFilter, *typeFilter, *resolutionFilter); err != nil {
		return err
	}

	store, err := openStore()
	if err != nil {
		return err
	}
	defer store.Close()

	issues, err := store.ListIssues()
	if err != nil {
		return fmt.Errorf("failed to list issues: %w", err)
	}

	// Apply filters
	issues = filterIssues(issues, *statusFilter, *priorityFilter, *typeFilter, *resolutionFilter)

	return outputIssues(store, issues, w, *jsonFlag, *treeFlag)
}

// formatIssueLine returns a formatted string for displaying an issue in list/ready output.
func formatIssueLine(issue *Issue) string {
	return fmt.Sprintf("%s  %-11s  P%d  %s  %s",
		issue.ID, issue.Status, issue.Priority, issue.Type, issue.Title)
}

// outputIssues handles the common output logic for list and ready commands.
func outputIssues(store *Store, issues []*Issue, w io.Writer, jsonOut, treeOut bool) error {
	if len(issues) == 0 {
		if jsonOut {
			return nil
		}
		fmt.Fprintln(w, "No issues found")
		return nil
	}

	if jsonOut {
		return outputIssuesJSON(store, issues, w)
	}

	if treeOut {
		return outputIssuesTree(store, issues, w)
	}

	for _, issue := range issues {
		fmt.Fprintln(w, formatIssueLine(issue))
	}
	return nil
}

// addBlockers adds blocker dependencies for an issue, validating that each blocker exists
// and preventing self-references.
func addBlockers(store *Store, issueID string, blockerIDs []string) error {
	for _, blockerID := range blockerIDs {
		if blockerID == issueID {
			return errors.New("issue cannot block itself")
		}
		if _, err := store.GetIssue(blockerID); err != nil {
			return fmt.Errorf("blocker issue %s: %w", blockerID, err)
		}
		if err := store.AddDependency(issueID, blockerID, DepBlocks); err != nil {
			return fmt.Errorf("blocker issue %s: %w", blockerID, err)
		}
	}
	return nil
}

// filterIssues applies status, priority, type, and resolution filters to a slice of issues.
func filterIssues(issues []*Issue, status string, priority int, issueType string, resolution string) []*Issue {
	if status == "" && priority < 0 && issueType == "" && resolution == "" {
		return issues // no filtering needed
	}

	var filtered []*Issue
	for _, issue := range issues {
		if status != "" && string(issue.Status) != status {
			continue
		}
		if priority >= 0 && issue.Priority != priority {
			continue
		}
		if issueType != "" && string(issue.Type) != issueType {
			continue
		}
		if resolution != "" && string(issue.Resolution) != resolution {
			continue
		}
		filtered = append(filtered, issue)
	}
	return filtered
}

// validateFilters checks that filter values are valid before applying them.
func validateFilters(status string, priority int, issueType string, resolution string) error {
	if status != "" && !Status(status).Valid() {
		return fmt.Errorf("invalid status: %q (valid: open, in_progress, closed)", status)
	}
	if priority >= 0 && priority > 4 {
		return fmt.Errorf("invalid priority: %d (valid: 0-4)", priority)
	}
	if issueType != "" && !IssueType(issueType).Valid() {
		return fmt.Errorf("invalid type: %q (valid: task, bug, feature, epic)", issueType)
	}
	if resolution != "" && !Resolution(resolution).Valid() {
		return fmt.Errorf("invalid resolution: %q (valid: done, wontfix, duplicate)", resolution)
	}
	return nil
}

// cmdShow displays details for a single issue
func cmdShow(args []string, w io.Writer) error {
	fs := flag.NewFlagSet("show", flag.ContinueOnError)
	fs.SetOutput(w)
	jsonOutput := fs.Bool("json", false, "Output as JSON")

	if err := fs.Parse(args); err != nil {
		return err
	}

	remaining := fs.Args()
	if len(remaining) == 0 {
		return errors.New("usage: bl show <id> [--json]")
	}
	id := remaining[0]

	store, err := openStore()
	if err != nil {
		return err
	}
	defer store.Close()

	issue, err := store.GetIssue(id)
	if err != nil {
		return fmt.Errorf("issue %s: %w", id, err)
	}

	if *jsonOutput {
		deps, err := store.GetDependencies(id)
		if err != nil {
			return fmt.Errorf("get dependencies: %w", err)
		}
		return outputSingleIssueJSON(issue, deps, w)
	}

	fmt.Fprintf(w, "ID:       %s\n", issue.ID)
	fmt.Fprintf(w, "Title:    %s\n", issue.Title)
	fmt.Fprintf(w, "Status:   %s\n", issue.Status)
	fmt.Fprintf(w, "Priority: P%d\n", issue.Priority)
	fmt.Fprintf(w, "Type:     %s\n", issue.Type)
	if issue.Description != "" {
		fmt.Fprintf(w, "Description: %s\n", issue.Description)
	}
	fmt.Fprintf(w, "Created:  %s\n", issue.CreatedAt.Format("2006-01-02 15:04:05"))
	fmt.Fprintf(w, "Updated:  %s\n", issue.UpdatedAt.Format("2006-01-02 15:04:05"))
	if issue.ClosedAt != nil {
		fmt.Fprintf(w, "Closed:   %s\n", issue.ClosedAt.Format("2006-01-02 15:04:05"))
	}
	if issue.Resolution != "" {
		fmt.Fprintf(w, "Resolution: %s\n", issue.Resolution)
	}

	// Show dependencies
	deps, err := store.GetDependencies(id)
	if err == nil && len(deps) > 0 {
		fmt.Fprintln(w, "\nDependencies:")
		for _, dep := range deps {
			fmt.Fprintf(w, "  %s %s\n", dep.Type, dep.DependsOnID)
		}
	}

	return nil
}

// cmdUpdate modifies an existing issue
func cmdUpdate(args []string, w io.Writer) error {
	if len(args) == 0 {
		return errors.New("usage: bl update <id> [--title <text>] [--status <open|in_progress|closed>] [--priority <0-4>] [--type <task|bug|feature|epic>] [--description <text>] [--blocked-by <id>] [--unblock <id>]")
	}

	id := args[0]
	flagArgs := args[1:]

	fs := flag.NewFlagSet("update", flag.ContinueOnError)
	fs.SetOutput(w)
	title := fs.String("title", "", "New title")
	status := fs.String("status", "", "New status")
	priority := fs.Int("priority", -1, "New priority (0-4)")
	issueType := fs.String("type", "", "New type")
	description := fs.String("description", "", "New description")
	addBlockersFlag := fs.StringSlice("blocked-by", nil, "Add blocker (repeatable)")
	rmBlockers := fs.StringSlice("unblock", nil, "Remove blocker (repeatable)")

	if err := fs.Parse(flagArgs); err != nil {
		return err
	}

	store, err := openStore()
	if err != nil {
		return err
	}
	defer store.Close()

	issue, err := store.GetIssue(id)
	if err != nil {
		return fmt.Errorf("issue %s: %w", id, err)
	}

	// Validate inputs before applying changes
	if *status != "" && !Status(*status).Valid() {
		return fmt.Errorf("invalid status: %q (valid: open, in_progress, closed)", *status)
	}
	if *priority >= 0 && *priority > 4 {
		return fmt.Errorf("invalid priority: %d (valid: 0-4)", *priority)
	}
	if *issueType != "" && !IssueType(*issueType).Valid() {
		return fmt.Errorf("invalid type: %q (valid: task, bug, feature, epic)", *issueType)
	}

	if *title != "" {
		issue.Title = *title
	}
	if *status != "" {
		issue.Status = Status(*status)
	}
	if *priority >= 0 {
		issue.Priority = *priority
	}
	if *issueType != "" {
		issue.Type = IssueType(*issueType)
	}
	if fs.Changed("description") {
		issue.Description = *description
	}

	if err := store.UpdateIssue(issue); err != nil {
		return fmt.Errorf("failed to update: %w", err)
	}

	// Handle blocker additions
	if err := addBlockers(store, id, *addBlockersFlag); err != nil {
		return err
	}

	// Handle blocker removals
	for _, blockerID := range *rmBlockers {
		if err := store.RemoveDependency(id, blockerID, DepBlocks); err != nil {
			return fmt.Errorf("blocker issue %s: %w", blockerID, err)
		}
	}

	fmt.Fprintf(w, "Updated %s: %s\n", id, issue.Title)
	return nil
}

// cmdDelete permanently removes an issue
func cmdDelete(args []string, w io.Writer) error {
	fs := flag.NewFlagSet("delete", flag.ContinueOnError)
	fs.SetOutput(w)
	confirm := fs.Bool("confirm", false, "Confirm deletion")

	if err := fs.Parse(args); err != nil {
		return err
	}

	remaining := fs.Args()
	if len(remaining) == 0 {
		return errors.New("usage: bl delete <id> --confirm")
	}
	id := remaining[0]

	if !*confirm {
		return errors.New("delete requires --confirm flag")
	}

	store, err := openStore()
	if err != nil {
		return err
	}
	defer store.Close()

	// Get issue first to show what was deleted
	issue, err := store.GetIssue(id)
	if err != nil {
		return fmt.Errorf("issue %s: %w", id, err)
	}

	if err := store.DeleteIssue(id); err != nil {
		return fmt.Errorf("failed to delete: %w", err)
	}

	fmt.Fprintf(w, "Deleted %s: %s\n", id, issue.Title)
	return nil
}

// cmdClose closes an issue
func cmdClose(args []string, w io.Writer) error {
	fs := flag.NewFlagSet("close", flag.ContinueOnError)
	resolutionFlag := fs.String("resolution", "done", "Resolution reason (done, wontfix, duplicate)")
	fs.SetOutput(w)

	if err := fs.Parse(args); err != nil {
		return err
	}

	if fs.NArg() == 0 {
		return errors.New("usage: bl close <id> [--resolution <done|wontfix|duplicate>]")
	}

	id := fs.Arg(0)
	resolution := Resolution(*resolutionFlag)

	if !resolution.Valid() {
		return fmt.Errorf("invalid resolution: %q (must be done, wontfix, or duplicate)", *resolutionFlag)
	}

	store, err := openStore()
	if err != nil {
		return err
	}
	defer store.Close()

	// Verify issue exists first
	issue, err := store.GetIssue(id)
	if err != nil {
		return fmt.Errorf("issue %s: %w", id, err)
	}

	if err := store.CloseIssue(id, resolution); err != nil {
		return fmt.Errorf("failed to close: %w", err)
	}

	fmt.Fprintf(w, "Closed %s: %s\n", id, issue.Title)
	return nil
}

// cmdReady lists issues that are ready to work on (not blocked)
func cmdReady(args []string, w io.Writer) error {
	fs := flag.NewFlagSet("ready", flag.ContinueOnError)
	fs.SetOutput(w)
	jsonFlag := fs.Bool("json", false, "Output as JSONL")
	treeFlag := fs.Bool("tree", false, "Show dependency tree")
	priorityFilter := fs.Int("priority", -1, "Filter by priority (0-4)")
	typeFilter := fs.String("type", "", "Filter by type (task, bug, feature, epic)")

	if err := fs.Parse(args); err != nil {
		return err
	}

	// Validate filter values before opening store (no status/resolution for ready)
	if err := validateFilters("", *priorityFilter, *typeFilter, ""); err != nil {
		return err
	}

	store, err := openStore()
	if err != nil {
		return err
	}
	defer store.Close()

	issues, err := store.GetReadyWork()
	if err != nil {
		return fmt.Errorf("failed to get ready work: %w", err)
	}

	// Apply filters (no status/resolution filter - ready work is already filtered to open/in_progress)
	issues = filterIssues(issues, "", *priorityFilter, *typeFilter, "")

	return outputIssues(store, issues, w, *jsonFlag, *treeFlag)
}

// cmdExport exports all issues to JSONL format
func cmdExport(args []string, w io.Writer) error {
	store, err := openStore()
	if err != nil {
		return err
	}
	defer store.Close()

	// If file argument provided, write to file
	if len(args) > 0 {
		filePath := args[0]
		if err := ExportToFile(store, filePath); err != nil {
			return fmt.Errorf("export failed: %w", err)
		}
		fmt.Fprintf(w, "Exported to %s\n", filePath)
		return nil
	}

	// Otherwise write to stdout
	return ExportToJSONL(store, w)
}

// cmdImport imports issues from a JSONL file
func cmdImport(args []string, w io.Writer) error {
	if len(args) == 0 {
		return errors.New("usage: bl import <file>")
	}

	filePath := args[0]

	store, err := openStore()
	if err != nil {
		return err
	}
	defer store.Close()

	stats, err := ImportFromFile(store, filePath)
	if err != nil {
		return fmt.Errorf("import failed: %w", err)
	}

	fmt.Fprintf(w, "Imported: %d created, %d updated\n", stats.Created, stats.Updated)
	return nil
}

// outputIssuesJSON outputs issues as JSONL (one JSON object per line)
func outputIssuesJSON(store *Store, issues []*Issue, w io.Writer) error {
	// Batch-fetch all dependencies to avoid N+1 queries
	allDeps, err := store.GetAllDependencies()
	if err != nil {
		return fmt.Errorf("get all dependencies: %w", err)
	}

	return WriteIssuesAsJSONL(issues, allDeps, w)
}

// outputSingleIssueJSON outputs a single issue as JSON (not JSONL)
func outputSingleIssueJSON(issue *Issue, deps []*Dependency, w io.Writer) error {
	export := toIssueExport(issue, deps)
	encoder := json.NewEncoder(w)
	return encoder.Encode(export)
}

// sortByPriorityThenID sorts issues by priority (ascending) then by ID (alphabetical).
func sortByPriorityThenID(issues []*Issue) {
	sort.Slice(issues, func(i, j int) bool {
		if issues[i].Priority != issues[j].Priority {
			return issues[i].Priority < issues[j].Priority
		}
		return issues[i].ID < issues[j].ID
	})
}

// outputIssuesTree renders issues as a dependency tree
func outputIssuesTree(store *Store, issues []*Issue, w io.Writer) error {
	allDeps, err := store.GetAllDependencies()
	if err != nil {
		return fmt.Errorf("failed to get dependencies: %w", err)
	}

	// Build tree structure: roots are issues not blocked by any open issue
	// Children are issues that ARE blocked by open issues
	issueMap := make(map[string]*Issue)
	for _, issue := range issues {
		issueMap[issue.ID] = issue
	}

	// Identify children: issues that have an OPEN blocker in our list
	// The blocker becomes the parent in the tree
	children := make(map[string][]*Issue) // parent ID -> children
	isChild := make(map[string]bool)

	for _, dep := range allDeps {
		for _, d := range dep {
			if d.Type != DepBlocks {
				continue
			}
			// d.IssueID is blocked by d.DependsOnID
			// So d.DependsOnID is the parent, d.IssueID is the child
			child, childOk := issueMap[d.IssueID]
			parent, parentOk := issueMap[d.DependsOnID]
			if !childOk || !parentOk {
				continue
			}
			// Only count as child if parent is open (not closed)
			if parent.Status != StatusClosed {
				children[d.DependsOnID] = append(children[d.DependsOnID], child)
				isChild[d.IssueID] = true
			}
		}
	}

	// Roots are issues that aren't children of any open issue
	var roots []*Issue
	for _, issue := range issues {
		if !isChild[issue.ID] {
			roots = append(roots, issue)
		}
	}

	// Sort roots by priority then ID for deterministic output
	sortByPriorityThenID(roots)

	// Render tree
	for _, root := range roots {
		fmt.Fprintln(w, formatIssueLine(root))
		printTree(w, children, root.ID, "")
	}

	return nil
}

// printTree recursively prints children with tree-drawing characters
func printTree(w io.Writer, children map[string][]*Issue, parentID string, prefix string) {
	kids := children[parentID]
	sortByPriorityThenID(kids)

	for i, child := range kids {
		isLast := i == len(kids)-1
		connector := "├── "
		if isLast {
			connector = "└── "
		}
		fmt.Fprintf(w, "%s%s%s\n", prefix, connector, formatIssueLine(child))

		extension := "│   "
		if isLast {
			extension = "    "
		}
		printTree(w, children, child.ID, prefix+extension)
	}
}

// cmdOnboard prints Claude Code integration instructions
func cmdOnboard(w io.Writer) error {
	const instructions = `# beads-lite

This project uses beads-lite for task tracking. You MUST use it to track work.

## Required Workflow

1. Run ` + "`bl ready`" + ` at session start to see available work
2. When you start working on a task: ` + "`bl update <id> --status in_progress`" + `
3. When you discover new work, create a task: ` + "`bl create \"description\"`" + `
4. When tasks depend on each other: ` + "`bl update <id> --blocked-by <blocker>`" + `
5. When you complete work: ` + "`bl close <id>`" + `

## Commands

` + "```" + `
bl ready              # what can I work on now?
bl ready --json       # machine-readable output
bl list               # all tasks
bl list --tree        # dependency visualization
bl list --status in_progress  # see what's being worked on
bl create "title"     # new task
bl update <id> --status in_progress  # claim work
bl close <id>         # complete task (resolution: done)
bl close <id> --resolution wontfix   # close as won't fix
bl close <id> --resolution duplicate # close as duplicate
bl update <a> --blocked-by <b>       # a blocked by b
bl show <id>          # task details
bl list --status closed --resolution wontfix  # filter by resolution
` + "```" + `

## Closing Tasks

When closing tasks, specify WHY with --resolution:
- ` + "`done`" + ` (default): Work completed successfully
- ` + "`wontfix`" + `: Intentionally rejected (document reasoning in description)
- ` + "`duplicate`" + `: Duplicate of another issue

Use ` + "`bl list --status closed --resolution wontfix`" + ` to review rejected ideas.

## Epic Workflow

Epics group related tasks. Use blockers for actual work dependencies, not organization.

` + "```" + `
# Create epic to track a feature
bl create "User authentication" --type epic

# Create tasks for the epic (work on them immediately)
bl create "Add login endpoint"
bl create "Add session storage"
bl create "Add logout endpoint"

# If tasks have real dependencies, add blockers
bl update <logout-id> --blocked-by <login-id>

# View all work
bl list --tree

# Close tasks as completed, close epic when feature is done
bl close <epic-id>
` + "```" + `

## Rules

- Always check ` + "`bl ready`" + ` before starting work
- Mark tasks ` + "`in_progress`" + ` when you start working on them
- Create tasks for any new work you discover
- Close tasks when complete - this unblocks dependent tasks
- Use ` + "`--json`" + ` flag when you need to parse output programmatically
`
	fmt.Fprint(w, instructions)
	return nil
}

func cmdVersion(w io.Writer) error {
	fmt.Fprintf(w, "bl version %s\n", Version)
	return nil
}

func cmdUpgrade(w io.Writer) error {
	const repo = "kylesnowschwartz/beads-lite"

	// Get latest release version
	resp, err := http.Get("https://api.github.com/repos/" + repo + "/releases/latest")
	if err != nil {
		return fmt.Errorf("failed to check for updates: %w", err)
	}
	defer resp.Body.Close()

	var release struct {
		TagName string `json:"tag_name"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&release); err != nil {
		return fmt.Errorf("failed to parse release info: %w", err)
	}

	latest := release.TagName
	if latest == Version {
		fmt.Fprintf(w, "Already at latest version %s\n", Version)
		return nil
	}

	fmt.Fprintf(w, "Upgrading from %s to %s...\n", Version, latest)

	// Determine platform
	goos := runtime.GOOS
	goarch := runtime.GOARCH
	tarball := fmt.Sprintf("beads-lite_%s_%s.tar.gz", goos, goarch)
	url := fmt.Sprintf("https://github.com/%s/releases/download/%s/%s", repo, latest, tarball)

	// Download tarball
	resp, err = http.Get(url)
	if err != nil {
		return fmt.Errorf("failed to download: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("download failed: %s", resp.Status)
	}

	// Get current executable path
	execPath, err := os.Executable()
	if err != nil {
		return fmt.Errorf("failed to get executable path: %w", err)
	}

	// Resolve symlinks to get real path
	execPath, err = filepath.EvalSymlinks(execPath)
	if err != nil {
		return fmt.Errorf("failed to resolve executable path: %w", err)
	}

	// Create temp file for tarball
	tmpFile, err := os.CreateTemp("", "bl-upgrade-*.tar.gz")
	if err != nil {
		return fmt.Errorf("failed to create temp file: %w", err)
	}
	defer os.Remove(tmpFile.Name())

	if _, err := io.Copy(tmpFile, resp.Body); err != nil {
		tmpFile.Close()
		return fmt.Errorf("failed to download: %w", err)
	}
	tmpFile.Close()

	// Extract and replace
	tmpDir, err := os.MkdirTemp("", "bl-upgrade-")
	if err != nil {
		return fmt.Errorf("failed to create temp dir: %w", err)
	}
	defer os.RemoveAll(tmpDir)

	// Use tar command to extract (simpler than implementing tar in Go)
	cmd := exec.Command("tar", "-xzf", tmpFile.Name(), "-C", tmpDir)
	if err := cmd.Run(); err != nil {
		return fmt.Errorf("failed to extract: %w", err)
	}

	// Replace executable
	newBinary := filepath.Join(tmpDir, "bl")
	if err := os.Rename(newBinary, execPath); err != nil {
		// Try copy if rename fails (cross-device)
		src, err := os.Open(newBinary)
		if err != nil {
			return fmt.Errorf("failed to open new binary: %w", err)
		}
		defer src.Close()

		dst, err := os.OpenFile(execPath, os.O_WRONLY|os.O_TRUNC, 0755)
		if err != nil {
			return fmt.Errorf("failed to open executable for writing: %w", err)
		}
		defer dst.Close()

		if _, err := io.Copy(dst, src); err != nil {
			return fmt.Errorf("failed to write new binary: %w", err)
		}
	}

	fmt.Fprintf(w, "Upgraded to %s\n", latest)
	return nil
}
