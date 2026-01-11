package beadslite

import (
	"fmt"
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

func TestStoreRemoveDependencyNonExistent(t *testing.T) {
	store := newTestStore(t)
	defer store.Close()

	issueA := NewIssue("Issue A")
	issueB := NewIssue("Issue B")
	store.CreateIssue(issueA)
	store.CreateIssue(issueB)

	// Remove a dependency that was never added
	// This documents current behavior: silent success (DELETE affects 0 rows)
	err := store.RemoveDependency(issueA.ID, issueB.ID, DepBlocks)
	if err != nil {
		t.Errorf("RemoveDependency() on non-existent dep should not error: %v", err)
	}

	// Also test with non-existent issue IDs
	err = store.RemoveDependency("bl-nonexistent", issueB.ID, DepBlocks)
	if err != nil {
		t.Errorf("RemoveDependency() with non-existent issue_id should not error: %v", err)
	}
}

func TestStoreUpdateIssueNonExistent(t *testing.T) {
	store := newTestStore(t)
	defer store.Close()

	// Create an issue object without storing it
	issue := NewIssue("Non-existent Issue")

	// Update should succeed (SQL UPDATE affects 0 rows, which is not an error)
	// This documents current behavior: silent success on non-existent ID
	err := store.UpdateIssue(issue)
	if err != nil {
		t.Errorf("UpdateIssue() on non-existent ID should not error: %v", err)
	}

	// Verify issue was NOT created (update doesn't insert)
	_, err = store.GetIssue(issue.ID)
	if err == nil {
		t.Error("GetIssue() should fail for non-existent issue")
	}
}

func TestStoreRemoveAllDependencies(t *testing.T) {
	store := newTestStore(t)
	defer store.Close()

	// Create issues
	issueA := NewIssue("Issue A")
	issueB := NewIssue("Issue B")
	issueC := NewIssue("Issue C")
	store.CreateIssue(issueA)
	store.CreateIssue(issueB)
	store.CreateIssue(issueC)

	// Add multiple dependencies to A
	store.AddDependency(issueA.ID, issueB.ID, DepBlocks)
	store.AddDependency(issueA.ID, issueC.ID, DepBlocks)

	// Verify A has 2 dependencies
	deps, _ := store.GetDependencies(issueA.ID)
	if len(deps) != 2 {
		t.Fatalf("Before removal: got %d deps, want 2", len(deps))
	}

	// Remove all dependencies for A
	err := store.RemoveAllDependencies(issueA.ID)
	if err != nil {
		t.Fatalf("RemoveAllDependencies() error = %v", err)
	}

	// Verify A has no dependencies
	deps, _ = store.GetDependencies(issueA.ID)
	if len(deps) != 0 {
		t.Errorf("After removal: got %d deps, want 0", len(deps))
	}

	// Verify issue A still exists
	issue, err := store.GetIssue(issueA.ID)
	if err != nil {
		t.Errorf("Issue should still exist after removing deps: %v", err)
	}
	if issue.ID != issueA.ID {
		t.Errorf("Issue ID = %q, want %q", issue.ID, issueA.ID)
	}
}

func TestStoreGetAllDependencies(t *testing.T) {
	store := newTestStore(t)
	defer store.Close()

	// Create issues
	issueA := NewIssue("Issue A")
	issueB := NewIssue("Issue B")
	issueC := NewIssue("Issue C")
	store.CreateIssue(issueA)
	store.CreateIssue(issueB)
	store.CreateIssue(issueC)

	// Add dependencies: A blocked by B, A blocked by C, B blocked by C
	store.AddDependency(issueA.ID, issueB.ID, DepBlocks)
	store.AddDependency(issueA.ID, issueC.ID, DepBlocks)
	store.AddDependency(issueB.ID, issueC.ID, DepBlocks)

	// Get all dependencies
	allDeps, err := store.GetAllDependencies()
	if err != nil {
		t.Fatalf("GetAllDependencies() error = %v", err)
	}

	// Verify map structure
	if len(allDeps) != 2 {
		t.Errorf("GetAllDependencies() returned %d issue keys, want 2 (A and B)", len(allDeps))
	}

	// Verify A has 2 deps
	if len(allDeps[issueA.ID]) != 2 {
		t.Errorf("Issue A has %d deps, want 2", len(allDeps[issueA.ID]))
	}

	// Verify B has 1 dep
	if len(allDeps[issueB.ID]) != 1 {
		t.Errorf("Issue B has %d deps, want 1", len(allDeps[issueB.ID]))
	}

	// Verify C has no deps (should not be in map)
	if _, exists := allDeps[issueC.ID]; exists {
		t.Errorf("Issue C should not be in deps map (it has no deps)")
	}
}

func TestStoreWithTransactionRollback(t *testing.T) {
	store := newTestStore(t)
	defer store.Close()

	issue := NewIssue("Original Issue")
	store.CreateIssue(issue)

	// Execute transaction that creates an issue then fails
	testErr := fmt.Errorf("intentional test failure")
	err := store.WithTransaction(func() error {
		// Create another issue inside transaction
		newIssue := NewIssue("Transaction Issue")
		if err := store.CreateIssue(newIssue); err != nil {
			return err
		}

		// Update existing issue
		issue.Title = "Modified Title"
		if err := store.UpdateIssue(issue); err != nil {
			return err
		}

		// Return error to trigger rollback
		return testErr
	})

	// Should return the test error
	if err != testErr {
		t.Errorf("WithTransaction() error = %v, want %v", err, testErr)
	}

	// Verify only 1 issue exists (transaction issue was rolled back)
	issues, _ := store.ListIssues()
	if len(issues) != 1 {
		t.Errorf("After rollback: got %d issues, want 1", len(issues))
	}

	// Verify original issue title was NOT modified (rolled back)
	got, _ := store.GetIssue(issue.ID)
	if got.Title != "Original Issue" {
		t.Errorf("Title = %q, want %q (should be rolled back)", got.Title, "Original Issue")
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
