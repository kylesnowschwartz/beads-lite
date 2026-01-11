package beadslite

import (
	"os"
	"path/filepath"
	"testing"
)

func TestNewStore(t *testing.T) {
	dir := t.TempDir()
	dbPath := filepath.Join(dir, "test.db")

	store, err := NewStore(dbPath)
	if err != nil {
		t.Fatalf("NewStore() error = %v", err)
	}
	defer store.Close()

	// Check database file was created
	if _, err := os.Stat(dbPath); os.IsNotExist(err) {
		t.Error("Database file was not created")
	}
}

func TestStoreCreateAndGetIssue(t *testing.T) {
	store := newTestStore(t)
	defer store.Close()

	issue := NewIssue("Test Issue")
	issue.Description = "Test description"
	issue.Priority = 1

	// Create
	err := store.CreateIssue(issue)
	if err != nil {
		t.Fatalf("CreateIssue() error = %v", err)
	}

	// Get
	got, err := store.GetIssue(issue.ID)
	if err != nil {
		t.Fatalf("GetIssue() error = %v", err)
	}

	if got.ID != issue.ID {
		t.Errorf("ID = %q, want %q", got.ID, issue.ID)
	}
	if got.Title != issue.Title {
		t.Errorf("Title = %q, want %q", got.Title, issue.Title)
	}
	if got.Description != issue.Description {
		t.Errorf("Description = %q, want %q", got.Description, issue.Description)
	}
	if got.Priority != issue.Priority {
		t.Errorf("Priority = %d, want %d", got.Priority, issue.Priority)
	}
}

func TestStoreGetIssueNotFound(t *testing.T) {
	store := newTestStore(t)
	defer store.Close()

	_, err := store.GetIssue("bl-nonexistent")
	if err == nil {
		t.Error("GetIssue() expected error for non-existent issue")
	}
}

func TestStoreUpdateIssue(t *testing.T) {
	store := newTestStore(t)
	defer store.Close()

	issue := NewIssue("Original Title")
	store.CreateIssue(issue)

	// Update
	issue.Title = "Updated Title"
	issue.Status = StatusInProgress
	err := store.UpdateIssue(issue)
	if err != nil {
		t.Fatalf("UpdateIssue() error = %v", err)
	}

	// Verify
	got, _ := store.GetIssue(issue.ID)
	if got.Title != "Updated Title" {
		t.Errorf("Title = %q, want %q", got.Title, "Updated Title")
	}
	if got.Status != StatusInProgress {
		t.Errorf("Status = %q, want %q", got.Status, StatusInProgress)
	}
}

func TestStoreCloseIssue(t *testing.T) {
	store := newTestStore(t)
	defer store.Close()

	issue := NewIssue("Task to close")
	store.CreateIssue(issue)

	err := store.CloseIssue(issue.ID, ResolutionDone)
	if err != nil {
		t.Fatalf("CloseIssue() error = %v", err)
	}

	got, _ := store.GetIssue(issue.ID)
	if got.Status != StatusClosed {
		t.Errorf("Status = %q, want %q", got.Status, StatusClosed)
	}
	if got.ClosedAt == nil {
		t.Error("ClosedAt should be set")
	}
}

func TestStoreListIssues(t *testing.T) {
	store := newTestStore(t)
	defer store.Close()

	// Create multiple issues
	store.CreateIssue(NewIssue("Issue 1"))
	store.CreateIssue(NewIssue("Issue 2"))
	store.CreateIssue(NewIssue("Issue 3"))

	issues, err := store.ListIssues()
	if err != nil {
		t.Fatalf("ListIssues() error = %v", err)
	}

	if len(issues) != 3 {
		t.Errorf("ListIssues() returned %d issues, want 3", len(issues))
	}
}

