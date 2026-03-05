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
	"strconv"
	"strings"
	"time"

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
	case "claim":
		return cmdClaim(cmdArgs, w)
	case "unclaim":
		return cmdUnclaim(cmdArgs, w)
	case "agent-state":
		return cmdAgentState(cmdArgs, w)
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
  claim <id>            Atomically claim an issue (for multi-agent use)
  unclaim <id>          Release a claimed issue
  agent-state <id>      Set or query agent liveness state
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
  -p0, -p1, -p2, -p3, -p4  Shorthand priority filter (e.g. -p1 == --priority 1)
  --type <string>       Filter by type (task, bug, feature, epic)
  --assigned-to <name>  Filter by assignee

Ready-Only Flags:
  --unclaimed           Only show tasks not claimed by any agent

List-Only Flags:
  --status <string>     Filter by status (backlog, todo, doing, review, done)
  --resolution <string> Filter by resolution (done, wontfix, duplicate)

Show Flags:
  --json                Output as JSON
  --tree                Show subtree of downstream dependents

Create Flags:
  --description <text>  Issue description
  --priority <int>      Priority (0-4), default 2
  -p0, -p1, -p2, -p3, -p4  Shorthand priority (e.g. -p1 == --priority 1)
  --type <string>       Type (task, bug, feature, epic), default task
  --blocked-by <id>     Issue ID that blocks this (repeatable)
  --epic <id>           Parent epic (groups under epic without blocking)
  --spec <text>         Add specification (repeatable, creates unchecked)

Update Flags:
  --title <string>      New title
  --status <string>     New status (backlog, todo, doing, review, done)
  --priority <int>      New priority (0-4)
  -p0, -p1, -p2, -p3, -p4  Shorthand priority (e.g. -p1 == --priority 1)
  --type <string>       New type (task, bug, feature, epic)
  --description <text>  New description
  --blocked-by <id>     Add blocker (repeatable)
  --unblock <id>        Remove blocker (repeatable)
  --epic <id>           Set parent epic (groups under epic without blocking)
  --remove-epic <id>    Remove parent epic link
  --spec <text>         Add specification (repeatable, creates unchecked)
  --check-spec <n>      Check specification by index (1-based, repeatable)
  --uncheck-spec <n>    Uncheck specification by index (1-based, repeatable)
  --remove-spec <n>     Remove specification by index (1-based, repeatable)

Claim Flags:
  --agent <name>        Agent name (required)

Close Flags:
  --resolution <string> Resolution (done, wontfix, duplicate), default done

Delete Flags:
  --confirm             Required to confirm permanent deletion

