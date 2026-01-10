package beadslite

import (
	"errors"
	"flag"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"
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
		return cmdList(w)
	case "show":
		return cmdShow(cmdArgs, w)
	case "update":
		return cmdUpdate(cmdArgs, w)
	case "close":
		return cmdClose(cmdArgs, w)
	case "ready":
		return cmdReady(w)
	case "dep":
		return cmdDep(cmdArgs, w)
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
  list                  List all issues
  show <id>             Show issue details
  update <id> [flags]   Update an issue
  close <id>            Close an issue
  ready                 List unblocked work
  dep add <id> <dep-id> Add dependency (id blocked by dep-id)
  dep rm <id> <dep-id>  Remove dependency

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
func cmdList(w io.Writer) error {
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
		fmt.Fprintln(w, "No issues found")
		return nil
	}

	for _, issue := range issues {
		fmt.Fprintf(w, "%s  %-11s  P%d  %s  %s\n",
			issue.ID, issue.Status, issue.Priority, issue.Type, issue.Title)
	}
	return nil
}

// cmdShow displays details for a single issue
func cmdShow(args []string, w io.Writer) error {
	if len(args) == 0 {
		return errors.New("usage: bl show <id>")
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
func cmdReady(w io.Writer) error {
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
		fmt.Fprintln(w, "No ready work")
		return nil
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
