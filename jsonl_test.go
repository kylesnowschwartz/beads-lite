package beadslite

import (
	"bytes"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

func TestExportToJSONL(t *testing.T) {
	store, cleanup := setupTestStore(t)
	defer cleanup()

	// Create test issues
	now := time.Now()
	issueA := &Issue{
		ID:        "bl-a1b2",
		Title:     "Task A",
		Status:    StatusOpen,
		Priority:  2,
		Type:      IssueTypeTask,
		CreatedAt: now,
		UpdatedAt: now,
	}
	issueB := &Issue{
		ID:        "bl-c3d4",
		Title:     "Task B",
		Status:    StatusOpen,
		Priority:  2,
		Type:      IssueTypeTask,
		CreatedAt: now,
		UpdatedAt: now,
	}

	if err := store.CreateIssue(issueA); err != nil {
		t.Fatalf("CreateIssue(A): %v", err)
	}
	if err := store.CreateIssue(issueB); err != nil {
		t.Fatalf("CreateIssue(B): %v", err)
	}

	// B is blocked by A
	if err := store.AddDependency("bl-c3d4", "bl-a1b2", DepBlocks); err != nil {
		t.Fatalf("AddDependency: %v", err)
	}

	// Export to buffer
	var buf bytes.Buffer
	if err := ExportToJSONL(store, &buf); err != nil {
		t.Fatalf("ExportToJSONL: %v", err)
	}

	output := buf.String()
	lines := strings.Split(strings.TrimSpace(output), "\n")

	// Should have 2 lines (one per issue), sorted by ID
	if len(lines) != 2 {
		t.Fatalf("expected 2 lines, got %d: %s", len(lines), output)
	}

	// First issue should be bl-a1b2 (sorted by ID)
	if !strings.Contains(lines[0], `"id":"bl-a1b2"`) {
		t.Errorf("first line should contain bl-a1b2: %s", lines[0])
	}
	if !strings.Contains(lines[0], `"dependencies":[]`) {
		t.Errorf("first line should have empty dependencies: %s", lines[0])
	}

	// Second issue should be bl-c3d4 with dependency
	if !strings.Contains(lines[1], `"id":"bl-c3d4"`) {
		t.Errorf("second line should contain bl-c3d4: %s", lines[1])
	}
	if !strings.Contains(lines[1], `"depends_on":"bl-a1b2"`) {
		t.Errorf("second line should have dependency on bl-a1b2: %s", lines[1])
	}
	if !strings.Contains(lines[1], `"type":"blocks"`) {
		t.Errorf("second line should have blocks dependency type: %s", lines[1])
	}
}

func TestImportFromJSONL(t *testing.T) {
	store, cleanup := setupTestStore(t)
	defer cleanup()

	// JSONL input with two issues and a dependency
	input := `{"id":"bl-x1y2","title":"Import Task X","status":"open","priority":1,"issue_type":"task","created_at":"2026-01-01T00:00:00Z","updated_at":"2026-01-01T00:00:00Z","dependencies":[]}
{"id":"bl-z3w4","title":"Import Task Z","status":"in_progress","priority":3,"issue_type":"bug","created_at":"2026-01-01T01:00:00Z","updated_at":"2026-01-01T02:00:00Z","dependencies":[{"depends_on":"bl-x1y2","type":"blocks"}]}`

	reader := strings.NewReader(input)
	stats, err := ImportFromJSONL(store, reader)
	if err != nil {
		t.Fatalf("ImportFromJSONL: %v", err)
	}

	if stats.Created != 2 {
		t.Errorf("expected 2 created, got %d", stats.Created)
	}
	if stats.Updated != 0 {
		t.Errorf("expected 0 updated, got %d", stats.Updated)
	}

	// Verify issues were created
	issues, err := store.ListIssues()
	if err != nil {
		t.Fatalf("ListIssues: %v", err)
	}
	if len(issues) != 2 {
		t.Fatalf("expected 2 issues, got %d", len(issues))
	}

	// Verify dependency was created
	deps, err := store.GetDependencies("bl-z3w4")
	if err != nil {
		t.Fatalf("GetDependencies: %v", err)
	}
	if len(deps) != 1 {
		t.Fatalf("expected 1 dependency, got %d", len(deps))
	}
	if deps[0].DependsOnID != "bl-x1y2" {
		t.Errorf("expected dependency on bl-x1y2, got %s", deps[0].DependsOnID)
	}

	// Verify blocking works
	ready, err := store.GetReadyWork()
	if err != nil {
		t.Fatalf("GetReadyWork: %v", err)
	}
	if len(ready) != 1 {
		t.Fatalf("expected 1 ready issue, got %d", len(ready))
	}
	if ready[0].ID != "bl-x1y2" {
		t.Errorf("expected bl-x1y2 to be ready, got %s", ready[0].ID)
	}
}

func TestImportFromJSONL_Upsert(t *testing.T) {
	store, cleanup := setupTestStore(t)
	defer cleanup()

	// Create existing issue
	existing := &Issue{
		ID:        "bl-existing",
		Title:     "Original Title",
		Status:    StatusOpen,
		Priority:  2,
		Type:      IssueTypeTask,
		CreatedAt: time.Now(),
		UpdatedAt: time.Now(),
	}
	if err := store.CreateIssue(existing); err != nil {
		t.Fatalf("CreateIssue: %v", err)
	}

	// Import with updated title
	input := `{"id":"bl-existing","title":"Updated Title","status":"in_progress","priority":1,"issue_type":"feature","created_at":"2026-01-01T00:00:00Z","updated_at":"2026-01-01T00:00:00Z","dependencies":[]}`

	reader := strings.NewReader(input)
	stats, err := ImportFromJSONL(store, reader)
	if err != nil {
		t.Fatalf("ImportFromJSONL: %v", err)
	}

	if stats.Created != 0 {
		t.Errorf("expected 0 created, got %d", stats.Created)
	}
	if stats.Updated != 1 {
		t.Errorf("expected 1 updated, got %d", stats.Updated)
	}

	// Verify issue was updated
	issue, err := store.GetIssue("bl-existing")
	if err != nil {
		t.Fatalf("GetIssue: %v", err)
	}
	if issue.Title != "Updated Title" {
		t.Errorf("expected 'Updated Title', got %q", issue.Title)
	}
	if issue.Status != StatusInProgress {
		t.Errorf("expected 'in_progress', got %q", issue.Status)
	}
}

func TestRoundTrip(t *testing.T) {
	// Create first store with issues
	store1, cleanup1 := setupTestStore(t)
	defer cleanup1()

	now := time.Now()
	issueA := &Issue{
		ID:        "bl-rt01",
		Title:     "Round Trip A",
		Status:    StatusOpen,
		Priority:  1,
		Type:      IssueTypeBug,
		CreatedAt: now,
		UpdatedAt: now,
	}
	issueB := &Issue{
		ID:        "bl-rt02",
		Title:     "Round Trip B",
		Status:    StatusInProgress,
		Priority:  2,
		Type:      IssueTypeFeature,
		CreatedAt: now,
		UpdatedAt: now,
	}
	issueC := &Issue{
		ID:        "bl-rt03",
		Title:     "Round Trip C",
		Status:    StatusOpen,
		Priority:  3,
		Type:      IssueTypeTask,
		CreatedAt: now,
		UpdatedAt: now,
	}

	for _, issue := range []*Issue{issueA, issueB, issueC} {
		if err := store1.CreateIssue(issue); err != nil {
			t.Fatalf("CreateIssue: %v", err)
		}
	}

	// B blocked by A, C blocked by B
	if err := store1.AddDependency("bl-rt02", "bl-rt01", DepBlocks); err != nil {
		t.Fatalf("AddDependency: %v", err)
	}
	if err := store1.AddDependency("bl-rt03", "bl-rt02", DepBlocks); err != nil {
		t.Fatalf("AddDependency: %v", err)
	}

	// Export
	var buf bytes.Buffer
	if err := ExportToJSONL(store1, &buf); err != nil {
		t.Fatalf("ExportToJSONL: %v", err)
	}

	// Import into fresh store
	store2, cleanup2 := setupTestStore(t)
	defer cleanup2()

	reader := strings.NewReader(buf.String())
	_, err := ImportFromJSONL(store2, reader)
	if err != nil {
		t.Fatalf("ImportFromJSONL: %v", err)
	}

	// Verify same issues exist
	issues, err := store2.ListIssues()
	if err != nil {
		t.Fatalf("ListIssues: %v", err)
	}
	if len(issues) != 3 {
		t.Fatalf("expected 3 issues, got %d", len(issues))
	}

	// Verify blocking chain works
	ready, err := store2.GetReadyWork()
	if err != nil {
		t.Fatalf("GetReadyWork: %v", err)
	}
	if len(ready) != 1 {
		t.Fatalf("expected 1 ready issue, got %d", len(ready))
	}
	if ready[0].ID != "bl-rt01" {
		t.Errorf("expected bl-rt01 to be ready, got %s", ready[0].ID)
	}
}

func TestExportToFile(t *testing.T) {
	store, cleanup := setupTestStore(t)
	defer cleanup()

	now := time.Now()
	issue := &Issue{
		ID:        "bl-file",
		Title:     "File Export Test",
		Status:    StatusOpen,
		Priority:  2,
		Type:      IssueTypeTask,
		CreatedAt: now,
		UpdatedAt: now,
	}
	if err := store.CreateIssue(issue); err != nil {
		t.Fatalf("CreateIssue: %v", err)
	}

	// Export to temp file
	tmpDir := t.TempDir()
	filePath := filepath.Join(tmpDir, "export.jsonl")

	if err := ExportToFile(store, filePath); err != nil {
		t.Fatalf("ExportToFile: %v", err)
	}

	// Verify file exists and has content
	data, err := os.ReadFile(filePath)
	if err != nil {
		t.Fatalf("ReadFile: %v", err)
	}
	if !strings.Contains(string(data), "bl-file") {
		t.Errorf("exported file should contain issue ID: %s", string(data))
	}
}

func TestImportFromFile(t *testing.T) {
	store, cleanup := setupTestStore(t)
	defer cleanup()

	// Write JSONL to temp file
	tmpDir := t.TempDir()
	filePath := filepath.Join(tmpDir, "import.jsonl")

	content := `{"id":"bl-fromfile","title":"From File","status":"open","priority":2,"issue_type":"task","created_at":"2026-01-01T00:00:00Z","updated_at":"2026-01-01T00:00:00Z","dependencies":[]}`
	if err := os.WriteFile(filePath, []byte(content), 0644); err != nil {
		t.Fatalf("WriteFile: %v", err)
	}

	stats, err := ImportFromFile(store, filePath)
	if err != nil {
		t.Fatalf("ImportFromFile: %v", err)
	}

	if stats.Created != 1 {
		t.Errorf("expected 1 created, got %d", stats.Created)
	}

	issue, err := store.GetIssue("bl-fromfile")
	if err != nil {
		t.Fatalf("GetIssue: %v", err)
	}
	if issue.Title != "From File" {
		t.Errorf("expected 'From File', got %q", issue.Title)
	}
}

func setupTestStore(t *testing.T) (*Store, func()) {
	store, err := NewStore(":memory:")
	if err != nil {
		t.Fatalf("NewStore: %v", err)
	}
	return store, func() { store.Close() }
}