Agent-State Flags:
  --state <string>      Agent state to set (idle, running, stuck, done, dead)
  --list                List issues matching the given --state`)
}

func getDBPath() string {
	if root := os.Getenv("BL_ROOT"); root != "" {
		return filepath.Join(root, beadsDir, dbName)
	}
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
	dir := beadsDir
	if root := os.Getenv("BL_ROOT"); root != "" {
		dir = filepath.Join(root, beadsDir)
	}
	if err := os.MkdirAll(dir, 0755); err != nil {
		return fmt.Errorf("failed to create %s: %w", dir, err)
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
	priorityStr := fs.String("priority", "2", "Priority (0-4 or p0-p4)")
	issueType := fs.String("type", "task", "Type (task, bug, feature, epic)")
	blockedBy := fs.StringSlice("blocked-by", nil, "Issue ID that blocks this (repeatable)")
	epicID := fs.String("epic", "", "Parent epic ID (groups under epic without blocking)")
	specs := fs.StringSlice("spec", nil, "Specification text (repeatable, creates unchecked)")
	var sh priorityShorthands
	addPriorityShorthands(fs, &sh)

	if err := fs.Parse(expandPriorityShorthands(args)); err != nil {
		return err
	}

	longPriority, err := parsePriority(*priorityStr)
	if err != nil {
		return err
	}

	priority, err := resolvePriorityShorthands(sh, fs.Changed("priority"), longPriority)
	if err != nil {
		return err
	}
	// For create, a missing --priority means use the default (2), not "no filter".
	// parsePriority returns -1 for empty string, but the default is "2" so this
	// only triggers when the user explicitly clears the flag — treat -1 as default.
	if priority < 0 {
		priority = 2
	}

	remaining := fs.Args()
	if len(remaining) == 0 {
		return errors.New("usage: bl create <title> [--description <text>] [--priority <0-4>] [--type <task|bug|feature|epic>] [--blocked-by <id>] [--epic <id>] [--spec <text>]")
	}

	title := strings.Join(remaining, " ")

	store, err := openStore()
	if err != nil {
		return err
	}
	defer store.Close()

	issue := NewIssue(title)
	issue.Status = StatusTodo // CLI-created tasks are actionable by default
	issue.Description = *description
	issue.Priority = priority
	issue.Type = IssueType(*issueType)
	for _, text := range *specs {
		issue.Specifications = append(issue.Specifications, Spec{Text: text})
	}

	if err := store.CreateIssue(issue); err != nil {
		return fmt.Errorf("failed to create issue: %w", err)
	}

	// Add dependencies if specified
	if err := addBlockers(store, issue.ID, *blockedBy); err != nil {
		return err
	}

	// Link to parent epic
	if err := addParent(store, issue.ID, *epicID); err != nil {
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
	statusFilter := fs.String("status", "", "Filter by status (backlog, todo, doing, review, done)")
	priorityStr := fs.String("priority", "", "Filter by priority (0-4 or p0-p4)")
	typeFilter := fs.String("type", "", "Filter by type (task, bug, feature, epic)")
	resolutionFilter := fs.String("resolution", "", "Filter by resolution (done, wontfix, duplicate)")
	assignedToFilter := fs.String("assigned-to", "", "Filter by assignee")
	var sh priorityShorthands
	addPriorityShorthands(fs, &sh)

	if err := fs.Parse(expandPriorityShorthands(args)); err != nil {
		return err
	}

	longPriorityFilter, err := parsePriority(*priorityStr)
	if err != nil {
		return err
	}

	priorityFilter, err := resolvePriorityShorthands(sh, fs.Changed("priority"), longPriorityFilter)
	if err != nil {
		return err
	}

	// Validate filter values before opening store
	if err := validateFilters(*statusFilter, priorityFilter, *typeFilter, *resolutionFilter); err != nil {
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
	issues = filterIssues(issues, issueFilter{
		status:     *statusFilter,
		priority:   priorityFilter,
		issueType:  *typeFilter,
		resolution: *resolutionFilter,
		assignedTo: *assignedToFilter,
	})

	return outputIssues(store, issues, nil, w, *jsonFlag, *treeFlag)
}

// formatIssueLine returns a formatted string for displaying an issue in list/ready output.
func formatIssueLine(issue *Issue) string {
	if issue.AssignedTo != "" {
		return fmt.Sprintf("%s  %-7s  P%d  %s  [%s]  %s",
			issue.ID, issue.Status, issue.Priority, issue.Type, issue.AssignedTo, issue.Title)
	}
	return fmt.Sprintf("%s  %-7s  P%d  %s  %s",
		issue.ID, issue.Status, issue.Priority, issue.Type, issue.Title)
}

// outputIssues handles the common output logic for list and ready commands.
// treeIssues is the expanded set for tree building (pass nil to use issues for both).
func outputIssues(store *Store, issues []*Issue, treeIssues []*Issue, w io.Writer, jsonOut, treeOut bool) error {
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
		return outputIssuesTree(store, issues, treeIssues, w)
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

// addParent links an issue under a parent epic with a non-blocking relationship.
func addParent(store *Store, issueID, epicID string) error {
	if epicID == "" {
		return nil
	}
	if epicID == issueID {
		return errors.New("issue cannot be its own parent")
	}
	parent, err := store.GetIssue(epicID)
	if err != nil {
		return fmt.Errorf("epic %s: %w", epicID, err)
	}
	if parent.Type != IssueTypeEpic {
		return fmt.Errorf("epic %s: issue is type %q, not epic", epicID, parent.Type)
	}
	if err := store.AddDependency(issueID, epicID, DepParent); err != nil {
		return fmt.Errorf("epic %s: %w", epicID, err)
	}
	return nil
}

// issueFilter holds all filter criteria for listing issues.
type issueFilter struct {
	status     string
	priority   int // -1 means no filter
	issueType  string
	resolution string
	assignedTo string
	unclaimed  bool
}

// filterIssues applies filters to a slice of issues.
func filterIssues(issues []*Issue, f issueFilter) []*Issue {
	if f.status == "" && f.priority < 0 && f.issueType == "" && f.resolution == "" && f.assignedTo == "" && !f.unclaimed {
		return issues // no filtering needed
	}

	var filtered []*Issue
	for _, issue := range issues {
		if f.status != "" && string(issue.Status) != f.status {
			continue
		}
		if f.priority >= 0 && issue.Priority != f.priority {
			continue
		}
		if f.issueType != "" && string(issue.Type) != f.issueType {
			continue
		}
		if f.resolution != "" && string(issue.Resolution) != f.resolution {
			continue
		}
		if f.assignedTo != "" && issue.AssignedTo != f.assignedTo {
			continue
		}
		if f.unclaimed && issue.AssignedTo != "" {
			continue
		}
		filtered = append(filtered, issue)
	}
	return filtered
}

// parsePriority accepts "0"-"4" or "p0"-"p4" (case-insensitive).
// Returns -1 for empty string (no filter). Returns error for invalid values.
func parsePriority(s string) (int, error) {
	if s == "" {
		return -1, nil
	}
	// Strip optional p/P prefix
	s = strings.TrimPrefix(strings.ToLower(s), "p")
	n, err := strconv.Atoi(s)
	if err != nil {
		return 0, fmt.Errorf("invalid priority: %q (use 0-4 or p0-p4)", s)
	}
	if n < 0 || n > 4 {
		return 0, fmt.Errorf("invalid priority: %d (valid: 0-4)", n)
	}
	return n, nil
}

// priorityShorthands holds -p0 through -p4 boolean flags for shorthand priority input.
type priorityShorthands struct {
	p0, p1, p2, p3, p4 bool
}

// addPriorityShorthands registers --p0 through --p4 on the given FlagSet.
// These are long-form flags (double-dash) internally; expandPriorityShorthands
// rewrites single-dash -pN to --pN before parsing so the user can type either form.
func addPriorityShorthands(fs *flag.FlagSet, sh *priorityShorthands) {
	fs.BoolVar(&sh.p0, "p0", false, "Shorthand for --priority 0")
	fs.BoolVar(&sh.p1, "p1", false, "Shorthand for --priority 1")
	fs.BoolVar(&sh.p2, "p2", false, "Shorthand for --priority 2")
	fs.BoolVar(&sh.p3, "p3", false, "Shorthand for --priority 3")
	fs.BoolVar(&sh.p4, "p4", false, "Shorthand for --priority 4")
}

// expandPriorityShorthands rewrites -p0 through -p4 to --p0 through --p4 in args
// so pflag's long-flag parser handles them correctly. Other args are unchanged.
func expandPriorityShorthands(args []string) []string {
	out := make([]string, len(args))
	for i, a := range args {
		switch a {
		case "-p0", "-p1", "-p2", "-p3", "-p4":
			out[i] = "-" + a // turn "-p0" into "--p0"
		default:
			out[i] = a
		}
	}
	return out
}

// resolvePriorityShorthands checks -p0 through -p4 flags against --priority.
// Returns the resolved priority value or an error on conflict.
// priorityFlagChanged reports whether --priority was explicitly set by the user.
func resolvePriorityShorthands(sh priorityShorthands, priorityFlagChanged bool, longPriority int) (int, error) {
	// Count how many shorthand flags are set.
	set := []int{}
	if sh.p0 {
		set = append(set, 0)
	}
	if sh.p1 {
		set = append(set, 1)
	}
	if sh.p2 {
		set = append(set, 2)
	}
	if sh.p3 {
		set = append(set, 3)
	}
	if sh.p4 {
		set = append(set, 4)
	}

	if len(set) > 1 {
		return 0, fmt.Errorf("only one -pN flag may be set at a time (got -p%d and -p%d)", set[0], set[1])
	}

	if len(set) == 1 && priorityFlagChanged {
		return 0, fmt.Errorf("-p%d and --priority cannot be used together", set[0])
	}

	if len(set) == 1 {
		return set[0], nil
	}

	// No shorthand — return the long-form value as-is (may be -1 for "no filter").
	return longPriority, nil
}

// validateFilters checks that filter values are valid before applying them.
func validateFilters(status string, priority int, issueType string, resolution string) error {
	if status != "" && !Status(status).Valid() {
		return fmt.Errorf("invalid status: %q (valid: backlog, todo, doing, review, done)", status)
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
	treeFlag := fs.Bool("tree", false, "Show subtree of dependents")

	if err := fs.Parse(args); err != nil {
		return err
	}

	remaining := fs.Args()
	if len(remaining) == 0 {
		return errors.New("usage: bl show <id> [--json] [--tree]")
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

	if *treeFlag {
		// Show this issue as root with all its downstream dependents
		allIssues, err := store.ListIssues()
		if err != nil {
			return fmt.Errorf("failed to list issues: %w", err)
		}
		return outputIssuesTree(store, []*Issue{issue}, allIssues, w)
	}

	fmt.Fprintf(w, "ID:       %s\n", issue.ID)
	fmt.Fprintf(w, "Title:    %s\n", issue.Title)
	fmt.Fprintf(w, "Status:   %s\n", issue.Status)
	fmt.Fprintf(w, "Priority: P%d\n", issue.Priority)
	fmt.Fprintf(w, "Type:     %s\n", issue.Type)
	if issue.AssignedTo != "" {
		fmt.Fprintf(w, "Assigned: %s\n", issue.AssignedTo)
	}
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
	if issue.AgentState != "" {
		fmt.Fprintf(w, "AgentState: %s\n", issue.AgentState)
	}
	if issue.LastActivity != nil {
		fmt.Fprintf(w, "LastActivity: %s\n", issue.LastActivity.Format("2006-01-02 15:04:05"))
	}

	// Show specifications
	if len(issue.Specifications) > 0 {
		checked, total := issue.SpecProgress()
		fmt.Fprintf(w, "\nSpecifications (%d/%d):\n", checked, total)
		for i, spec := range issue.Specifications {
			mark := " "
			if spec.Checked {
				mark = "x"
			}
			fmt.Fprintf(w, "  %d. [%s] %s\n", i+1, mark, spec.Text)
		}
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
		return errors.New("usage: bl update <id> [--title <text>] [--status <backlog|todo|doing|review|done>] [--priority <0-4>] [--type <task|bug|feature|epic>] [--description <text>] [--blocked-by <id>] [--unblock <id>] [--force]")
	}

	id := args[0]
	flagArgs := args[1:]

	fs := flag.NewFlagSet("update", flag.ContinueOnError)
	fs.SetOutput(w)
	title := fs.String("title", "", "New title")
	status := fs.String("status", "", "New status")
	priorityStr := fs.String("priority", "", "New priority (0-4 or p0-p4)")
	issueType := fs.String("type", "", "New type")
	description := fs.String("description", "", "New description")
	addBlockersFlag := fs.StringSlice("blocked-by", nil, "Add blocker (repeatable)")
	rmBlockers := fs.StringSlice("unblock", nil, "Remove blocker (repeatable)")
	epicID := fs.String("epic", "", "Set parent epic (groups under epic without blocking)")
	removeEpic := fs.String("remove-epic", "", "Remove parent epic link")
	addSpecs := fs.StringSlice("spec", nil, "Add specification (repeatable, creates unchecked)")
	checkSpecs := fs.IntSlice("check-spec", nil, "Check specification by index (1-based, repeatable)")
	uncheckSpecs := fs.IntSlice("uncheck-spec", nil, "Uncheck specification by index (1-based, repeatable)")
	removeSpecs := fs.IntSlice("remove-spec", nil, "Remove specification by index (1-based, repeatable)")
	force := fs.Bool("force", false, "Bypass transition gates (e.g. spec requirements)")
	var sh priorityShorthands
	addPriorityShorthands(fs, &sh)

	if err := fs.Parse(expandPriorityShorthands(flagArgs)); err != nil {
		return err
	}

	longPriority, err := parsePriority(*priorityStr)
	if err != nil {
		return err
	}

	priority, err := resolvePriorityShorthands(sh, fs.Changed("priority"), longPriority)
	if err != nil {
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
		return fmt.Errorf("invalid status: %q (valid: backlog, todo, doing, review, done)", *status)
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
	if priority >= 0 {
		issue.Priority = priority
	}
	if *issueType != "" {
		issue.Type = IssueType(*issueType)
	}
	if fs.Changed("description") {
		issue.Description = *description
	}

	// Spec mutations: remove first (highest index first to preserve positions),
	// then toggle checks, then append new specs.
	if len(*removeSpecs) > 0 {
		// Sort descending so removals don't shift earlier indices.
		indices := make([]int, len(*removeSpecs))
		copy(indices, *removeSpecs)
		sort.Sort(sort.Reverse(sort.IntSlice(indices)))
		for _, idx := range indices {
			i := idx - 1 // 1-based -> 0-based
			if i < 0 || i >= len(issue.Specifications) {
				return fmt.Errorf("spec index %d out of range (1-%d)", idx, len(issue.Specifications))
			}
			issue.Specifications = append(issue.Specifications[:i], issue.Specifications[i+1:]...)
		}
	}
	for _, idx := range *checkSpecs {
		i := idx - 1
		if i < 0 || i >= len(issue.Specifications) {
			return fmt.Errorf("spec index %d out of range (1-%d)", idx, len(issue.Specifications))
		}
		issue.Specifications[i].Checked = true
	}
	for _, idx := range *uncheckSpecs {
		i := idx - 1
		if i < 0 || i >= len(issue.Specifications) {
			return fmt.Errorf("spec index %d out of range (1-%d)", idx, len(issue.Specifications))
		}
		issue.Specifications[i].Checked = false
	}
	for _, text := range *addSpecs {
		issue.Specifications = append(issue.Specifications, Spec{Text: text})
	}

	if *status != "" && !*force {
		policy := TransitionPolicy{RequireSpecsForReview: true}
		if err := ValidateTransition(issue, issue.Status, policy); err != nil {
			return fmt.Errorf("transition blocked: %w (use --force to override)", err)
		}
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

	// Handle epic linking
	if err := addParent(store, id, *epicID); err != nil {
		return err
	}

	// Handle epic unlinking
	if *removeEpic != "" {
		if err := store.RemoveDependency(id, *removeEpic, DepParent); err != nil {
			return fmt.Errorf("remove epic %s: %w", *removeEpic, err)
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

// cmdClaim atomically claims an issue for an agent.
func cmdClaim(args []string, w io.Writer) error {
	fs := flag.NewFlagSet("claim", flag.ContinueOnError)
	fs.SetOutput(w)
	agent := fs.String("agent", "", "Agent name (required)")

	if err := fs.Parse(args); err != nil {
		return err
	}

	if fs.NArg() == 0 {
		return errors.New("usage: bl claim <id> --agent <name>")
	}
	if *agent == "" {
		return errors.New("--agent is required")
	}

	id := fs.Arg(0)

	store, err := openStore()
	if err != nil {
		return err
	}
	defer store.Close()

	// Verify issue exists
	issue, err := store.GetIssue(id)
	if err != nil {
		return fmt.Errorf("issue %s: %w", id, err)
	}

	claimed, err := store.ClaimIssue(id, *agent)
	if err != nil {
		return fmt.Errorf("failed to claim: %w", err)
	}
	if !claimed {
		return fmt.Errorf("already claimed by %s", issue.AssignedTo)
	}

	fmt.Fprintf(w, "Claimed %s for %s: %s\n", id, *agent, issue.Title)
	return nil
}

// cmdUnclaim releases a claimed issue.
func cmdUnclaim(args []string, w io.Writer) error {
	if len(args) == 0 {
		return errors.New("usage: bl unclaim <id>")
	}

	id := args[0]

	store, err := openStore()
	if err != nil {
		return err
	}
	defer store.Close()

	issue, err := store.GetIssue(id)
	if err != nil {
		return fmt.Errorf("issue %s: %w", id, err)
	}

	if err := store.UnclaimIssue(id); err != nil {
		return fmt.Errorf("failed to unclaim: %w", err)
	}

	fmt.Fprintf(w, "Unclaimed %s: %s\n", id, issue.Title)
	return nil
}

// cmdAgentState sets or queries the agent_state field on an issue.
// Without --list, it updates the state of a single issue.
// With --list, it lists all issues in the given state (no id required).
func cmdAgentState(args []string, w io.Writer) error {
	fs := flag.NewFlagSet("agent-state", flag.ContinueOnError)
	fs.SetOutput(w)
	stateFlag := fs.String("state", "", "Agent state (idle, running, stuck, done, dead)")
	listFlag := fs.Bool("list", false, "List issues matching --state")

	if err := fs.Parse(args); err != nil {
		return err
	}

	if *stateFlag == "" {
		return errors.New("usage: bl agent-state <id> --state <idle|running|stuck|done|dead>\n       bl agent-state --state <state> --list")
	}

	state := AgentState(*stateFlag)
	if !state.Valid() || state == "" {
		return fmt.Errorf("invalid agent state: %q (valid: idle, running, stuck, done, dead)", *stateFlag)
	}

	store, err := openStore()
	if err != nil {
		return err
	}
	defer store.Close()

	if *listFlag {
		issues, err := store.GetAgentsByState(state)
		if err != nil {
			return fmt.Errorf("failed to list agents by state: %w", err)
		}
		if len(issues) == 0 {
			fmt.Fprintln(w, "No issues found")
			return nil
		}
		for _, issue := range issues {
			fmt.Fprintln(w, formatIssueLine(issue))
		}
		return nil
	}

	remaining := fs.Args()
	if len(remaining) == 0 {
		return errors.New("usage: bl agent-state <id> --state <idle|running|stuck|done|dead>")
	}
	id := remaining[0]

	// Verify issue exists before updating state
	issue, err := store.GetIssue(id)
	if err != nil {
		return fmt.Errorf("issue %s: %w", id, err)
	}

	now := time.Now()
	if err := store.SetAgentState(id, state, &now); err != nil {
		return fmt.Errorf("failed to set agent state: %w", err)
	}

	fmt.Fprintf(w, "Updated %s agent_state=%s: %s\n", id, state, issue.Title)
	return nil
}

// cmdReady lists issues that are ready to work on (not blocked)
func cmdReady(args []string, w io.Writer) error {
	fs := flag.NewFlagSet("ready", flag.ContinueOnError)
	fs.SetOutput(w)
	jsonFlag := fs.Bool("json", false, "Output as JSONL")
	treeFlag := fs.Bool("tree", false, "Show dependency tree")
	priorityStr := fs.String("priority", "", "Filter by priority (0-4 or p0-p4)")
	typeFilter := fs.String("type", "", "Filter by type (task, bug, feature, epic)")
	unclaimedFlag := fs.Bool("unclaimed", false, "Only show unclaimed tasks")
	assignedToFilter := fs.String("assigned-to", "", "Filter by assignee")
	var sh priorityShorthands
	addPriorityShorthands(fs, &sh)

	if err := fs.Parse(expandPriorityShorthands(args)); err != nil {
		return err
	}

	longPriorityFilter, err := parsePriority(*priorityStr)
	if err != nil {
		return err
	}

	priorityFilter, err := resolvePriorityShorthands(sh, fs.Changed("priority"), longPriorityFilter)
	if err != nil {
		return err
	}

	// Validate filter values before opening store (no status/resolution for ready)
	if err := validateFilters("", priorityFilter, *typeFilter, ""); err != nil {
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
	issues = filterIssues(issues, issueFilter{
		priority:   priorityFilter,
		issueType:  *typeFilter,
		assignedTo: *assignedToFilter,
		unclaimed:  *unclaimedFlag,
	})

	// For tree view, fetch all open issues so blocked children appear
	// under their ready parents (otherwise tree would be flat)
	var treeIssues []*Issue
	if *treeFlag {
		allIssues, err := store.ListIssues()
		if err != nil {
			return fmt.Errorf("failed to list issues for tree: %w", err)
		}
		// Only include non-closed issues in the tree
		for _, issue := range allIssues {
			if issue.Status != StatusDone {
				treeIssues = append(treeIssues, issue)
			}
		}
	}

	return outputIssues(store, issues, treeIssues, w, *jsonFlag, *treeFlag)
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

// outputIssuesTree renders issues as a dependency tree.
// The issues slice determines which issues appear; treeIssues provides
// the full set used for building parent-child relationships. When treeIssues
// is nil, issues is used for both (the list --tree behavior).
// When treeIssues is set (ready --tree), it allows blocked children to
// appear under their ready parents.
func outputIssuesTree(store *Store, issues []*Issue, treeIssues []*Issue, w io.Writer) error {
	allDeps, err := store.GetAllDependencies()
	if err != nil {
		return fmt.Errorf("failed to get dependencies: %w", err)
	}

	if treeIssues == nil {
		treeIssues = issues
	}

	// Build map of ALL issues available for tree building
	treeMap := make(map[string]*Issue)
	for _, issue := range treeIssues {
		treeMap[issue.ID] = issue
	}

	// Also build a set of the requested issues (for root selection in list mode)
	requestedSet := make(map[string]bool)
	for _, issue := range issues {
		requestedSet[issue.ID] = true
	}

	// Identify children: issues blocked by an open parent
	children := make(map[string][]*Issue) // parent ID -> children
	isChild := make(map[string]bool)

	for _, dep := range allDeps {
		for _, d := range dep {
			if d.Type != DepBlocks && d.Type != DepParent {
				continue
			}
			// d.IssueID depends on d.DependsOnID (parent in tree)
			child, childOk := treeMap[d.IssueID]
			parent, parentOk := treeMap[d.DependsOnID]
			if !childOk || !parentOk {
				continue
			}
			if parent.Status != StatusDone {
				children[d.DependsOnID] = append(children[d.DependsOnID], child)
				isChild[d.IssueID] = true
			}
		}
	}

	// Roots: requested issues that aren't children of any open issue in the tree
	var roots []*Issue
	for _, issue := range issues {
		if !isChild[issue.ID] {
			roots = append(roots, issue)
		}
	}

	sortByPriorityThenID(roots)

	printed := make(map[string]bool)
	for _, root := range roots {
		fmt.Fprintln(w, formatIssueLine(root))
		printed[root.ID] = true
		printTree(w, children, root.ID, "", printed)
	}

	return nil
}

// printTree recursively prints children with tree-drawing characters.
// The printed set prevents duplicate subtrees when an issue has multiple
// parent edges (e.g., both a parent epic link and a blocks dependency).
func printTree(w io.Writer, children map[string][]*Issue, parentID string, prefix string, printed map[string]bool) {
	kids := children[parentID]
	sortByPriorityThenID(kids)

	for i, child := range kids {
		if printed[child.ID] {
			continue
		}

		// Check if any later sibling is still unprinted
		isLast := true
		for _, later := range kids[i+1:] {
			if !printed[later.ID] {
				isLast = false
				break
			}
		}

		connector := "├── "
		if isLast {
			connector = "└── "
		}
		fmt.Fprintf(w, "%s%s%s\n", prefix, connector, formatIssueLine(child))
		printed[child.ID] = true

		extension := "│   "
		if isLast {
			extension = "    "
		}
		printTree(w, children, child.ID, prefix+extension, printed)
	}
}

// cmdOnboard prints Claude Code integration instructions
func cmdOnboard(w io.Writer) error {
	const instructions = `# beads-lite

