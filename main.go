package beadslite

import (
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"sort"
	"strings"

	flag "github.com/spf13/pflag"
)

const (
	beadsDir = ".beads"
	dbName   = "beads.db"
)

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
	case "close":
		return cmdClose(cmdArgs, w)
	case "ready":
		return cmdReady(cmdArgs, w)
	case "dep":
		return cmdDep(cmdArgs, w)
	case "export":
		return cmdExport(cmdArgs, w)
	case "import":
		return cmdImport(cmdArgs, w)
	case "onboard":
		return cmdOnboard(w)
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
  init                  Initialize .beads/ directory and database
  create <title>        Create a new issue, prints ID
  list [--json] [--tree] List all issues
  show <id> [--json]    Show issue details
  update <id> [flags]   Update an issue
  close <id>            Close an issue
  ready [--json]        List unblocked work
  dep add <id> <dep-id> Add dependency (id blocked by dep-id)
  dep rm <id> <dep-id>  Remove dependency
  export [file]         Export all issues to JSONL (stdout or file)
  import <file>         Import issues from JSONL file
  onboard               Print Claude Code integration instructions

List/Ready/Show flags:
  --json                Output as JSONL (one JSON object per line)
  --tree                Show dependency tree (list only)

Update flags:
  --title <string>      New title
  --status <string>     New status (open, in_progress, closed)
  --priority <int>      New priority (0-4)
  --type <string>       New type (task, bug, feature, epic)`)
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

// cmdInit creates the .beads directory and initializes the database
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
	if len(args) == 0 {
		return errors.New("usage: bl create <title>")
	}

	title := strings.Join(args, " ")

	store, err := openStore()
	if err != nil {
		return err
	}
	defer store.Close()

	issue := NewIssue(title)
	if err := store.CreateIssue(issue); err != nil {
		return fmt.Errorf("failed to create issue: %w", err)
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

	if err := fs.Parse(args); err != nil {
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

	if len(issues) == 0 {
		if *jsonFlag {
			// Empty JSONL output is just empty
			return nil
		}
		fmt.Fprintln(w, "No issues found")
		return nil
	}

	if *jsonFlag {
		return outputIssuesJSON(store, issues, w)
	}

	if *treeFlag {
		return outputIssuesTree(store, issues, w)
	}

	for _, issue := range issues {
		fmt.Fprintf(w, "%s  %-11s  P%d  %s  %s\n",
			issue.ID, issue.Status, issue.Priority, issue.Type, issue.Title)
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
		deps, _ := store.GetDependencies(id)
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
		return errors.New("usage: bl update <id> [--title <title>] [--status <status>] [--priority <priority>] [--type <type>]")
	}

	id := args[0]
	flagArgs := args[1:]

	fs := flag.NewFlagSet("update", flag.ContinueOnError)
	fs.SetOutput(w)
	title := fs.String("title", "", "New title")
	status := fs.String("status", "", "New status")
	priority := fs.Int("priority", -1, "New priority (0-4)")
	issueType := fs.String("type", "", "New type")

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

	if err := store.UpdateIssue(issue); err != nil {
		return fmt.Errorf("failed to update: %w", err)
	}

	fmt.Fprintf(w, "Updated %s\n", id)
	return nil
}

// cmdClose closes an issue
func cmdClose(args []string, w io.Writer) error {
	if len(args) == 0 {
		return errors.New("usage: bl close <id>")
	}

	id := args[0]

	store, err := openStore()
	if err != nil {
		return err
	}
	defer store.Close()

	// Verify issue exists first
	if _, err := store.GetIssue(id); err != nil {
		return fmt.Errorf("issue %s: %w", id, err)
	}

	if err := store.CloseIssue(id); err != nil {
		return fmt.Errorf("failed to close: %w", err)
	}

	fmt.Fprintf(w, "Closed %s\n", id)
	return nil
}

// cmdReady lists issues that are ready to work on (not blocked)
func cmdReady(args []string, w io.Writer) error {
	fs := flag.NewFlagSet("ready", flag.ContinueOnError)
	fs.SetOutput(w)
	jsonFlag := fs.Bool("json", false, "Output as JSONL")

	if err := fs.Parse(args); err != nil {
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

	if len(issues) == 0 {
		if *jsonFlag {
			return nil
		}
		fmt.Fprintln(w, "No ready work")
		return nil
	}

	if *jsonFlag {
		return outputIssuesJSON(store, issues, w)
	}

	for _, issue := range issues {
		fmt.Fprintf(w, "%s  %-11s  P%d  %s  %s\n",
			issue.ID, issue.Status, issue.Priority, issue.Type, issue.Title)
	}
	return nil
}

// cmdDep handles dependency subcommands (add, rm)
func cmdDep(args []string, w io.Writer) error {
	if len(args) < 1 {
		return errors.New("usage: bl dep <add|rm> <issue-id> <depends-on-id>")
	}

	subcmd := args[0]
	subArgs := args[1:]

	switch subcmd {
	case "add":
		return cmdDepAdd(subArgs, w)
	case "rm":
		return cmdDepRm(subArgs, w)
	default:
		return fmt.Errorf("unknown dep subcommand: %s", subcmd)
	}
}

func cmdDepAdd(args []string, w io.Writer) error {
	if len(args) < 2 {
		return errors.New("usage: bl dep add <issue-id> <depends-on-id>")
	}

	issueID := args[0]
	dependsOnID := args[1]

	store, err := openStore()
	if err != nil {
		return err
	}
	defer store.Close()

	// Verify both issues exist
	if _, err := store.GetIssue(issueID); err != nil {
		return fmt.Errorf("issue %s: %w", issueID, err)
	}
	if _, err := store.GetIssue(dependsOnID); err != nil {
		return fmt.Errorf("issue %s: %w", dependsOnID, err)
	}

	if err := store.AddDependency(issueID, dependsOnID, DepBlocks); err != nil {
		return fmt.Errorf("failed to add dependency: %w", err)
	}

	fmt.Fprintf(w, "Added dependency: %s blocked by %s\n", issueID, dependsOnID)
	return nil
}

func cmdDepRm(args []string, w io.Writer) error {
	if len(args) < 2 {
		return errors.New("usage: bl dep rm <issue-id> <depends-on-id>")
	}

	issueID := args[0]
	dependsOnID := args[1]

	store, err := openStore()
	if err != nil {
		return err
	}
	defer store.Close()

	if err := store.RemoveDependency(issueID, dependsOnID, DepBlocks); err != nil {
		return fmt.Errorf("failed to remove dependency: %w", err)
	}

	fmt.Fprintf(w, "Removed dependency: %s no longer blocked by %s\n", issueID, dependsOnID)
	return nil
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
	encoder := json.NewEncoder(w)
	for _, issue := range issues {
		deps, _ := store.GetDependencies(issue.ID)
		export := IssueExport{
			ID:           issue.ID,
			Title:        issue.Title,
			Description:  issue.Description,
			Status:       issue.Status,
			Priority:     issue.Priority,
			Type:         issue.Type,
			CreatedAt:    issue.CreatedAt,
			UpdatedAt:    issue.UpdatedAt,
			ClosedAt:     issue.ClosedAt,
			Dependencies: make([]DependencyExport, len(deps)),
		}
		for i, dep := range deps {
			export.Dependencies[i] = DependencyExport{
				DependsOn: dep.DependsOnID,
				Type:      dep.Type,
			}
		}
		if err := encoder.Encode(export); err != nil {
			return err
		}
	}
	return nil
}

// outputSingleIssueJSON outputs a single issue as JSON (not JSONL)
func outputSingleIssueJSON(issue *Issue, deps []*Dependency, w io.Writer) error {
	export := IssueExport{
		ID:           issue.ID,
		Title:        issue.Title,
		Description:  issue.Description,
		Status:       issue.Status,
		Priority:     issue.Priority,
		Type:         issue.Type,
		CreatedAt:    issue.CreatedAt,
		UpdatedAt:    issue.UpdatedAt,
		ClosedAt:     issue.ClosedAt,
		Dependencies: make([]DependencyExport, len(deps)),
	}
	for i, dep := range deps {
		export.Dependencies[i] = DependencyExport{
			DependsOn: dep.DependsOnID,
			Type:      dep.Type,
		}
	}
	encoder := json.NewEncoder(w)
	return encoder.Encode(export)
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
	sort.Slice(roots, func(i, j int) bool {
		if roots[i].Priority != roots[j].Priority {
			return roots[i].Priority < roots[j].Priority
		}
		return roots[i].ID < roots[j].ID
	})

	// Render tree
	for _, root := range roots {
		fmt.Fprintf(w, "%s  %s  P%d  %s  %s\n",
			root.ID, root.Status, root.Priority, root.Type, root.Title)
		printTree(w, children, root.ID, "")
	}

	return nil
}

// printTree recursively prints children with tree-drawing characters
func printTree(w io.Writer, children map[string][]*Issue, parentID string, prefix string) {
	kids := children[parentID]
	// Sort children by priority then ID
	sort.Slice(kids, func(i, j int) bool {
		if kids[i].Priority != kids[j].Priority {
			return kids[i].Priority < kids[j].Priority
		}
		return kids[i].ID < kids[j].ID
	})

	for i, child := range kids {
		isLast := i == len(kids)-1
		connector := "├── "
		if isLast {
			connector = "└── "
		}
		fmt.Fprintf(w, "%s%s%s  %s  P%d  %s  %s\n",
			prefix, connector,
			child.ID, child.Status, child.Priority, child.Type, child.Title)

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
2. When you discover new work, create a task: ` + "`bl create \"description\"`" + `
3. When tasks depend on each other: ` + "`bl dep add <blocked> <blocker>`" + `
4. When you complete work: ` + "`bl close <id>`" + `

## Commands

` + "```" + `
bl ready              # what can I work on now?
bl ready --json       # machine-readable output
bl list               # all tasks
bl list --tree        # dependency visualization
bl create "title"     # new task
bl close <id>         # complete task
bl dep add <a> <b>    # a is blocked by b
bl show <id>          # task details
` + "```" + `

## Rules

- Always check ` + "`bl ready`" + ` before starting work
- Create tasks for any new work you discover
- Close tasks when complete - this unblocks dependent tasks
- Use ` + "`--json`" + ` flag when you need to parse output programmatically
`
	fmt.Fprint(w, instructions)
	return nil
}
