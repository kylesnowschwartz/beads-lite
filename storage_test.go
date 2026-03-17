package beadslite

import (
	"fmt"
	"os"
	"path/filepath"
	"testing"
	"time"
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
	issue.Status = StatusDoing
	err := store.UpdateIssue(issue)
	if err != nil {
		t.Fatalf("UpdateIssue() error = %v", err)
	}

	// Verify
	got, _ := store.GetIssue(issue.ID)
	if got.Title != "Updated Title" {
		t.Errorf("Title = %q, want %q", got.Title, "Updated Title")
	}
	if got.Status != StatusDoing {
		t.Errorf("Status = %q, want %q", got.Status, StatusDoing)
	}
}

func TestStoreCloseIssue(t *testing.T) {
	store := newTestStore(t)
	defer store.Close()

	issue := NewIssue("Task to close")
	store.CreateIssue(issue)

	_, err := store.CloseIssue(issue.ID, ResolutionDone)
	if err != nil {
		t.Fatalf("CloseIssue() error = %v", err)
	}

	got, _ := store.GetIssue(issue.ID)
	if got.Status != StatusDone {
		t.Errorf("Status = %q, want %q", got.Status, StatusDone)
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

	// Create chain: A blocks B blocks C (all set to todo so they appear in ready work)
	issueA := NewIssue("Task A")
	issueA.Status = StatusTodo
	issueB := NewIssue("Task B")
	issueB.Status = StatusTodo
	issueC := NewIssue("Task C")
	issueC.Status = StatusTodo
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

func TestStoreClaimIssue(t *testing.T) {
	store := newTestStore(t)
	defer store.Close()

	issue := NewIssue("Claimable Task")
	store.CreateIssue(issue)

	// First claim should succeed
	claimed, err := store.ClaimIssue(issue.ID, "agent-1")
	if err != nil {
		t.Fatalf("ClaimIssue() error = %v", err)
	}
	if !claimed {
		t.Error("first claim should succeed")
	}

	// Verify the issue is now assigned and in_progress
	got, _ := store.GetIssue(issue.ID)
	if got.AssignedTo != "agent-1" {
		t.Errorf("AssignedTo = %q, want %q", got.AssignedTo, "agent-1")
	}
	if got.Status != StatusDoing {
		t.Errorf("Status = %q, want %q", got.Status, StatusDoing)
	}
}

func TestStoreClaimIssueAlreadyClaimed(t *testing.T) {
	store := newTestStore(t)
	defer store.Close()

	issue := NewIssue("Contested Task")
	store.CreateIssue(issue)

	// First agent claims
	claimed1, err := store.ClaimIssue(issue.ID, "agent-1")
	if err != nil {
		t.Fatalf("first ClaimIssue() error = %v", err)
	}
	if !claimed1 {
		t.Fatal("first claim should succeed")
	}

	// Second agent tries to claim the same task
	claimed2, err := store.ClaimIssue(issue.ID, "agent-2")
	if err != nil {
		t.Fatalf("second ClaimIssue() error = %v", err)
	}
	if claimed2 {
		t.Error("second claim should fail (already claimed)")
	}

	// Verify still assigned to agent-1
	got, _ := store.GetIssue(issue.ID)
	if got.AssignedTo != "agent-1" {
		t.Errorf("AssignedTo = %q, want %q (should still be agent-1)", got.AssignedTo, "agent-1")
	}
}

func TestStoreClaimIssueIdempotent(t *testing.T) {
	store := newTestStore(t)
	defer store.Close()

	issue := NewIssue("Task")
	store.CreateIssue(issue)

	// Same agent claiming twice should fail on second attempt
	// (the WHERE clause requires assigned_to IS NULL)
	store.ClaimIssue(issue.ID, "agent-1")
	claimed, _ := store.ClaimIssue(issue.ID, "agent-1")
	if claimed {
		t.Error("re-claim by same agent should return false (already assigned)")
	}
}

func TestStoreUnclaimIssue(t *testing.T) {
	store := newTestStore(t)
	defer store.Close()

	issue := NewIssue("Unclaim Me")
	store.CreateIssue(issue)

	// Claim then unclaim
	store.ClaimIssue(issue.ID, "agent-1")
	err := store.UnclaimIssue(issue.ID)
	if err != nil {
		t.Fatalf("UnclaimIssue() error = %v", err)
	}

	// Verify reset
	got, _ := store.GetIssue(issue.ID)
	if got.AssignedTo != "" {
		t.Errorf("AssignedTo = %q, want empty", got.AssignedTo)
	}
	if got.Status != StatusTodo {
		t.Errorf("Status = %q, want %q", got.Status, StatusTodo)
	}

	// Another agent should now be able to claim
	claimed, _ := store.ClaimIssue(issue.ID, "agent-2")
	if !claimed {
		t.Error("claim after unclaim should succeed")
	}
}

func TestStoreCloseIssueClearsAssignment(t *testing.T) {
	store := newTestStore(t)
	defer store.Close()

	issue := NewIssue("Assigned then closed")
	store.CreateIssue(issue)

	store.ClaimIssue(issue.ID, "agent-1")
	store.CloseIssue(issue.ID, ResolutionDone)

	got, _ := store.GetIssue(issue.ID)
	if got.AssignedTo != "" {
		t.Errorf("closing should clear AssignedTo, got %q", got.AssignedTo)
	}
}

func TestStoreConcurrentClaim(t *testing.T) {
	// Use a file-based database for real multi-goroutine contention.
	// In-memory DBs don't exercise WAL + busy_timeout.
	dir := t.TempDir()
	dbPath := filepath.Join(dir, "concurrent.db")

	store, err := NewStore(dbPath)
	if err != nil {
		t.Fatalf("NewStore() error = %v", err)
	}
	defer store.Close()

	issue := NewIssue("Race Condition Task")
	store.CreateIssue(issue)

	const numAgents = 10
	results := make(chan bool, numAgents)
	errs := make(chan error, numAgents)

	// Race: N goroutines all try to claim the same task
	for i := 0; i < numAgents; i++ {
		go func(agentNum int) {
			// Each goroutine opens its own connection (like separate processes would)
			s, err := NewStore(dbPath)
			if err != nil {
				errs <- err
				results <- false
				return
			}
			defer s.Close()

			claimed, err := s.ClaimIssue(issue.ID, fmt.Sprintf("agent-%d", agentNum))
			if err != nil {
				errs <- err
				results <- false
				return
			}
			errs <- nil
			results <- claimed
		}(i)
	}

	// Collect results
	winCount := 0
	for i := 0; i < numAgents; i++ {
		if err := <-errs; err != nil {
			t.Errorf("agent error: %v", err)
		}
		if <-results {
			winCount++
		}
	}

	// Exactly one agent should have won the race
	if winCount != 1 {
		t.Errorf("expected exactly 1 winner, got %d", winCount)
	}
}

func TestStoreSetAgentState(t *testing.T) {
	store := newTestStore(t)
	defer store.Close()

	issue := NewIssue("Agent task")
	if err := store.CreateIssue(issue); err != nil {
		t.Fatalf("CreateIssue() error = %v", err)
	}

	now := time.Now()
	if err := store.SetAgentState(issue.ID, AgentStateRunning, &now); err != nil {
		t.Fatalf("SetAgentState() error = %v", err)
	}

	got, err := store.GetIssue(issue.ID)
	if err != nil {
		t.Fatalf("GetIssue() error = %v", err)
	}
	if got.AgentState != AgentStateRunning {
		t.Errorf("AgentState = %q, want %q", got.AgentState, AgentStateRunning)
	}
	if got.LastActivity == nil {
		t.Error("LastActivity should be set")
	}
}

func TestStoreSetAgentStateInvalid(t *testing.T) {
	store := newTestStore(t)
	defer store.Close()

	issue := NewIssue("Agent task")
	store.CreateIssue(issue)

	err := store.SetAgentState(issue.ID, AgentState("invalid"), nil)
	if err == nil {
		t.Error("SetAgentState() should return error for invalid state")
	}
}

func TestStoreGetAgentsByState(t *testing.T) {
	store := newTestStore(t)
	defer store.Close()

	issueA := NewIssue("Running task A")
	issueB := NewIssue("Running task B")
	issueC := NewIssue("Idle task")
	store.CreateIssue(issueA)
	store.CreateIssue(issueB)
	store.CreateIssue(issueC)

	now := time.Now()
	store.SetAgentState(issueA.ID, AgentStateRunning, &now)
	store.SetAgentState(issueB.ID, AgentStateRunning, &now)
	store.SetAgentState(issueC.ID, AgentStateIdle, &now)

	running, err := store.GetAgentsByState(AgentStateRunning)
	if err != nil {
		t.Fatalf("GetAgentsByState() error = %v", err)
	}
	if len(running) != 2 {
		t.Errorf("GetAgentsByState(running) returned %d issues, want 2", len(running))
	}

	idle, err := store.GetAgentsByState(AgentStateIdle)
	if err != nil {
		t.Fatalf("GetAgentsByState() error = %v", err)
	}
	if len(idle) != 1 {
		t.Errorf("GetAgentsByState(idle) returned %d issues, want 1", len(idle))
	}

	stuck, err := store.GetAgentsByState(AgentStateStuck)
	if err != nil {
		t.Fatalf("GetAgentsByState() error = %v", err)
	}
	if len(stuck) != 0 {
		t.Errorf("GetAgentsByState(stuck) returned %d issues, want 0", len(stuck))
	}
}

func TestStoreAgentStateRoundTrip(t *testing.T) {
	store := newTestStore(t)
	defer store.Close()

	issue := NewIssue("Lifecycle task")
	store.CreateIssue(issue)

	// Verify default state is empty
	got, _ := store.GetIssue(issue.ID)
	if got.AgentState != "" {
		t.Errorf("default AgentState = %q, want empty", got.AgentState)
	}
	if got.LastActivity != nil {
		t.Error("default LastActivity should be nil")
	}

	// Transition through states
	now := time.Now()
	for _, state := range []AgentState{AgentStateRunning, AgentStateStuck, AgentStateDone} {
		if err := store.SetAgentState(issue.ID, state, &now); err != nil {
			t.Fatalf("SetAgentState(%q) error = %v", state, err)
		}
		got, _ = store.GetIssue(issue.ID)
		if got.AgentState != state {
			t.Errorf("AgentState = %q, want %q", got.AgentState, state)
		}
	}

	// Clear state by setting to idle with nil activity
	if err := store.SetAgentState(issue.ID, AgentStateIdle, nil); err != nil {
		t.Fatalf("SetAgentState(idle, nil) error = %v", err)
	}
	got, _ = store.GetIssue(issue.ID)
	if got.AgentState != AgentStateIdle {
		t.Errorf("AgentState = %q, want %q", got.AgentState, AgentStateIdle)
	}
	if got.LastActivity != nil {
		t.Error("LastActivity should be nil when set to nil")
	}
}

func TestStoreAgentStatePreservedOnUpdate(t *testing.T) {
	store := newTestStore(t)
	defer store.Close()

	issue := NewIssue("Task with agent state")
	store.CreateIssue(issue)

	now := time.Now()
	store.SetAgentState(issue.ID, AgentStateRunning, &now)

	// Fetch, modify title, update — agent_state should round-trip
	got, _ := store.GetIssue(issue.ID)
	got.Title = "Updated title"
	if err := store.UpdateIssue(got); err != nil {
		t.Fatalf("UpdateIssue() error = %v", err)
	}

	after, _ := store.GetIssue(issue.ID)
	if after.AgentState != AgentStateRunning {
		t.Errorf("AgentState = %q after UpdateIssue, want %q", after.AgentState, AgentStateRunning)
	}
	if after.LastActivity == nil {
		t.Error("LastActivity should still be set after UpdateIssue")
	}
}

func TestStoreAgentStateBackwardCompatibility(t *testing.T) {
	// Open a new store (simulates existing DB getting new columns via migration).
	// In-memory DBs are always fresh, so we use a file-based DB to test migration.
	dir := t.TempDir()
	dbPath := filepath.Join(dir, "compat.db")

	store1, err := NewStore(dbPath)
	if err != nil {
		t.Fatalf("NewStore() error = %v", err)
	}

	issue := NewIssue("Pre-migration task")
	store1.CreateIssue(issue)
	store1.Close()

	// Reopen: migrateSchema should add the columns without error
	store2, err := NewStore(dbPath)
	if err != nil {
		t.Fatalf("NewStore() reopen error = %v", err)
	}
	defer store2.Close()

	// Old issue should still be readable with zero agent state
	got, err := store2.GetIssue(issue.ID)
	if err != nil {
		t.Fatalf("GetIssue() after migration error = %v", err)
	}
	if got.ID != issue.ID {
		t.Errorf("ID = %q, want %q", got.ID, issue.ID)
	}
	if got.AgentState != "" {
		t.Errorf("AgentState = %q, want empty (pre-migration row)", got.AgentState)
	}

	// SetAgentState should now work on the migrated DB
	now := time.Now()
	if err := store2.SetAgentState(issue.ID, AgentStateDead, &now); err != nil {
		t.Fatalf("SetAgentState() on migrated DB error = %v", err)
	}
	got, _ = store2.GetIssue(issue.ID)
	if got.AgentState != AgentStateDead {
		t.Errorf("AgentState = %q, want %q", got.AgentState, AgentStateDead)
	}
}

func TestCloseIssue_AutoClosesParentEpic(t *testing.T) {
	store := newTestStore(t)
	defer store.Close()

	// Create epic with two children linked via DepParent
	epic := NewIssue("Auth feature")
	epic.Type = IssueTypeEpic
	store.CreateIssue(epic)

	child1 := NewIssue("Add login endpoint")
	child1.Status = StatusTodo
	store.CreateIssue(child1)
	store.AddDependency(child1.ID, epic.ID, DepParent)

	child2 := NewIssue("Add logout endpoint")
	child2.Status = StatusTodo
	store.CreateIssue(child2)
	store.AddDependency(child2.ID, epic.ID, DepParent)

	// Close first child — epic should stay open
	store.CloseIssue(child1.ID, ResolutionDone)

	// Close last child — epic should auto-close
	result, err := store.CloseIssue(child2.ID, ResolutionDone)
	if err != nil {
		t.Fatalf("CloseIssue() error = %v", err)
	}

	// Verify auto-close result was returned
	if result == nil {
		t.Fatal("expected AutoCloseResult, got nil")
	}
	if result.ID != epic.ID {
		t.Errorf("AutoCloseResult.ID = %q, want %q", result.ID, epic.ID)
	}
	if result.Title != epic.Title {
		t.Errorf("AutoCloseResult.Title = %q, want %q", result.Title, epic.Title)
	}

	// Verify epic is done
	got, _ := store.GetIssue(epic.ID)
	if got.Status != StatusDone {
		t.Errorf("epic status = %q, want %q", got.Status, StatusDone)
	}
	if got.Resolution != ResolutionDone {
		t.Errorf("epic resolution = %q, want %q", got.Resolution, ResolutionDone)
	}
}

func TestCloseIssue_NoAutoCloseWithOpenChildren(t *testing.T) {
	store := newTestStore(t)
	defer store.Close()

	epic := NewIssue("Multi-child epic")
	epic.Type = IssueTypeEpic
	store.CreateIssue(epic)

	child1 := NewIssue("Done child")
	child1.Status = StatusTodo
	store.CreateIssue(child1)
	store.AddDependency(child1.ID, epic.ID, DepParent)

	child2 := NewIssue("Still open child")
	child2.Status = StatusTodo
	store.CreateIssue(child2)
	store.AddDependency(child2.ID, epic.ID, DepParent)

	// Close one child while sibling remains open
	result, err := store.CloseIssue(child1.ID, ResolutionDone)
	if err != nil {
		t.Fatalf("CloseIssue() error = %v", err)
	}
	if result != nil {
		t.Errorf("expected no auto-close, got %+v", result)
	}

	// Epic should still be open
	got, _ := store.GetIssue(epic.ID)
	if got.Status == StatusDone {
		t.Error("epic should not be closed while children remain open")
	}
}

func TestCloseIssue_NoCascadePastParent(t *testing.T) {
	store := newTestStore(t)
	defer store.Close()

	// Grandparent epic A -> child epic B -> grandchild task C
	epicA := NewIssue("Grandparent epic")
	epicA.Type = IssueTypeEpic
	store.CreateIssue(epicA)

	epicB := NewIssue("Child epic")
	epicB.Type = IssueTypeEpic
	epicB.Status = StatusTodo
	store.CreateIssue(epicB)
	store.AddDependency(epicB.ID, epicA.ID, DepParent)

	taskC := NewIssue("Grandchild task")
	taskC.Status = StatusTodo
	store.CreateIssue(taskC)
	store.AddDependency(taskC.ID, epicB.ID, DepParent)

	// Close the grandchild — should auto-close epicB but NOT epicA
	result, err := store.CloseIssue(taskC.ID, ResolutionDone)
	if err != nil {
		t.Fatalf("CloseIssue() error = %v", err)
	}

	// epicB should have been auto-closed
	if result == nil {
		t.Fatal("expected epicB to auto-close")
	}
	if result.ID != epicB.ID {
		t.Errorf("auto-closed ID = %q, want %q", result.ID, epicB.ID)
	}

	gotB, _ := store.GetIssue(epicB.ID)
	if gotB.Status != StatusDone {
		t.Errorf("epicB status = %q, want %q", gotB.Status, StatusDone)
	}

	// epicA should still be open (no cascade)
	gotA, _ := store.GetIssue(epicA.ID)
	if gotA.Status == StatusDone {
		t.Error("epicA should NOT be auto-closed (no cascade past parent)")
	}
}

func TestCloseIssue_WontfixTriggersParentCheck(t *testing.T) {
	store := newTestStore(t)
	defer store.Close()

	epic := NewIssue("Feature epic")
	epic.Type = IssueTypeEpic
	store.CreateIssue(epic)

	child1 := NewIssue("Completed child")
	child1.Status = StatusTodo
	store.CreateIssue(child1)
	store.AddDependency(child1.ID, epic.ID, DepParent)

	child2 := NewIssue("Rejected child")
	child2.Status = StatusTodo
	store.CreateIssue(child2)
	store.AddDependency(child2.ID, epic.ID, DepParent)

	// Close first child normally
	store.CloseIssue(child1.ID, ResolutionDone)

	// Close second child as wontfix — parent should still auto-close with resolution "done"
	result, err := store.CloseIssue(child2.ID, ResolutionWontfix)
	if err != nil {
		t.Fatalf("CloseIssue() error = %v", err)
	}
	if result == nil {
		t.Fatal("expected auto-close after wontfix")
	}

	got, _ := store.GetIssue(epic.ID)
	if got.Status != StatusDone {
		t.Errorf("epic status = %q, want %q", got.Status, StatusDone)
	}
	if got.Resolution != ResolutionDone {
		t.Errorf("epic resolution = %q, want %q (parent always closes as done)", got.Resolution, ResolutionDone)
	}
}

func TestCloseIssue_NoParentDependency(t *testing.T) {
	store := newTestStore(t)
	defer store.Close()

	task := NewIssue("Standalone task")
	task.Status = StatusTodo
	store.CreateIssue(task)

	// Close a task with no parent link
	result, err := store.CloseIssue(task.ID, ResolutionDone)
	if err != nil {
		t.Fatalf("CloseIssue() error = %v", err)
	}
	if result != nil {
		t.Errorf("expected no auto-close for parentless task, got %+v", result)
	}

	// Task should be closed
	got, _ := store.GetIssue(task.ID)
	if got.Status != StatusDone {
		t.Errorf("status = %q, want %q", got.Status, StatusDone)
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