func TestStoreAddAndRemoveDependency(t *testing.T) {
	store := newTestStore(t)
	defer store.Close()

	issueA := NewIssue("Issue A")
	issueB := NewIssue("Issue B")
	store.CreateIssue(issueA)
	store.CreateIssue(issueB)

	// Add dependency: B blocked by A
	err := store.AddDependency(issueB.ID, issueA.ID, DepBlocks)
	if err != nil {
		t.Fatalf("AddDependency() error = %v", err)
	}

	// Verify
	deps, err := store.GetDependencies(issueB.ID)
	if err != nil {
		t.Fatalf("GetDependencies() error = %v", err)
	}
	if len(deps) != 1 {
		t.Fatalf("GetDependencies() returned %d deps, want 1", len(deps))
	}
	if deps[0].DependsOnID != issueA.ID {
		t.Errorf("DependsOnID = %q, want %q", deps[0].DependsOnID, issueA.ID)
	}

	// Remove
	err = store.RemoveDependency(issueB.ID, issueA.ID, DepBlocks)
	if err != nil {
		t.Fatalf("RemoveDependency() error = %v", err)
	}

	deps, _ = store.GetDependencies(issueB.ID)
	if len(deps) != 0 {
		t.Errorf("After removal, got %d deps, want 0", len(deps))
	}
}

func TestStoreGetReadyWork(t *testing.T) {
	store := newTestStore(t)
	defer store.Close()

	// Create chain: A blocks B blocks C
	issueA := NewIssue("Task A")
	issueB := NewIssue("Task B")
	issueC := NewIssue("Task C")
	store.CreateIssue(issueA)
	store.CreateIssue(issueB)
	store.CreateIssue(issueC)

	store.AddDependency(issueB.ID, issueA.ID, DepBlocks) // B blocked by A
	store.AddDependency(issueC.ID, issueB.ID, DepBlocks) // C blocked by B

	// Only A should be ready
	ready, err := store.GetReadyWork()
	if err != nil {
		t.Fatalf("GetReadyWork() error = %v", err)
	}
	if len(ready) != 1 {
		t.Fatalf("GetReadyWork() returned %d issues, want 1", len(ready))
	}
	if ready[0].ID != issueA.ID {
		t.Errorf("Ready issue = %q, want %q", ready[0].ID, issueA.ID)
	}

	// Close A, now B should be ready
	store.CloseIssue(issueA.ID, ResolutionDone)
	ready, _ = store.GetReadyWork()
	if len(ready) != 1 {
		t.Fatalf("After closing A, got %d ready, want 1", len(ready))
	}
	if ready[0].ID != issueB.ID {
		t.Errorf("Ready issue = %q, want %q", ready[0].ID, issueB.ID)
	}

	// Close B, now C should be ready
	store.CloseIssue(issueB.ID, ResolutionDone)
	ready, _ = store.GetReadyWork()
	if len(ready) != 1 {
		t.Fatalf("After closing B, got %d ready, want 1", len(ready))
	}
	if ready[0].ID != issueC.ID {
		t.Errorf("Ready issue = %q, want %q", ready[0].ID, issueC.ID)
	}
}

func TestStoreGetReadyWorkWithParentChild(t *testing.T) {
	store := newTestStore(t)
	defer store.Close()

	// Create epic with subtasks
	epic := NewIssue("Epic")
	epic.Type = IssueTypeEpic
	task1 := NewIssue("Subtask 1")
	task2 := NewIssue("Subtask 2")
	blocker := NewIssue("Blocker")

	store.CreateIssue(epic)
	store.CreateIssue(task1)
	store.CreateIssue(task2)
	store.CreateIssue(blocker)

	// Set up parent-child relationships
	store.AddDependency(task1.ID, epic.ID, DepParentChild)
	store.AddDependency(task2.ID, epic.ID, DepParentChild)

	// Block the epic
	store.AddDependency(epic.ID, blocker.ID, DepBlocks)

	// Only blocker should be ready (epic blocked, children transitively blocked)
	ready, err := store.GetReadyWork()
	if err != nil {
		t.Fatalf("GetReadyWork() error = %v", err)
	}
	if len(ready) != 1 {
		t.Fatalf("GetReadyWork() returned %d issues, want 1", len(ready))
	}
	if ready[0].ID != blocker.ID {
		t.Errorf("Ready issue = %q, want %q (blocker)", ready[0].ID, blocker.ID)
	}

	// Close blocker, now epic and both subtasks should be ready
	store.CloseIssue(blocker.ID, ResolutionDone)
	ready, _ = store.GetReadyWork()
	if len(ready) != 3 {
		t.Errorf("After closing blocker, got %d ready, want 3", len(ready))
	}
}

// Helper to create a test store with in-memory database
func newTestStore(t *testing.T) *Store {
	t.Helper()
	store, err := NewStore(":memory:")
	if err != nil {
		t.Fatalf("NewStore() error = %v", err)
	}
	return store
}
