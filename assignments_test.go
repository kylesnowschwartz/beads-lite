package beadslite

import (
	"path/filepath"
	"testing"
	"time"
)

// Test that ClaimRole succeeds on first claim and blocks a second live claim.
func TestClaimRole_BlocksConcurrentLiveClaim(t *testing.T) {
	store := newTestStore(t)
	defer store.Close()

	issue := NewIssue("Reviewable Issue")
	if err := store.CreateIssue(issue); err != nil {
		t.Fatalf("CreateIssue: %v", err)
	}

	ok, err := store.ClaimRole(issue.ID, RoleReviewer, "rev-1")
	if err != nil {
		t.Fatalf("first ClaimRole: %v", err)
	}
	if !ok {
		t.Fatal("first claim should succeed")
	}

	ok, err = store.ClaimRole(issue.ID, RoleReviewer, "rev-2")
	if err != nil {
		t.Fatalf("second ClaimRole: %v", err)
	}
	if ok {
		t.Fatal("second claim should fail while role is running")
	}
}

// Test that worker, reviewer, and oracle can each hold the same issue
// concurrently — the whole reason for the role-scoped key.
func TestClaimRole_ParallelRolesPerIssue(t *testing.T) {
	store := newTestStore(t)
	defer store.Close()

	issue := NewIssue("Parallel Roles")
	if err := store.CreateIssue(issue); err != nil {
		t.Fatalf("CreateIssue: %v", err)
	}

	wOK, err := store.ClaimIssue(issue.ID, "worker-1")
	if err != nil || !wOK {
		t.Fatalf("worker claim: ok=%v err=%v", wOK, err)
	}
	rOK, err := store.ClaimRole(issue.ID, RoleReviewer, "reviewer-1")
	if err != nil || !rOK {
		t.Fatalf("reviewer claim: ok=%v err=%v", rOK, err)
	}
	oOK, err := store.ClaimRole(issue.ID, RoleOracle, "oracle-1")
	if err != nil || !oOK {
		t.Fatalf("oracle claim: ok=%v err=%v", oOK, err)
	}

	got, err := store.GetIssue(issue.ID)
	if err != nil {
		t.Fatalf("GetIssue: %v", err)
	}
	if len(got.Assignments) != 3 {
		t.Fatalf("expected 3 assignments, got %d: %+v", len(got.Assignments), got.Assignments)
	}
	wantAgents := map[Role]string{
		RoleWorker:   "worker-1",
		RoleReviewer: "reviewer-1",
		RoleOracle:   "oracle-1",
	}
	for _, a := range got.Assignments {
		if wantAgents[a.Role] != a.Agent {
			t.Errorf("role %s: agent %q want %q", a.Role, a.Agent, wantAgents[a.Role])
		}
		if a.State != AgentStateRunning {
			t.Errorf("role %s: state %q want running", a.Role, a.State)
		}
	}
}

// Test that a finished claim (state=done) can be reclaimed by a different agent.
func TestClaimRole_ReclaimAfterDone(t *testing.T) {
	store := newTestStore(t)
	defer store.Close()

	issue := NewIssue("Reclaimable")
	if err := store.CreateIssue(issue); err != nil {
		t.Fatalf("CreateIssue: %v", err)
	}

	if ok, _ := store.ClaimRole(issue.ID, RoleReviewer, "rev-1"); !ok {
		t.Fatal("first claim should succeed")
	}
	now := time.Now()
	if err := store.SetRoleState(issue.ID, RoleReviewer, AgentStateDone, &now); err != nil {
		t.Fatalf("SetRoleState: %v", err)
	}

	ok, err := store.ClaimRole(issue.ID, RoleReviewer, "rev-2")
	if err != nil {
		t.Fatalf("reclaim: %v", err)
	}
	if !ok {
		t.Fatal("reclaim after done should succeed")
	}
}

// Test that UnclaimRole removes a single role without affecting siblings.
func TestUnclaimRole_LeavesSiblingsAlone(t *testing.T) {
	store := newTestStore(t)
	defer store.Close()

	issue := NewIssue("Unclaim Test")
	if err := store.CreateIssue(issue); err != nil {
		t.Fatalf("CreateIssue: %v", err)
	}
	store.ClaimIssue(issue.ID, "w1")
	store.ClaimRole(issue.ID, RoleReviewer, "r1")
	store.ClaimRole(issue.ID, RoleOracle, "o1")

	if err := store.UnclaimRole(issue.ID, RoleReviewer); err != nil {
		t.Fatalf("UnclaimRole reviewer: %v", err)
	}

	got, _ := store.GetIssue(issue.ID)
	if len(got.Assignments) != 2 {
		t.Fatalf("expected 2 remaining assignments, got %d", len(got.Assignments))
	}
	for _, a := range got.Assignments {
		if a.Role == RoleReviewer {
			t.Error("reviewer assignment should be gone")
		}
	}
}