This project uses beads-lite for task tracking. You MUST use it to track work.

## Required Workflow

1. Run ` + "`bl ready`" + ` at session start to see available work
2. When you discover new work, create a task: ` + "`bl create \"description\"`" + `
3. When tasks depend on each other: ` + "`bl update <id> --blocked-by <blocker>`" + `
4. When you complete work: ` + "`bl close <id>`" + `

## Commands

` + "```" + `
bl ready              # what can I work on now?
bl ready --json       # machine-readable output
bl list               # all tasks
bl list --tree        # dependency visualization
bl create "title"     # new task
bl close <id>         # complete task (resolution: done)
bl close <id> --resolution wontfix   # close as won't fix
bl close <id> --resolution duplicate # close as duplicate
bl update <a> --blocked-by <b>       # a blocked by b
bl show <id>          # task details
bl show <id> --tree   # show issue with dependency subtree
bl list --status done --resolution wontfix  # filter by resolution
` + "```" + `

## Closing Tasks

When closing tasks, specify WHY with --resolution:
- ` + "`done`" + ` (default): Work completed successfully
- ` + "`wontfix`" + `: Intentionally rejected (document reasoning in description)
- ` + "`duplicate`" + `: Duplicate of another issue

Use ` + "`bl list --status done --resolution wontfix`" + ` to review rejected ideas.

## Multi-Agent Claiming

When multiple agents share a database, use atomic claiming to avoid duplicate work:

` + "```" + `
bl claim <id> --agent <name>   # atomically claim (fails if already claimed)
bl unclaim <id>                # release claim
bl ready --unclaimed           # only show tasks no one has claimed
bl ready --assigned-to <name>  # show tasks assigned to specific agent
bl list --assigned-to <name>   # list tasks assigned to specific agent
` + "```" + `

## Epic Workflow

Epics group related tasks. Use ` + "`--epic`" + ` to link tasks under epics (non-blocking).
Use ` + "`--blocked-by`" + ` only for real work dependencies.

` + "```" + `
# Create epic to track a feature
bl create "User authentication" --type epic

# Create tasks linked to the epic
bl create "Add login endpoint" --epic <epic-id>
bl create "Add session storage" --epic <epic-id>
bl create "Add logout endpoint" --epic <epic-id>

# Link existing tasks to an epic
bl update <task-id> --epic <epic-id>

# If tasks have real work dependencies, add blockers
bl update <logout-id> --blocked-by <login-id>

# View tree (epics show children)
bl list --tree
bl ready --tree

# Close tasks as completed, close epic when feature is done
bl close <epic-id>
` + "```" + `

IMPORTANT: Always link tasks to their parent epic with ` + "`--epic <id>`" + `.
Epics without linked children are invisible in tree views.

## Rules

- Always check ` + "`bl ready`" + ` before starting work
- Create tasks for any new work you discover
- Link tasks to their parent epic with ` + "`--epic <id>`" + `
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
