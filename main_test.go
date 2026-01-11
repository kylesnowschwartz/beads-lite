package beadslite

import (
	"bytes"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// CLI tests execute the CLI via runCLI helper and check output/exit codes.

func TestCLI_Init(t *testing.T) {
	dir := t.TempDir()
	oldDir, _ := os.Getwd()
	os.Chdir(dir)
	defer os.Chdir(oldDir)

	out, err := runCLI([]string{"init"})
	if err != nil {
		t.Fatalf("init failed: %v", err)
	}

	// Check .beads-lite directory created
	if _, statErr := os.Stat(".beads-lite"); os.IsNotExist(statErr) {
		t.Error(".beads-lite directory not created")
	}

	// Check database file created
	if _, statErr := os.Stat(".beads-lite/beads.db"); os.IsNotExist(statErr) {
		t.Error(".beads-lite/beads.db not created")
	}

	if !strings.Contains(out, "Initialized") {
		t.Errorf("expected 'Initialized' in output, got: %s", out)
	}
}

func TestCLI_Init_AlreadyExists(t *testing.T) {
	dir := t.TempDir()
	oldDir, _ := os.Getwd()
	os.Chdir(dir)
	defer os.Chdir(oldDir)

	// First init
	runCLI([]string{"init"})

	// Second init should succeed (idempotent)
	_, err := runCLI([]string{"init"})
	if err != nil {
		t.Fatalf("second init should succeed: %v", err)
	}
}

func TestCLI_Create(t *testing.T) {
	dir := t.TempDir()
	oldDir, _ := os.Getwd()
	os.Chdir(dir)
	defer os.Chdir(oldDir)

	runCLI([]string{"init"})

	out, err := runCLI([]string{"create", "Test Task"})
	if err != nil {
		t.Fatalf("create failed: %v", err)
	}

	// Output should contain the ID (bl-xxxx format)
	if !strings.Contains(out, "bl-") {
		t.Errorf("expected ID in output, got: %s", out)
	}
}

func TestCLI_Create_NoTitle(t *testing.T) {
	dir := t.TempDir()
	oldDir, _ := os.Getwd()
	os.Chdir(dir)
	defer os.Chdir(oldDir)

	runCLI([]string{"init"})

	_, err := runCLI([]string{"create"})
	if err == nil {
		t.Error("create without title should fail")
	}
}

func TestCLI_List(t *testing.T) {
	dir := t.TempDir()
	oldDir, _ := os.Getwd()
	os.Chdir(dir)
	defer os.Chdir(oldDir)

	runCLI([]string{"init"})
	runCLI([]string{"create", "Task One"})
	runCLI([]string{"create", "Task Two"})

	out, err := runCLI([]string{"list"})
	if err != nil {
		t.Fatalf("list failed: %v", err)
	}

	if !strings.Contains(out, "Task One") {
		t.Errorf("expected 'Task One' in output, got: %s", out)
	}
	if !strings.Contains(out, "Task Two") {
		t.Errorf("expected 'Task Two' in output, got: %s", out)
	}
}

func TestCLI_List_Empty(t *testing.T) {
	dir := t.TempDir()
	oldDir, _ := os.Getwd()
	os.Chdir(dir)
	defer os.Chdir(oldDir)

	runCLI([]string{"init"})

	out, err := runCLI([]string{"list"})
	if err != nil {
		t.Fatalf("list failed: %v", err)
	}

	if !strings.Contains(out, "No issues") {
		t.Errorf("expected 'No issues' in output, got: %s", out)
	}
}

func TestCLI_Show(t *testing.T) {
	dir := t.TempDir()
	oldDir, _ := os.Getwd()
	os.Chdir(dir)
	defer os.Chdir(oldDir)

	runCLI([]string{"init"})
	createOut, _ := runCLI([]string{"create", "My Task"})
	id := extractID(createOut)

	out, err := runCLI([]string{"show", id})
	if err != nil {
		t.Fatalf("show failed: %v", err)
	}

	if !strings.Contains(out, "My Task") {
		t.Errorf("expected 'My Task' in output, got: %s", out)
	}
	if !strings.Contains(out, id) {
		t.Errorf("expected ID in output, got: %s", out)
	}
}

func TestCLI_Show_NotFound(t *testing.T) {
	dir := t.TempDir()
	oldDir, _ := os.Getwd()
	os.Chdir(dir)
	defer os.Chdir(oldDir)

	runCLI([]string{"init"})

	_, err := runCLI([]string{"show", "bl-9999"})
	if err == nil {
		t.Error("show non-existent should fail")
	}
}

func TestCLI_Update(t *testing.T) {
	dir := t.TempDir()
	oldDir, _ := os.Getwd()
	os.Chdir(dir)
	defer os.Chdir(oldDir)

	runCLI([]string{"init"})
	createOut, _ := runCLI([]string{"create", "Original Title"})
	id := extractID(createOut)

	_, err := runCLI([]string{"update", id, "--title", "New Title"})
	if err != nil {
		t.Fatalf("update failed: %v", err)
	}

	showOut, _ := runCLI([]string{"show", id})
	if !strings.Contains(showOut, "New Title") {
		t.Errorf("expected updated title, got: %s", showOut)
	}
}

func TestCLI_Update_Status(t *testing.T) {
	dir := t.TempDir()
	oldDir, _ := os.Getwd()
	os.Chdir(dir)
	defer os.Chdir(oldDir)

	runCLI([]string{"init"})
	createOut, _ := runCLI([]string{"create", "Task"})
	id := extractID(createOut)

	_, err := runCLI([]string{"update", id, "--status", "in_progress"})
	if err != nil {
		t.Fatalf("update status failed: %v", err)
	}

	showOut, _ := runCLI([]string{"show", id})
	if !strings.Contains(showOut, "in_progress") {
		t.Errorf("expected in_progress status, got: %s", showOut)
	}
}

func TestCLI_Close(t *testing.T) {
	dir := t.TempDir()
	oldDir, _ := os.Getwd()
	os.Chdir(dir)
	defer os.Chdir(oldDir)

	runCLI([]string{"init"})
	createOut, _ := runCLI([]string{"create", "Task"})
	id := extractID(createOut)

	_, err := runCLI([]string{"close", id})
	if err != nil {
		t.Fatalf("close failed: %v", err)
	}

	showOut, _ := runCLI([]string{"show", id})
	if !strings.Contains(showOut, "closed") {
		t.Errorf("expected closed status, got: %s", showOut)
	}
}

func TestCLI_Ready(t *testing.T) {
	dir := t.TempDir()
	oldDir, _ := os.Getwd()
	os.Chdir(dir)
	defer os.Chdir(oldDir)

	runCLI([]string{"init"})
	createOut, _ := runCLI([]string{"create", "Ready Task"})
	id := extractID(createOut)

	out, err := runCLI([]string{"ready"})
	if err != nil {
		t.Fatalf("ready failed: %v", err)
	}

	if !strings.Contains(out, id) {
		t.Errorf("expected issue ID in ready output, got: %s", out)
	}
}

func TestCLI_Ready_Empty(t *testing.T) {
	dir := t.TempDir()
	oldDir, _ := os.Getwd()
	os.Chdir(dir)
	defer os.Chdir(oldDir)

	runCLI([]string{"init"})

	out, err := runCLI([]string{"ready"})
	if err != nil {
		t.Fatalf("ready failed: %v", err)
	}

	if !strings.Contains(out, "No ready") {
		t.Errorf("expected 'No ready' in output, got: %s", out)
	}
}

func TestCLI_DepAdd(t *testing.T) {
	dir := t.TempDir()
	oldDir, _ := os.Getwd()
	os.Chdir(dir)
	defer os.Chdir(oldDir)

	runCLI([]string{"init"})
	outA, _ := runCLI([]string{"create", "Task A"})
	outB, _ := runCLI([]string{"create", "Task B"})
	idA := extractID(outA)
	idB := extractID(outB)

	_, err := runCLI([]string{"dep", "add", idB, idA}) // B blocked by A
	if err != nil {
		t.Fatalf("dep add failed: %v", err)
	}

	// B should not be in ready list
	readyOut, _ := runCLI([]string{"ready"})
	if strings.Contains(readyOut, idB) {
		t.Errorf("B should be blocked, but found in ready: %s", readyOut)
	}
	if !strings.Contains(readyOut, idA) {
		t.Errorf("A should be ready: %s", readyOut)
	}
}

func TestCLI_DepRm(t *testing.T) {
	dir := t.TempDir()
	oldDir, _ := os.Getwd()
	os.Chdir(dir)
	defer os.Chdir(oldDir)

	runCLI([]string{"init"})
	outA, _ := runCLI([]string{"create", "Task A"})
	outB, _ := runCLI([]string{"create", "Task B"})
	idA := extractID(outA)
	idB := extractID(outB)

	runCLI([]string{"dep", "add", idB, idA})
	_, err := runCLI([]string{"dep", "rm", idB, idA})
	if err != nil {
		t.Fatalf("dep rm failed: %v", err)
	}

	// B should now be in ready list
	readyOut, _ := runCLI([]string{"ready"})
	if !strings.Contains(readyOut, idB) {
		t.Errorf("B should be ready after dep removed: %s", readyOut)
	}
}

func TestCLI_DepAdd_NotFound(t *testing.T) {
	dir := t.TempDir()
	oldDir, _ := os.Getwd()
	os.Chdir(dir)
	defer os.Chdir(oldDir)

	runCLI([]string{"init"})
	outA, _ := runCLI([]string{"create", "Task A"})
	idA := extractID(outA)

	_, err := runCLI([]string{"dep", "add", "bl-9999", idA})
	if err == nil {
		t.Error("dep add with non-existent issue should fail")
	}
}

// TestCLI_BlockingChain is the key acceptance test from the context packet
func TestCLI_BlockingChain(t *testing.T) {
	dir := t.TempDir()
	oldDir, _ := os.Getwd()
	os.Chdir(dir)
	defer os.Chdir(oldDir)

	// Setup
	runCLI([]string{"init"})
	outA, _ := runCLI([]string{"create", "Task A"})
	outB, _ := runCLI([]string{"create", "Task B"})
	outC, _ := runCLI([]string{"create", "Task C"})

	idA := extractID(outA)
	idB := extractID(outB)
	idC := extractID(outC)

	// B blocked by A, C blocked by B
	runCLI([]string{"dep", "add", idB, idA})
	runCLI([]string{"dep", "add", idC, idB})

	// Only A should be ready
	ready1, _ := runCLI([]string{"ready"})
	if !strings.Contains(ready1, idA) {
		t.Errorf("A should be ready: %s", ready1)
	}
	if strings.Contains(ready1, idB) {
		t.Errorf("B should NOT be ready: %s", ready1)
	}
	if strings.Contains(ready1, idC) {
		t.Errorf("C should NOT be ready: %s", ready1)
	}

	// Close A, now only B should be ready
	runCLI([]string{"close", idA})
	ready2, _ := runCLI([]string{"ready"})
	if strings.Contains(ready2, idA) {
		t.Errorf("A should NOT be in ready (closed): %s", ready2)
	}
	if !strings.Contains(ready2, idB) {
		t.Errorf("B should now be ready: %s", ready2)
	}
	if strings.Contains(ready2, idC) {
		t.Errorf("C should still NOT be ready: %s", ready2)
	}

	// Close B, now C should be ready
	runCLI([]string{"close", idB})
	ready3, _ := runCLI([]string{"ready"})
	if !strings.Contains(ready3, idC) {
		t.Errorf("C should now be ready: %s", ready3)
	}
}

func TestCLI_Help(t *testing.T) {
	out, _ := runCLI([]string{})
	if !strings.Contains(out, "Usage") || !strings.Contains(out, "Commands") {
		t.Errorf("expected help text, got: %s", out)
	}
}

func TestCLI_UnknownCommand(t *testing.T) {
	_, err := runCLI([]string{"bogus"})
	if err == nil {
		t.Error("unknown command should fail")
	}
}

func TestCLI_NoInit(t *testing.T) {
	dir := t.TempDir()
	oldDir, _ := os.Getwd()
	os.Chdir(dir)
	defer os.Chdir(oldDir)

	// Without init, commands should fail gracefully
	_, err := runCLI([]string{"list"})
	if err == nil {
		t.Error("list without init should fail")
	}
}

func TestCLI_Export_Stdout(t *testing.T) {
	dir := t.TempDir()
	oldDir, _ := os.Getwd()
	os.Chdir(dir)
	defer os.Chdir(oldDir)

	runCLI([]string{"init"})
	runCLI([]string{"create", "Export Test"})

	out, err := runCLI([]string{"export"})
	if err != nil {
		t.Fatalf("export failed: %v", err)
	}

	// Should output JSONL to stdout
	if !strings.Contains(out, `"title":"Export Test"`) {
		t.Errorf("expected JSON with title, got: %s", out)
	}
	if !strings.Contains(out, `"dependencies":[]`) {
		t.Errorf("expected dependencies array, got: %s", out)
	}
}

func TestCLI_Export_File(t *testing.T) {
	dir := t.TempDir()
	oldDir, _ := os.Getwd()
	os.Chdir(dir)
	defer os.Chdir(oldDir)

	runCLI([]string{"init"})
	runCLI([]string{"create", "File Export Test"})

	out, err := runCLI([]string{"export", "backup.jsonl"})
	if err != nil {
		t.Fatalf("export to file failed: %v", err)
	}

	if !strings.Contains(out, "Exported to backup.jsonl") {
		t.Errorf("expected confirmation message, got: %s", out)
	}

	// Verify file exists and has content
	data, err := os.ReadFile("backup.jsonl")
	if err != nil {
		t.Fatalf("read backup file: %v", err)
	}
	if !strings.Contains(string(data), "File Export Test") {
		t.Errorf("backup file should contain task title: %s", string(data))
	}
}

func TestCLI_Import(t *testing.T) {
	dir := t.TempDir()
	oldDir, _ := os.Getwd()
	os.Chdir(dir)
	defer os.Chdir(oldDir)

	runCLI([]string{"init"})

	// Create JSONL file
	content := `{"id":"bl-imp1","title":"Imported Task","status":"open","priority":2,"issue_type":"task","created_at":"2026-01-01T00:00:00Z","updated_at":"2026-01-01T00:00:00Z","dependencies":[]}`
	os.WriteFile("import.jsonl", []byte(content), 0644)

	out, err := runCLI([]string{"import", "import.jsonl"})
	if err != nil {
		t.Fatalf("import failed: %v", err)
	}

	if !strings.Contains(out, "1 created") {
		t.Errorf("expected '1 created' in output, got: %s", out)
	}

	// Verify issue exists
	listOut, _ := runCLI([]string{"list"})
	if !strings.Contains(listOut, "Imported Task") {
		t.Errorf("imported task should appear in list: %s", listOut)
	}
}

func TestCLI_Import_NoFile(t *testing.T) {
	dir := t.TempDir()
	oldDir, _ := os.Getwd()
	os.Chdir(dir)
	defer os.Chdir(oldDir)

	runCLI([]string{"init"})

	_, err := runCLI([]string{"import"})
	if err == nil {
		t.Error("import without file should fail")
	}
}

// TestCLI_RoundTrip_Full is the acceptance test from the Phase 3 spec
func TestCLI_RoundTrip_Full(t *testing.T) {
	dir := t.TempDir()
	oldDir, _ := os.Getwd()
	os.Chdir(dir)
	defer os.Chdir(oldDir)

	// Setup: init, create tasks, add dependency
	runCLI([]string{"init"})
	outA, _ := runCLI([]string{"create", "Task A"})
	outB, _ := runCLI([]string{"create", "Task B"})
	idA := extractID(outA)
	idB := extractID(outB)
	runCLI([]string{"dep", "add", idB, idA}) // B blocked by A

	// Export to file
	runCLI([]string{"export", "backup.jsonl"})

	// Verify backup file content
	backupData, _ := os.ReadFile("backup.jsonl")
	if !strings.Contains(string(backupData), idA) {
		t.Fatalf("backup should contain issue A ID")
	}
	if !strings.Contains(string(backupData), `"depends_on"`) {
		t.Fatalf("backup should contain dependency info")
	}

	// Delete the database (simulating corruption recovery)
	os.RemoveAll(".beads-lite")

	// Re-init and import
	runCLI([]string{"init"})
	importOut, err := runCLI([]string{"import", "backup.jsonl"})
	if err != nil {
		t.Fatalf("import after restore failed: %v", err)
	}
	if !strings.Contains(importOut, "2 created") {
		t.Errorf("expected 2 issues created, got: %s", importOut)
	}

	// Verify ready shows Task A (not B which is blocked)
	readyOut, _ := runCLI([]string{"ready"})
	if !strings.Contains(readyOut, "Task A") {
		t.Errorf("Task A should be ready: %s", readyOut)
	}
	if strings.Contains(readyOut, "Task B") {
		t.Errorf("Task B should be blocked: %s", readyOut)
	}

	// Verify list shows both tasks
	listOut, _ := runCLI([]string{"list"})
	if !strings.Contains(listOut, "Task A") || !strings.Contains(listOut, "Task B") {
		t.Errorf("list should show both tasks: %s", listOut)
	}
}

// Helper functions

// runCLI executes the CLI with the given args and returns stdout/stderr combined
func runCLI(args []string) (string, error) {
	var buf bytes.Buffer
	err := Run(args, &buf)
	return buf.String(), err
}

// extractID pulls the bl-xxxx ID from CLI output
func extractID(output string) string {
	lines := strings.Split(output, "\n")
	for _, line := range lines {
		// Look for bl-xxxx pattern
		if idx := strings.Index(line, "bl-"); idx >= 0 {
			// Find the end of the ID (space, tab, newline, or colon)
			id := line[idx:]
			if endIdx := strings.IndexAny(id, " \t\n:"); endIdx > 0 {
				id = id[:endIdx]
			}
			return strings.TrimSpace(id)
		}
	}
	return ""
}

// dbPath returns the database path for the current directory
func dbPath() string {
	return filepath.Join(".beads-lite", "beads.db")
}

// Tests for --json flag (Phase 4)

func TestCLI_List_JSON(t *testing.T) {
	dir := t.TempDir()
	oldDir, _ := os.Getwd()
	os.Chdir(dir)
	defer os.Chdir(oldDir)

	runCLI([]string{"init"})
	runCLI([]string{"create", "JSON Task"})

	out, err := runCLI([]string{"list", "--json"})
	if err != nil {
		t.Fatalf("list --json failed: %v", err)
	}

	// Should be valid JSONL (one JSON object per line)
	if !strings.Contains(out, `"title":"JSON Task"`) {
		t.Errorf("expected JSON with title, got: %s", out)
	}
	if !strings.Contains(out, `"id":"bl-`) {
		t.Errorf("expected JSON with id, got: %s", out)
	}
	if !strings.Contains(out, `"status":"open"`) {
		t.Errorf("expected JSON with status, got: %s", out)
	}
}

func TestCLI_Ready_JSON(t *testing.T) {
	dir := t.TempDir()
	oldDir, _ := os.Getwd()
	os.Chdir(dir)
	defer os.Chdir(oldDir)

	runCLI([]string{"init"})
	runCLI([]string{"create", "Ready JSON Task"})

	out, err := runCLI([]string{"ready", "--json"})
	if err != nil {
		t.Fatalf("ready --json failed: %v", err)
	}

	// Should be valid JSONL
	if !strings.Contains(out, `"title":"Ready JSON Task"`) {
		t.Errorf("expected JSON with title, got: %s", out)
	}
}

func TestCLI_Show_JSON(t *testing.T) {
	dir := t.TempDir()
	oldDir, _ := os.Getwd()
	os.Chdir(dir)
	defer os.Chdir(oldDir)

	runCLI([]string{"init"})
	createOut, _ := runCLI([]string{"create", "Show JSON Task"})
	id := extractID(createOut)

	out, err := runCLI([]string{"show", id, "--json"})
	if err != nil {
		t.Fatalf("show --json failed: %v", err)
	}

	// Should be a single JSON object
	if !strings.Contains(out, `"title":"Show JSON Task"`) {
		t.Errorf("expected JSON with title, got: %s", out)
	}
	if !strings.Contains(out, `"id":"`+id+`"`) {
		t.Errorf("expected JSON with correct id, got: %s", out)
	}
}

// Tests for --tree flag (Phase 4)

func TestCLI_List_Tree(t *testing.T) {
	dir := t.TempDir()
	oldDir, _ := os.Getwd()
	os.Chdir(dir)
	defer os.Chdir(oldDir)

	runCLI([]string{"init"})
	outA, _ := runCLI([]string{"create", "Parent Task"})
	outB, _ := runCLI([]string{"create", "Child Task"})
	idA := extractID(outA)
	idB := extractID(outB)

	// B blocked by A (A is parent, B is child in tree)
	runCLI([]string{"dep", "add", idB, idA})

	out, err := runCLI([]string{"list", "--tree"})
	if err != nil {
		t.Fatalf("list --tree failed: %v", err)
	}

	// Should show tree structure with box-drawing characters
	// Parent should appear, child should be indented under it
	if !strings.Contains(out, "Parent Task") {
		t.Errorf("expected 'Parent Task' in output, got: %s", out)
	}
	if !strings.Contains(out, "Child Task") {
		t.Errorf("expected 'Child Task' in output, got: %s", out)
	}
	// Should have tree drawing characters
	if !strings.Contains(out, "└──") && !strings.Contains(out, "├──") {
		t.Errorf("expected tree drawing characters, got: %s", out)
	}
}

func TestCLI_List_Tree_MultipleRoots(t *testing.T) {
	dir := t.TempDir()
	oldDir, _ := os.Getwd()
	os.Chdir(dir)
	defer os.Chdir(oldDir)

	runCLI([]string{"init"})
	runCLI([]string{"create", "Root One"})
	runCLI([]string{"create", "Root Two"})

	out, err := runCLI([]string{"list", "--tree"})
	if err != nil {
		t.Fatalf("list --tree failed: %v", err)
	}

	// Both roots should appear at the top level (no indentation prefix)
	if !strings.Contains(out, "Root One") {
		t.Errorf("expected 'Root One' in output, got: %s", out)
	}
	if !strings.Contains(out, "Root Two") {
		t.Errorf("expected 'Root Two' in output, got: %s", out)
	}
}

func TestCLI_List_Tree_Chain(t *testing.T) {
	dir := t.TempDir()
	oldDir, _ := os.Getwd()
	os.Chdir(dir)
	defer os.Chdir(oldDir)

	runCLI([]string{"init"})
	outA, _ := runCLI([]string{"create", "Task A"})
	outB, _ := runCLI([]string{"create", "Task B"})
	outC, _ := runCLI([]string{"create", "Task C"})
	idA := extractID(outA)
	idB := extractID(outB)
	idC := extractID(outC)

	// C blocked by B, B blocked by A
	runCLI([]string{"dep", "add", idB, idA})
	runCLI([]string{"dep", "add", idC, idB})

	out, err := runCLI([]string{"list", "--tree"})
	if err != nil {
		t.Fatalf("list --tree failed: %v", err)
	}

	// Should show: A -> B -> C hierarchy
	if !strings.Contains(out, "Task A") {
		t.Errorf("expected 'Task A' in output, got: %s", out)
	}
	if !strings.Contains(out, "Task B") {
		t.Errorf("expected 'Task B' in output, got: %s", out)
	}
	if !strings.Contains(out, "Task C") {
		t.Errorf("expected 'Task C' in output, got: %s", out)
	}
}

// Tests for onboard command (Phase 5)

func TestCLI_Onboard(t *testing.T) {
	// onboard doesn't need init - it just prints instructions
	out, err := runCLI([]string{"onboard"})
	if err != nil {
		t.Fatalf("onboard failed: %v", err)
	}

	// Should contain key elements
	if !strings.Contains(out, "beads-lite") {
		t.Errorf("expected 'beads-lite' in output, got: %s", out)
	}
	if !strings.Contains(out, "bl ready") {
		t.Errorf("expected 'bl ready' in output, got: %s", out)
	}
	if !strings.Contains(out, "bl close") {
		t.Errorf("expected 'bl close' in output, got: %s", out)
	}
	if !strings.Contains(out, "--json") {
		t.Errorf("expected '--json' in output, got: %s", out)
	}
	if !strings.Contains(out, "--tree") {
		t.Errorf("expected '--tree' in output, got: %s", out)
	}
}

func TestCLI_Onboard_IsValidMarkdown(t *testing.T) {
	out, err := runCLI([]string{"onboard"})
	if err != nil {
		t.Fatalf("onboard failed: %v", err)
	}

	// Should start with markdown header
	if !strings.HasPrefix(out, "#") {
		t.Errorf("expected markdown header at start, got: %s", out[:min(50, len(out))])
	}
}