// Test that worker SetAgentState mirrors the legacy column AND the assignments
// row simultaneously, so neither view diverges.
func TestSetAgentState_MirrorsToAssignments(t *testing.T) {
	store := newTestStore(t)
	defer store.Close()

	issue := NewIssue("Mirror Test")
	if err := store.CreateIssue(issue); err != nil {
		t.Fatalf("CreateIssue: %v", err)
	}
	store.ClaimIssue(issue.ID, "w1")

	now := time.Now()
	if err := store.SetAgentState(issue.ID, AgentStateStuck, &now); err != nil {
		t.Fatalf("SetAgentState: %v", err)
	}

	got, _ := store.GetIssue(issue.ID)
	if got.AgentState != AgentStateStuck {
		t.Errorf("legacy column: %q want stuck", got.AgentState)
	}
	var workerSeen bool
	for _, a := range got.Assignments {
		if a.Role == RoleWorker {
			workerSeen = true
			if a.State != AgentStateStuck {
				t.Errorf("assignments row: %q want stuck", a.State)
			}
		}
	}
	if !workerSeen {
		t.Error("worker assignment row missing")
	}
}

// Test that opening a database with legacy assigned_to/agent_state values
// backfills the assignments table on first access. This is the migration
// path users hit after `ralph-ban update` swaps the bl binary.
func TestMigration_BackfillsLegacyClaims(t *testing.T) {
	dir := t.TempDir()
	dbPath := filepath.Join(dir, "legacy.db")

	// Stage 1: open a store, plant a legacy-style claim by writing directly
	// to the issues table without the assignments table being touched.
	store, err := NewStore(dbPath)
	if err != nil {
		t.Fatalf("NewStore: %v", err)
	}
	issue := NewIssue("Legacy Claim")
	if err := store.CreateIssue(issue); err != nil {
		t.Fatalf("CreateIssue: %v", err)
	}
	now := time.Now()
	if _, err := store.db.Exec(`
		UPDATE issues SET assigned_to = ?, agent_state = ?, last_activity = ?
		WHERE id = ?`, "legacy-worker", string(AgentStateRunning), now, issue.ID); err != nil {
		t.Fatalf("plant legacy claim: %v", err)
	}
	// Erase the assignments table so we simulate a pre-migration database.
	if _, err := store.db.Exec(`DELETE FROM assignments`); err != nil {
		t.Fatalf("clear assignments: %v", err)
	}
	store.Close()

	// Stage 2: reopen the database. NewStore runs migrateSchema which should
	// backfill the assignments table from the legacy columns.
	store2, err := NewStore(dbPath)
	if err != nil {
		t.Fatalf("reopen NewStore: %v", err)
	}
	defer store2.Close()

	got, err := store2.GetIssue(issue.ID)
	if err != nil {
		t.Fatalf("GetIssue after migration: %v", err)
	}
	if len(got.Assignments) != 1 {
		t.Fatalf("expected 1 backfilled assignment, got %d: %+v", len(got.Assignments), got.Assignments)
	}
	a := got.Assignments[0]
	if a.Role != RoleWorker {
		t.Errorf("role %q want worker", a.Role)
	}
	if a.Agent != "legacy-worker" {
		t.Errorf("agent %q want legacy-worker", a.Agent)
	}
	if a.State != AgentStateRunning {
		t.Errorf("state %q want running", a.State)
	}
}

// Test that the migration backfill is idempotent — a second invocation
// over an already-migrated database is a no-op (no PK violation, no duplicate).
func TestMigration_IdempotentReruns(t *testing.T) {
	dir := t.TempDir()
	dbPath := filepath.Join(dir, "idempotent.db")

	store, err := NewStore(dbPath)
	if err != nil {
		t.Fatalf("NewStore: %v", err)
	}
	issue := NewIssue("Stable Claim")
	if err := store.CreateIssue(issue); err != nil {
		t.Fatalf("CreateIssue: %v", err)
	}
	store.ClaimIssue(issue.ID, "w1")
	store.Close()

	// Reopen — migrateSchema runs again. Should be a no-op.
	store2, err := NewStore(dbPath)
	if err != nil {
		t.Fatalf("reopen: %v", err)
	}
	defer store2.Close()

	got, _ := store2.GetIssue(issue.ID)
	if len(got.Assignments) != 1 {
		t.Fatalf("expected 1 assignment after reopen, got %d", len(got.Assignments))
	}
	if got.Assignments[0].Agent != "w1" {
		t.Errorf("agent %q want w1", got.Assignments[0].Agent)
	}
}
