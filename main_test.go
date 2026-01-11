package beadslite

import (
	"bytes"
	"fmt"
	"os"
	"strings"
	"testing"
)

// CLI tests execute the CLI via runCLI helper and check output/exit codes.

// setupTestDir creates a temp directory and changes to it for the duration of the test.
// Uses t.Cleanup() to automatically restore the working directory when the test completes.
func setupTestDir(t *testing.T) {
	t.Helper()
	dir := t.TempDir()
	oldDir, _ := os.Getwd()
	os.Chdir(dir)
	t.Cleanup(func() { os.Chdir(oldDir) })
}

func TestCLI_Init(t *testing.T) {
	setupTestDir(t)

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
	setupTestDir(t)

	// First init
	runCLI([]string{"init"})

	// Second init should succeed (idempotent)
	_, err := runCLI([]string{"init"})
	if err != nil {
		t.Fatalf("second init should succeed: %v", err)
	}
}

func TestCLI_Create(t *testing.T) {
	setupTestDir(t)

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
	setupTestDir(t)

	runCLI([]string{"init"})

	_, err := runCLI([]string{"create"})
	if err == nil {
		t.Error("create without title should fail")
	}
}

func TestCLI_List(t *testing.T) {
	setupTestDir(t)

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
	setupTestDir(t)

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
	setupTestDir(t)

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
	setupTestDir(t)

	runCLI([]string{"init"})

	_, err := runCLI([]string{"show", "bl-9999"})
	if err == nil {
		t.Error("show non-existent should fail")
	}
}

func TestCLI_Update(t *testing.T) {
	setupTestDir(t)

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
	setupTestDir(t)

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
	setupTestDir(t)

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
	setupTestDir(t)

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
	setupTestDir(t)

	runCLI([]string{"init"})

	out, err := runCLI([]string{"ready"})
	if err != nil {
		t.Fatalf("ready failed: %v", err)
	}

	if !strings.Contains(out, "No issues found") {
		t.Errorf("expected 'No issues found' in output, got: %s", out)
	}
}

func TestCLI_Update_BlockedBy(t *testing.T) {
	setupTestDir(t)

	runCLI([]string{"init"})
	outA, _ := runCLI([]string{"create", "Task A"})
	outB, _ := runCLI([]string{"create", "Task B"})
	idA := extractID(outA)
	idB := extractID(outB)

	_, err := runCLI([]string{"update", idB, "--blocked-by", idA}) // B blocked by A
	if err != nil {
		t.Fatalf("update --blocked-by failed: %v", err)
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

func TestCLI_Update_Unblock(t *testing.T) {
	setupTestDir(t)

	runCLI([]string{"init"})
	outA, _ := runCLI([]string{"create", "Task A"})
	outB, _ := runCLI([]string{"create", "Task B"})
	idA := extractID(outA)
	idB := extractID(outB)

	runCLI([]string{"update", idB, "--blocked-by", idA})
	_, err := runCLI([]string{"update", idB, "--unblock", idA})
	if err != nil {
		t.Fatalf("update --unblock failed: %v", err)
	}

	// B should now be in ready list
	readyOut, _ := runCLI([]string{"ready"})
	if !strings.Contains(readyOut, idB) {
		t.Errorf("B should be ready after blocker removed: %s", readyOut)
	}
}

func TestCLI_Update_BlockedBy_NotFound(t *testing.T) {
	setupTestDir(t)

	runCLI([]string{"init"})
	outA, _ := runCLI([]string{"create", "Task A"})
	idA := extractID(outA)

	// Non-existent issue being updated
	_, err := runCLI([]string{"update", "bl-9999", "--blocked-by", idA})
	if err == nil {
		t.Error("update non-existent issue should fail")
	}

	// Non-existent blocker
	_, err = runCLI([]string{"update", idA, "--blocked-by", "bl-9999"})
	if err == nil {
		t.Error("update with non-existent blocker should fail")
	}
}

// TestCLI_BlockingChain is the key acceptance test from the context packet
func TestCLI_BlockingChain(t *testing.T) {
	setupTestDir(t)

	// Setup
	runCLI([]string{"init"})
	outA, _ := runCLI([]string{"create", "Task A"})
	outB, _ := runCLI([]string{"create", "Task B"})
	outC, _ := runCLI([]string{"create", "Task C"})

	idA := extractID(outA)
	idB := extractID(outB)
	idC := extractID(outC)

	// B blocked by A, C blocked by B
	runCLI([]string{"update", idB, "--blocked-by", idA})
	runCLI([]string{"update", idC, "--blocked-by", idB})

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
	setupTestDir(t)

	// Without init, commands should fail gracefully
	_, err := runCLI([]string{"list"})
	if err == nil {
		t.Error("list without init should fail")
	}
}

func TestCLI_Export_Stdout(t *testing.T) {
	setupTestDir(t)

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
	setupTestDir(t)

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
	setupTestDir(t)

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
	setupTestDir(t)

	runCLI([]string{"init"})

	_, err := runCLI([]string{"import"})
	if err == nil {
		t.Error("import without file should fail")
	}
}

// TestCLI_RoundTrip_Full is the acceptance test from the Phase 3 spec
func TestCLI_RoundTrip_Full(t *testing.T) {
	setupTestDir(t)

	// Setup: init, create tasks, add dependency
	runCLI([]string{"init"})
	outA, _ := runCLI([]string{"create", "Task A"})
	outB, _ := runCLI([]string{"create", "Task B"})
	idA := extractID(outA)
	idB := extractID(outB)
	runCLI([]string{"update", idB, "--blocked-by", idA}) // B blocked by A

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

// Tests for --json flag (Phase 4)

func TestCLI_List_JSON(t *testing.T) {
	setupTestDir(t)

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
	setupTestDir(t)

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

func TestCLI_Ready_Tree(t *testing.T) {
	setupTestDir(t)

	runCLI([]string{"init"})

	// Create parent and child tasks
	parentOut, _ := runCLI([]string{"create", "Parent Task"})
	parentID := extractID(parentOut)
	childOut, _ := runCLI([]string{"create", "Child Task"})
	childID := extractID(childOut)

	// Add blocker (child blocked by parent)
	runCLI([]string{"update", childID, "--blocked-by", parentID})

	// Ready --tree should show hierarchical view
	out, err := runCLI([]string{"ready", "--tree"})
	if err != nil {
		t.Fatalf("ready --tree failed: %v", err)
	}

	// Should show parent (the only ready task, since child is blocked)
	if !strings.Contains(out, "Parent Task") {
		t.Errorf("expected Parent Task in tree output: %s", out)
	}
	// Child should NOT be shown (it's blocked)
	if strings.Contains(out, "Child Task") {
		t.Errorf("Child Task should not appear in ready tree (it's blocked): %s", out)
	}
}

func TestCLI_Show_JSON(t *testing.T) {
	setupTestDir(t)

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
	setupTestDir(t)

	runCLI([]string{"init"})
	outA, _ := runCLI([]string{"create", "Parent Task"})
	outB, _ := runCLI([]string{"create", "Child Task"})
	idA := extractID(outA)
	idB := extractID(outB)

	// B blocked by A (A is parent, B is child in tree)
	runCLI([]string{"update", idB, "--blocked-by", idA})

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
	setupTestDir(t)

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
	setupTestDir(t)

	runCLI([]string{"init"})
	outA, _ := runCLI([]string{"create", "Task A"})
	outB, _ := runCLI([]string{"create", "Task B"})
	outC, _ := runCLI([]string{"create", "Task C"})
	idA := extractID(outA)
	idB := extractID(outB)
	idC := extractID(outC)

	// C blocked by B, B blocked by A
	runCLI([]string{"update", idB, "--blocked-by", idA})
	runCLI([]string{"update", idC, "--blocked-by", idB})

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

// Tests for --description flag

func TestCLI_Create_WithDescription(t *testing.T) {
	setupTestDir(t)

	runCLI([]string{"init"})

	out, err := runCLI([]string{"create", "Fix bug", "--description", "Race condition in auth middleware"})
	if err != nil {
		t.Fatalf("create with description failed: %v", err)
	}

	id := extractID(out)

	// Verify description is stored
	showOut, _ := runCLI([]string{"show", id})
	if !strings.Contains(showOut, "Race condition in auth middleware") {
		t.Errorf("expected description in show output, got: %s", showOut)
	}
}

func TestCLI_Update_Description(t *testing.T) {
	setupTestDir(t)

	runCLI([]string{"init"})
	createOut, _ := runCLI([]string{"create", "Task"})
	id := extractID(createOut)

	_, err := runCLI([]string{"update", id, "--description", "Added via update"})
	if err != nil {
		t.Fatalf("update description failed: %v", err)
	}

	showOut, _ := runCLI([]string{"show", id})
	if !strings.Contains(showOut, "Added via update") {
		t.Errorf("expected updated description, got: %s", showOut)
	}
}

func TestCLI_Create_Description_JSON(t *testing.T) {
	setupTestDir(t)

	runCLI([]string{"init"})
	createOut, _ := runCLI([]string{"create", "Task", "--description", "Test description"})
	id := extractID(createOut)

	out, err := runCLI([]string{"show", id, "--json"})
	if err != nil {
		t.Fatalf("show --json failed: %v", err)
	}

	if !strings.Contains(out, `"description":"Test description"`) {
		t.Errorf("expected description in JSON output, got: %s", out)
	}
}

// Tests for filtering flags (--status, --priority, --type)

func TestCLI_List_FilterByStatus(t *testing.T) {
	setupTestDir(t)

	runCLI([]string{"init"})
	outOpen, _ := runCLI([]string{"create", "Open Task"})
	outClosed, _ := runCLI([]string{"create", "Closed Task"})
	idClosed := extractID(outClosed)
	runCLI([]string{"close", idClosed})

	// Filter by open status
	out, err := runCLI([]string{"list", "--status", "open"})
	if err != nil {
		t.Fatalf("list --status open failed: %v", err)
	}
	if !strings.Contains(out, "Open Task") {
		t.Errorf("expected 'Open Task' in output, got: %s", out)
	}
	if strings.Contains(out, "Closed Task") {
		t.Errorf("should NOT contain 'Closed Task', got: %s", out)
	}

	// Filter by closed status
	outClosed2, _ := runCLI([]string{"list", "--status", "closed"})
	if strings.Contains(outClosed2, "Open Task") {
		t.Errorf("should NOT contain 'Open Task' when filtering closed, got: %s", outClosed2)
	}
	if !strings.Contains(outClosed2, "Closed Task") {
		t.Errorf("expected 'Closed Task' in closed filter, got: %s", outClosed2)
	}
	_ = outOpen // silence unused
}

func TestCLI_List_FilterByPriority(t *testing.T) {
	setupTestDir(t)

	runCLI([]string{"init"})
	outP1, _ := runCLI([]string{"create", "P1 Task"})
	runCLI([]string{"create", "P2 Task"}) // default priority is P2
	idP1 := extractID(outP1)
	runCLI([]string{"update", idP1, "--priority", "1"})

	// Filter by P1
	out, err := runCLI([]string{"list", "--priority", "1"})
	if err != nil {
		t.Fatalf("list --priority 1 failed: %v", err)
	}
	if !strings.Contains(out, "P1 Task") {
		t.Errorf("expected 'P1 Task' in output, got: %s", out)
	}
	if strings.Contains(out, "P2 Task") {
		t.Errorf("should NOT contain 'P2 Task', got: %s", out)
	}
}

func TestCLI_List_FilterByType(t *testing.T) {
	setupTestDir(t)

	runCLI([]string{"init"})
	outBug, _ := runCLI([]string{"create", "Bug Report"})
	runCLI([]string{"create", "Feature Request"})
	idBug := extractID(outBug)
	runCLI([]string{"update", idBug, "--type", "bug"})

	// Filter by bug type
	out, err := runCLI([]string{"list", "--type", "bug"})
	if err != nil {
		t.Fatalf("list --type bug failed: %v", err)
	}
	if !strings.Contains(out, "Bug Report") {
		t.Errorf("expected 'Bug Report' in output, got: %s", out)
	}
	if strings.Contains(out, "Feature Request") {
		t.Errorf("should NOT contain 'Feature Request', got: %s", out)
	}
}

func TestCLI_List_CombinedFilters(t *testing.T) {
	setupTestDir(t)

	runCLI([]string{"init"})
	// Create 4 tasks with different combinations
	out1, _ := runCLI([]string{"create", "Open P1 Bug"})
	out2, _ := runCLI([]string{"create", "Open P2 Bug"})
	out3, _ := runCLI([]string{"create", "Open P1 Task"})
	out4, _ := runCLI([]string{"create", "Closed P1 Bug"})

	id1 := extractID(out1)
	id2 := extractID(out2)
	id3 := extractID(out3)
	id4 := extractID(out4)

	runCLI([]string{"update", id1, "--priority", "1", "--type", "bug"})
	runCLI([]string{"update", id2, "--type", "bug"})
	runCLI([]string{"update", id3, "--priority", "1"})
	runCLI([]string{"update", id4, "--priority", "1", "--type", "bug"})
	runCLI([]string{"close", id4})

	// Filter: open + P1 + bug -> only "Open P1 Bug"
	out, err := runCLI([]string{"list", "--status", "open", "--priority", "1", "--type", "bug"})
	if err != nil {
		t.Fatalf("combined filter failed: %v", err)
	}
	if !strings.Contains(out, "Open P1 Bug") {
		t.Errorf("expected 'Open P1 Bug' in output, got: %s", out)
	}
	if strings.Contains(out, "Open P2 Bug") || strings.Contains(out, "Open P1 Task") || strings.Contains(out, "Closed P1 Bug") {
		t.Errorf("should only contain 'Open P1 Bug', got: %s", out)
	}
}

func TestCLI_Ready_FilterByPriority(t *testing.T) {
	setupTestDir(t)

	runCLI([]string{"init"})
	outP0, _ := runCLI([]string{"create", "Critical Task"})
	runCLI([]string{"create", "Normal Task"})
	idP0 := extractID(outP0)
	runCLI([]string{"update", idP0, "--priority", "0"})

	// Filter ready by P0
	out, err := runCLI([]string{"ready", "--priority", "0"})
	if err != nil {
		t.Fatalf("ready --priority 0 failed: %v", err)
	}
	if !strings.Contains(out, "Critical Task") {
		t.Errorf("expected 'Critical Task' in output, got: %s", out)
	}
	if strings.Contains(out, "Normal Task") {
		t.Errorf("should NOT contain 'Normal Task', got: %s", out)
	}
}

func TestCLI_Ready_FilterByType(t *testing.T) {
	setupTestDir(t)

	runCLI([]string{"init"})
	outBug, _ := runCLI([]string{"create", "Fix Bug"})
	runCLI([]string{"create", "Add Feature"})
	idBug := extractID(outBug)
	runCLI([]string{"update", idBug, "--type", "bug"})

	// Filter ready by bug
	out, err := runCLI([]string{"ready", "--type", "bug"})
	if err != nil {
		t.Fatalf("ready --type bug failed: %v", err)
	}
	if !strings.Contains(out, "Fix Bug") {
		t.Errorf("expected 'Fix Bug' in output, got: %s", out)
	}
	if strings.Contains(out, "Add Feature") {
		t.Errorf("should NOT contain 'Add Feature', got: %s", out)
	}
}

// Tests for delete command

func TestCLI_Delete_RequiresConfirm(t *testing.T) {
	setupTestDir(t)

	runCLI([]string{"init"})
	createOut, _ := runCLI([]string{"create", "Task to Delete"})
	id := extractID(createOut)

	// Without --confirm, should fail
	_, err := runCLI([]string{"delete", id})
	if err == nil {
		t.Error("delete without --confirm should fail")
	}

	// Task should still exist
	_, showErr := runCLI([]string{"show", id})
	if showErr != nil {
		t.Errorf("task should still exist after failed delete: %v", showErr)
	}
}

func TestCLI_Delete_WithConfirm(t *testing.T) {
	setupTestDir(t)

	runCLI([]string{"init"})
	createOut, _ := runCLI([]string{"create", "Task to Delete"})
	id := extractID(createOut)

	// With --confirm, should succeed
	out, err := runCLI([]string{"delete", id, "--confirm"})
	if err != nil {
		t.Fatalf("delete with --confirm failed: %v", err)
	}
	if !strings.Contains(out, "Deleted") {
		t.Errorf("expected 'Deleted' in output, got: %s", out)
	}

	// Task should be gone
	_, showErr := runCLI([]string{"show", id})
	if showErr == nil {
		t.Error("task should not exist after delete")
	}
}

func TestCLI_Delete_NotFound(t *testing.T) {
	setupTestDir(t)

	runCLI([]string{"init"})

	_, err := runCLI([]string{"delete", "bl-9999", "--confirm"})
	if err == nil {
		t.Error("delete non-existent should fail")
	}
}

func TestCLI_Delete_RemovesDependencies(t *testing.T) {
	setupTestDir(t)

	runCLI([]string{"init"})
	outA, _ := runCLI([]string{"create", "Task A"})
	outB, _ := runCLI([]string{"create", "Task B"})
	idA := extractID(outA)
	idB := extractID(outB)

	// B blocked by A
	runCLI([]string{"update", idB, "--blocked-by", idA})

	// B should be blocked
	ready1, _ := runCLI([]string{"ready"})
	if strings.Contains(ready1, "Task B") {
		t.Errorf("B should be blocked before delete: %s", ready1)
	}

	// Delete A
	runCLI([]string{"delete", idA, "--confirm"})

	// B should now be ready (dependency removed)
	ready2, _ := runCLI([]string{"ready"})
	if !strings.Contains(ready2, "Task B") {
		t.Errorf("B should be ready after blocker deleted: %s", ready2)
	}
}

// Tests for create command extended flags (bl-cl0q)

func TestCLI_Create_WithPriority(t *testing.T) {
	setupTestDir(t)

	runCLI([]string{"init"})

	out, err := runCLI([]string{"create", "Critical Bug", "--priority", "0"})
	if err != nil {
		t.Fatalf("create with priority failed: %v", err)
	}

	id := extractID(out)
	showOut, _ := runCLI([]string{"show", id})
	if !strings.Contains(showOut, "P0") {
		t.Errorf("expected P0 priority, got: %s", showOut)
	}
}

func TestCLI_Create_WithType(t *testing.T) {
	setupTestDir(t)

	runCLI([]string{"init"})

	out, err := runCLI([]string{"create", "New Feature", "--type", "feature"})
	if err != nil {
		t.Fatalf("create with type failed: %v", err)
	}

	id := extractID(out)
	showOut, _ := runCLI([]string{"show", id})
	if !strings.Contains(showOut, "feature") {
		t.Errorf("expected feature type, got: %s", showOut)
	}
}

func TestCLI_Create_WithBlockedBy(t *testing.T) {
	setupTestDir(t)

	runCLI([]string{"init"})

	// Create blocker first
	blockerOut, _ := runCLI([]string{"create", "Blocker Task"})
	blockerID := extractID(blockerOut)

	// Create task blocked by blocker
	out, err := runCLI([]string{"create", "Blocked Task", "--blocked-by", blockerID})
	if err != nil {
		t.Fatalf("create with blocked-by failed: %v", err)
	}

	blockedID := extractID(out)

	// Blocked task should NOT be in ready list
	readyOut, _ := runCLI([]string{"ready"})
	if strings.Contains(readyOut, "Blocked Task") {
		t.Errorf("blocked task should not be ready: %s", readyOut)
	}
	if !strings.Contains(readyOut, "Blocker Task") {
		t.Errorf("blocker should be ready: %s", readyOut)
	}

	// Close blocker, blocked task should become ready
	runCLI([]string{"close", blockerID})
	readyOut2, _ := runCLI([]string{"ready"})
	if !strings.Contains(readyOut2, blockedID) {
		t.Errorf("blocked task should be ready after blocker closed: %s", readyOut2)
	}
}

func TestCLI_Create_WithMultipleBlockedBy(t *testing.T) {
	setupTestDir(t)

	runCLI([]string{"init"})

	// Create two blockers
	blocker1Out, _ := runCLI([]string{"create", "Blocker One"})
	blocker2Out, _ := runCLI([]string{"create", "Blocker Two"})
	blocker1ID := extractID(blocker1Out)
	blocker2ID := extractID(blocker2Out)

	// Create task blocked by both
	out, err := runCLI([]string{"create", "Double Blocked", "--blocked-by", blocker1ID, "--blocked-by", blocker2ID})
	if err != nil {
		t.Fatalf("create with multiple blocked-by failed: %v", err)
	}

	blockedID := extractID(out)

	// Should not be ready
	readyOut, _ := runCLI([]string{"ready"})
	if strings.Contains(readyOut, "Double Blocked") {
		t.Errorf("double blocked task should not be ready: %s", readyOut)
	}

	// Close first blocker - still blocked by second
	runCLI([]string{"close", blocker1ID})
	readyOut2, _ := runCLI([]string{"ready"})
	if strings.Contains(readyOut2, "Double Blocked") {
		t.Errorf("should still be blocked by second blocker: %s", readyOut2)
	}

	// Close second blocker - now ready
	runCLI([]string{"close", blocker2ID})
	readyOut3, _ := runCLI([]string{"ready"})
	if !strings.Contains(readyOut3, blockedID) {
		t.Errorf("should be ready after both blockers closed: %s", readyOut3)
	}
}

func TestCLI_Create_BlockedByInvalid(t *testing.T) {
	setupTestDir(t)

	runCLI([]string{"init"})

	// Try to create with non-existent blocker
	_, err := runCLI([]string{"create", "Task", "--blocked-by", "bl-9999"})
	if err == nil {
		t.Error("create with non-existent blocker should fail")
	}
}

func TestCLI_Update_CycleDetection(t *testing.T) {
	setupTestDir(t)

	runCLI([]string{"init"})
	outA, _ := runCLI([]string{"create", "Task A"})
	outB, _ := runCLI([]string{"create", "Task B"})
	idA := extractID(outA)
	idB := extractID(outB)

	// A blocked by B
	runCLI([]string{"update", idA, "--blocked-by", idB})

	// B blocked by A - creates cycle
	// Currently this is allowed by the storage layer.
	// This test documents the current behavior.
	_, err := runCLI([]string{"update", idB, "--blocked-by", idA})
	// NOTE: Currently cycles ARE allowed. This test documents this behavior.
	// If cycle detection is added, this test should change to expect an error.
	if err != nil {
		t.Logf("Cycle was rejected (good): %v", err)
	} else {
		t.Log("Cycle was allowed (current behavior - no cycle detection)")
		// Both tasks should still appear somewhere since there's no blocking algorithm protection
		readyOut, _ := runCLI([]string{"ready"})
		listOut, _ := runCLI([]string{"list"})
		t.Logf("ready: %s", readyOut)
		t.Logf("list: %s", listOut)
	}
}

func TestCLI_Close_NotFound(t *testing.T) {
	setupTestDir(t)

	runCLI([]string{"init"})

	_, err := runCLI([]string{"close", "bl-9999"})
	if err == nil {
		t.Error("close non-existent should fail")
	}
}

func TestCLI_Update_SelfReference(t *testing.T) {
	setupTestDir(t)

	runCLI([]string{"init"})
	out, _ := runCLI([]string{"create", "Task"})
	id := extractID(out)

	// Try to make task block itself
	_, err := runCLI([]string{"update", id, "--blocked-by", id})
	if err == nil {
		t.Error("self-reference dependency should fail")
	}
}

func TestCLI_Create_AllFlagsCombined(t *testing.T) {
	setupTestDir(t)

	runCLI([]string{"init"})

	blockerOut, _ := runCLI([]string{"create", "Epic"})
	blockerID := extractID(blockerOut)

	// Create with all flags at once
	out, err := runCLI([]string{"create", "Full Featured Task",
		"--description", "Detailed description here",
		"--priority", "1",
		"--type", "bug",
		"--blocked-by", blockerID})
	if err != nil {
		t.Fatalf("create with all flags failed: %v", err)
	}

	id := extractID(out)
	showOut, _ := runCLI([]string{"show", id})

	if !strings.Contains(showOut, "Full Featured Task") {
		t.Errorf("missing title: %s", showOut)
	}
	if !strings.Contains(showOut, "Detailed description here") {
		t.Errorf("missing description: %s", showOut)
	}
	if !strings.Contains(showOut, "P1") {
		t.Errorf("missing P1 priority: %s", showOut)
	}
	if !strings.Contains(showOut, "bug") {
		t.Errorf("missing bug type: %s", showOut)
	}

	// Should be blocked
	readyOut, _ := runCLI([]string{"ready"})
	if strings.Contains(readyOut, "Full Featured Task") {
		t.Errorf("should be blocked: %s", readyOut)
	}
}

func TestCLI_Create_InvalidPriority(t *testing.T) {
	setupTestDir(t)
	runCLI([]string{"init"})

	// Priority too high
	_, err := runCLI([]string{"create", "Test", "--priority", "5"})
	if err == nil {
		t.Error("create with priority 5 should fail")
	}

	// Priority too low (negative)
	_, err = runCLI([]string{"create", "Test", "--priority", "-1"})
	if err == nil {
		t.Error("create with negative priority should fail")
	}
}

func TestCLI_Create_InvalidType(t *testing.T) {
	setupTestDir(t)
	runCLI([]string{"init"})

	_, err := runCLI([]string{"create", "Test", "--type", "invalid"})
	if err == nil {
		t.Error("create with invalid type should fail")
	}
}

func TestCLI_List_InvalidStatus(t *testing.T) {
	setupTestDir(t)
	runCLI([]string{"init"})
	runCLI([]string{"create", "Test Task"})

	_, err := runCLI([]string{"list", "--status", "invalid"})
	if err == nil {
		t.Error("list with invalid status should fail")
	}
}

func TestCLI_List_InvalidPriority(t *testing.T) {
	setupTestDir(t)
	runCLI([]string{"init"})
	runCLI([]string{"create", "Test Task"})

	// Priority too high
	_, err := runCLI([]string{"list", "--priority", "5"})
	if err == nil {
		t.Error("list with priority 5 should fail")
	}

	// Negative priority is valid (means "no filter")
	_, err = runCLI([]string{"list", "--priority", "-1"})
	if err != nil {
		t.Errorf("negative priority should be valid (no filter): %v", err)
	}
}

func TestCLI_List_InvalidType(t *testing.T) {
	setupTestDir(t)
	runCLI([]string{"init"})
	runCLI([]string{"create", "Test Task"})

	_, err := runCLI([]string{"list", "--type", "invalid"})
	if err == nil {
		t.Error("list with invalid type should fail")
	}
}

func TestCLI_Ready_InvalidPriority(t *testing.T) {
	setupTestDir(t)
	runCLI([]string{"init"})
	runCLI([]string{"create", "Test Task"})

	_, err := runCLI([]string{"ready", "--priority", "5"})
	if err == nil {
		t.Error("ready with invalid priority should fail")
	}
}

func TestCLI_Ready_InvalidType(t *testing.T) {
	setupTestDir(t)
	runCLI([]string{"init"})
	runCLI([]string{"create", "Test Task"})

	_, err := runCLI([]string{"ready", "--type", "invalid"})
	if err == nil {
		t.Error("ready with invalid type should fail")
	}
}

// TestCLI_Ready_DiamondDependency tests diamond dependency pattern:
// A blocks B and C, both B and C block D.
// Only A should be ready until A is closed.
func TestCLI_Ready_DiamondDependency(t *testing.T) {
	setupTestDir(t)
	runCLI([]string{"init"})

	// Create 4 tasks: A, B, C, D
	outA, _ := runCLI([]string{"create", "Task A"})
	idA := extractID(outA)
	outB, _ := runCLI([]string{"create", "Task B"})
	idB := extractID(outB)
	outC, _ := runCLI([]string{"create", "Task C"})
	idC := extractID(outC)
	outD, _ := runCLI([]string{"create", "Task D"})
	idD := extractID(outD)

	// Diamond: A blocks B, A blocks C, B blocks D, C blocks D
	runCLI([]string{"update", idB, "--blocked-by", idA}) // B blocked by A
	runCLI([]string{"update", idC, "--blocked-by", idA}) // C blocked by A
	runCLI([]string{"update", idD, "--blocked-by", idB}) // D blocked by B
	runCLI([]string{"update", idD, "--blocked-by", idC}) // D blocked by C

	// Only A should be ready
	ready1, _ := runCLI([]string{"ready"})
	if !strings.Contains(ready1, "Task A") {
		t.Errorf("expected Task A to be ready: %s", ready1)
	}
	if strings.Contains(ready1, "Task B") || strings.Contains(ready1, "Task C") || strings.Contains(ready1, "Task D") {
		t.Errorf("only Task A should be ready: %s", ready1)
	}

	// Close A - now B and C should be ready
	runCLI([]string{"close", idA})
	ready2, _ := runCLI([]string{"ready"})
	if !strings.Contains(ready2, "Task B") {
		t.Errorf("expected Task B to be ready: %s", ready2)
	}
	if !strings.Contains(ready2, "Task C") {
		t.Errorf("expected Task C to be ready: %s", ready2)
	}
	if strings.Contains(ready2, "Task D") {
		t.Errorf("Task D should still be blocked: %s", ready2)
	}

	// Close B - D still blocked by C
	runCLI([]string{"close", idB})
	ready3, _ := runCLI([]string{"ready"})
	if strings.Contains(ready3, "Task D") {
		t.Errorf("Task D should still be blocked by C: %s", ready3)
	}

	// Close C - now D is ready
	runCLI([]string{"close", idC})
	ready4, _ := runCLI([]string{"ready"})
	if !strings.Contains(ready4, "Task D") {
		t.Errorf("expected Task D to be ready: %s", ready4)
	}
}

// TestCLI_Ready_ChainedBlocking tests that blocking propagates through chains.
// If A blocks B and B blocks C, then A being open blocks both B and C.
func TestCLI_Ready_ChainedBlocking(t *testing.T) {
	setupTestDir(t)
	runCLI([]string{"init"})

	// Create 3 tasks: Blocker, Middle, End
	outBlocker, _ := runCLI([]string{"create", "Blocker Task"})
	idBlocker := extractID(outBlocker)
	outMiddle, _ := runCLI([]string{"create", "Middle Task"})
	idMiddle := extractID(outMiddle)
	outEnd, _ := runCLI([]string{"create", "End Task"})
	idEnd := extractID(outEnd)

	// Chain: Blocker blocks Middle, Middle blocks End
	runCLI([]string{"update", idMiddle, "--blocked-by", idBlocker}) // Middle blocked by Blocker
	runCLI([]string{"update", idEnd, "--blocked-by", idMiddle})     // End blocked by Middle

	// Only Blocker should be ready
	ready1, _ := runCLI([]string{"ready"})
	if !strings.Contains(ready1, "Blocker Task") {
		t.Errorf("expected Blocker Task to be ready: %s", ready1)
	}
	if strings.Contains(ready1, "Middle Task") || strings.Contains(ready1, "End Task") {
		t.Errorf("only Blocker should be ready: %s", ready1)
	}

	// Close Blocker - Middle becomes ready, but End still blocked by Middle
	runCLI([]string{"close", idBlocker})
	ready2, _ := runCLI([]string{"ready"})
	if !strings.Contains(ready2, "Middle Task") {
		t.Errorf("expected Middle Task to be ready: %s", ready2)
	}
	if strings.Contains(ready2, "End Task") {
		t.Errorf("End Task should still be blocked by Middle: %s", ready2)
	}

	// Close Middle - End becomes ready
	runCLI([]string{"close", idMiddle})
	ready3, _ := runCLI([]string{"ready"})
	if !strings.Contains(ready3, "End Task") {
		t.Errorf("expected End Task to be ready: %s", ready3)
	}
}

// TestCLI_Ready_PartiallyClosedBlockers tests that a task is blocked until ALL blockers are closed.
func TestCLI_Ready_PartiallyClosedBlockers(t *testing.T) {
	setupTestDir(t)
	runCLI([]string{"init"})

	// Create 3 tasks: A, B both block C
	outA, _ := runCLI([]string{"create", "Task A"})
	idA := extractID(outA)
	outB, _ := runCLI([]string{"create", "Task B"})
	idB := extractID(outB)
	outC, _ := runCLI([]string{"create", "Task C"})
	idC := extractID(outC)

	// C blocked by both A and B
	runCLI([]string{"update", idC, "--blocked-by", idA})
	runCLI([]string{"update", idC, "--blocked-by", idB})

	// A and B ready, C blocked
	ready1, _ := runCLI([]string{"ready"})
	if strings.Contains(ready1, "Task C") {
		t.Errorf("Task C should be blocked: %s", ready1)
	}

	// Close A - C still blocked by B
	runCLI([]string{"close", idA})
	ready2, _ := runCLI([]string{"ready"})
	if strings.Contains(ready2, "Task C") {
		t.Errorf("Task C should still be blocked by B: %s", ready2)
	}

	// Close B - C now ready
	runCLI([]string{"close", idB})
	ready3, _ := runCLI([]string{"ready"})
	if !strings.Contains(ready3, "Task C") {
		t.Errorf("expected Task C to be ready: %s", ready3)
	}
}

// P1 Test Coverage: Import error paths

func TestCLI_Import_MalformedJSON(t *testing.T) {
	setupTestDir(t)
	runCLI([]string{"init"})

	// Create file with malformed JSON
	content := `{"id":"bl-good","title":"Good Task","status":"open","priority":2,"issue_type":"task","created_at":"2026-01-01T00:00:00Z","updated_at":"2026-01-01T00:00:00Z","dependencies":[]}
{not valid json at all
{"id":"bl-also","title":"Also Good","status":"open","priority":2,"issue_type":"task","created_at":"2026-01-01T00:00:00Z","updated_at":"2026-01-01T00:00:00Z","dependencies":[]}`
	os.WriteFile("malformed.jsonl", []byte(content), 0644)

	_, err := runCLI([]string{"import", "malformed.jsonl"})
	if err == nil {
		t.Error("import with malformed JSON should fail")
	}
	if !strings.Contains(err.Error(), "parse error") {
		t.Errorf("expected parse error, got: %v", err)
	}
}

func TestCLI_Import_NonExistentDependency(t *testing.T) {
	setupTestDir(t)
	runCLI([]string{"init"})

	// Create file with dependency referencing non-existent issue
	content := `{"id":"bl-orphan","title":"Orphan Task","status":"open","priority":2,"issue_type":"task","created_at":"2026-01-01T00:00:00Z","updated_at":"2026-01-01T00:00:00Z","dependencies":[{"depends_on":"bl-nonexistent","type":"blocks"}]}`
	os.WriteFile("orphan.jsonl", []byte(content), 0644)

	// Import should succeed (FK not enforced at runtime), but we document this behavior
	out, err := runCLI([]string{"import", "orphan.jsonl"})
	if err != nil {
		t.Logf("Import with orphan dep failed (stricter behavior): %v", err)
	} else {
		t.Logf("Import with orphan dep succeeded: %s", out)
		// Verify the issue was created
		listOut, _ := runCLI([]string{"list"})
		if !strings.Contains(listOut, "Orphan Task") {
			t.Errorf("orphan task should be created: %s", listOut)
		}
	}
}

func TestCLI_Close_AlreadyClosed(t *testing.T) {
	setupTestDir(t)
	runCLI([]string{"init"})

	createOut, _ := runCLI([]string{"create", "Task"})
	id := extractID(createOut)

	// Close once
	_, err := runCLI([]string{"close", id})
	if err != nil {
		t.Fatalf("first close should succeed: %v", err)
	}

	// Close again - should be idempotent (not error)
	_, err = runCLI([]string{"close", id})
	// Document current behavior: closing already-closed issue succeeds (idempotent)
	if err != nil {
		t.Logf("double close failed (stricter behavior): %v", err)
	} else {
		t.Log("double close succeeded (idempotent behavior)")
	}

	// Verify still closed
	showOut, _ := runCLI([]string{"show", id})
	if !strings.Contains(showOut, "closed") {
		t.Errorf("issue should still be closed: %s", showOut)
	}
}

// Resolution tests

func TestCLI_Close_Resolution(t *testing.T) {
	setupTestDir(t)
	runCLI([]string{"init"})

	// Test default resolution (done)
	createOut1, _ := runCLI([]string{"create", "Task 1"})
	id1 := extractID(createOut1)
	runCLI([]string{"close", id1})
	showOut1, _ := runCLI([]string{"show", id1})
	if !strings.Contains(showOut1, "Resolution: done") {
		t.Errorf("expected 'Resolution: done' in output, got: %s", showOut1)
	}

	// Test explicit wontfix resolution
	createOut2, _ := runCLI([]string{"create", "Task 2"})
	id2 := extractID(createOut2)
	runCLI([]string{"close", id2, "--resolution", "wontfix"})
	showOut2, _ := runCLI([]string{"show", id2})
	if !strings.Contains(showOut2, "Resolution: wontfix") {
		t.Errorf("expected 'Resolution: wontfix' in output, got: %s", showOut2)
	}

	// Test duplicate resolution
	createOut3, _ := runCLI([]string{"create", "Task 3"})
	id3 := extractID(createOut3)
	runCLI([]string{"close", id3, "--resolution", "duplicate"})
	showOut3, _ := runCLI([]string{"show", id3})
	if !strings.Contains(showOut3, "Resolution: duplicate") {
		t.Errorf("expected 'Resolution: duplicate' in output, got: %s", showOut3)
	}
}

func TestCLI_Close_InvalidResolution(t *testing.T) {
	setupTestDir(t)
	runCLI([]string{"init"})

	createOut, _ := runCLI([]string{"create", "Task"})
	id := extractID(createOut)

	_, err := runCLI([]string{"close", id, "--resolution", "invalid"})
	if err == nil {
		t.Error("expected error for invalid resolution")
	}
	if !strings.Contains(err.Error(), "invalid resolution") {
		t.Errorf("expected 'invalid resolution' error, got: %v", err)
	}
}

func TestCLI_List_FilterResolution(t *testing.T) {
	setupTestDir(t)
	runCLI([]string{"init"})

	// Create and close with different resolutions
	createOut1, _ := runCLI([]string{"create", "Done Task"})
	id1 := extractID(createOut1)
	runCLI([]string{"close", id1, "--resolution", "done"})

	createOut2, _ := runCLI([]string{"create", "Wontfix Task"})
	id2 := extractID(createOut2)
	runCLI([]string{"close", id2, "--resolution", "wontfix"})

	createOut3, _ := runCLI([]string{"create", "Another Wontfix"})
	id3 := extractID(createOut3)
	runCLI([]string{"close", id3, "--resolution", "wontfix"})

	// Filter by wontfix
	out, _ := runCLI([]string{"list", "--status", "closed", "--resolution", "wontfix"})
	if !strings.Contains(out, id2) || !strings.Contains(out, id3) {
		t.Errorf("expected wontfix issues in output, got: %s", out)
	}
	if strings.Contains(out, id1) {
		t.Errorf("done issue should not appear in wontfix filter, got: %s", out)
	}

	// Filter by done
	out2, _ := runCLI([]string{"list", "--status", "closed", "--resolution", "done"})
	if !strings.Contains(out2, id1) {
		t.Errorf("expected done issue in output, got: %s", out2)
	}
	if strings.Contains(out2, id2) || strings.Contains(out2, id3) {
		t.Errorf("wontfix issues should not appear in done filter, got: %s", out2)
	}
}

// P2 Test Coverage: Update validation tests

func TestCLI_Update_InvalidStatus(t *testing.T) {
	setupTestDir(t)
	runCLI([]string{"init"})

	createOut, _ := runCLI([]string{"create", "Task"})
	id := extractID(createOut)

	_, err := runCLI([]string{"update", id, "--status", "invalid"})
	if err == nil {
		t.Error("update with invalid status should fail")
	}
}

func TestCLI_Update_InvalidPriority(t *testing.T) {
	setupTestDir(t)
	runCLI([]string{"init"})

	createOut, _ := runCLI([]string{"create", "Task"})
	id := extractID(createOut)

	_, err := runCLI([]string{"update", id, "--priority", "99"})
	if err == nil {
		t.Error("update with invalid priority should fail")
	}
}

func TestCLI_Update_InvalidType(t *testing.T) {
	setupTestDir(t)
	runCLI([]string{"init"})

	createOut, _ := runCLI([]string{"create", "Task"})
	id := extractID(createOut)

	_, err := runCLI([]string{"update", id, "--type", "bogus"})
	if err == nil {
		t.Error("update with invalid type should fail")
	}
}

func TestCLI_Update_NoFlags(t *testing.T) {
	setupTestDir(t)
	runCLI([]string{"init"})

	createOut, _ := runCLI([]string{"create", "Task"})
	id := extractID(createOut)

	// Update with no flags should succeed (updates updated_at timestamp)
	out, err := runCLI([]string{"update", id})
	if err != nil {
		t.Errorf("update with no flags should succeed: %v", err)
	}
	if !strings.Contains(out, "Updated "+id) {
		t.Errorf("expected success message, got: %s", out)
	}
}

func TestCLI_List_Empty_JSON(t *testing.T) {
	setupTestDir(t)
	runCLI([]string{"init"})

	// List with --json on empty database should output nothing
	out, err := runCLI([]string{"list", "--json"})
	if err != nil {
		t.Fatalf("list --json on empty database should succeed: %v", err)
	}
	if out != "" {
		t.Errorf("expected empty output for empty database with --json, got: %s", out)
	}
}

func TestCLI_Ready_Empty_JSON(t *testing.T) {
	setupTestDir(t)
	runCLI([]string{"init"})

	// Ready with --json on empty database should output nothing
	out, err := runCLI([]string{"ready", "--json"})
	if err != nil {
		t.Fatalf("ready --json on empty database should succeed: %v", err)
	}
	if out != "" {
		t.Errorf("expected empty output for empty database with --json, got: %s", out)
	}
}

func TestCLI_Export_EmptyDatabase(t *testing.T) {
	setupTestDir(t)
	runCLI([]string{"init"})

	// Export with no issues
	out, err := runCLI([]string{"export"})
	if err != nil {
		t.Fatalf("export empty database should succeed: %v", err)
	}
	// Should output nothing (empty JSONL)
	if out != "" {
		t.Errorf("expected empty output for empty database, got: %s", out)
	}
}

func TestCLI_Update_DuplicateBlocker(t *testing.T) {
	setupTestDir(t)
	runCLI([]string{"init"})

	outA, _ := runCLI([]string{"create", "Task A"})
	outB, _ := runCLI([]string{"create", "Task B"})
	idA := extractID(outA)
	idB := extractID(outB)

	// Add blocker once
	runCLI([]string{"update", idB, "--blocked-by", idA})

	// Try to add same blocker again - should fail due to PK constraint
	_, err := runCLI([]string{"update", idB, "--blocked-by", idA})
	// Document behavior: duplicate blocker insertion fails silently or with error
	if err != nil {
		t.Logf("duplicate blocker rejected: %v", err)
	} else {
		t.Log("duplicate blocker accepted (idempotent)")
	}

	// Verify B is still blocked by A
	readyOut, _ := runCLI([]string{"ready"})
	if strings.Contains(readyOut, "Task B") {
		t.Errorf("B should still be blocked: %s", readyOut)
	}
}

func TestCLI_Create_WhitespaceOnlyTitle(t *testing.T) {
	setupTestDir(t)
	runCLI([]string{"init"})

	_, err := runCLI([]string{"create", "   "})
	if err == nil {
		t.Error("create with whitespace-only title should fail")
	}
}

func TestCLI_CycleDetection_ReadyBehavior(t *testing.T) {
	setupTestDir(t)
	runCLI([]string{"init"})

	outA, _ := runCLI([]string{"create", "Task A"})
	outB, _ := runCLI([]string{"create", "Task B"})
	idA := extractID(outA)
	idB := extractID(outB)

	// Create cycle: A blocks B, B blocks A
	runCLI([]string{"update", idA, "--blocked-by", idB})
	runCLI([]string{"update", idB, "--blocked-by", idA})

	// With a cycle, neither should be ready (both blocked)
	readyOut, _ := runCLI([]string{"ready"})

	// Document the behavior - this test verifies system doesn't crash
	// and documents whether cycles cause both tasks to be blocked
	if strings.Contains(readyOut, "Task A") || strings.Contains(readyOut, "Task B") {
		t.Logf("cycle allows tasks to be ready (unexpected): %s", readyOut)
	} else {
		t.Log("cycle correctly blocks both tasks")
	}
}

// P2 Test Coverage: Dependency types (parent-child, related)
// Note: These tests verify storage-level behavior since CLI doesn't expose these types

func TestStorage_ParentChildDependency(t *testing.T) {
	setupTestDir(t)

	store, err := NewStore(":memory:")
	if err != nil {
		t.Fatalf("failed to create store: %v", err)
	}
	defer store.Close()

	// Create parent and child issues
	parent := NewIssue("Parent Task")
	child := NewIssue("Child Task")
	store.CreateIssue(parent)
	store.CreateIssue(child)

	// Add parent-child dependency
	err = store.AddDependency(child.ID, parent.ID, DepParentChild)
	if err != nil {
		t.Fatalf("failed to add parent-child dep: %v", err)
	}

	// Verify dependency was created
	deps, err := store.GetDependencies(child.ID)
	if err != nil {
		t.Fatalf("failed to get deps: %v", err)
	}
	if len(deps) != 1 || deps[0].Type != DepParentChild {
		t.Errorf("expected parent-child dep, got: %v", deps)
	}

	// Parent-child deps should propagate blocking when parent is blocked
	// Create a blocker for parent
	blocker := NewIssue("Blocker")
	store.CreateIssue(blocker)
	store.AddDependency(parent.ID, blocker.ID, DepBlocks)

	// With parent blocked by blocker, child should also be blocked (via recursive CTE)
	ready, err := store.GetReadyWork()
	if err != nil {
		t.Fatalf("GetReadyWork failed: %v", err)
	}

	// Only blocker should be ready
	var readyIDs []string
	for _, r := range ready {
		readyIDs = append(readyIDs, r.ID)
	}

	foundBlocker := false
	for _, id := range readyIDs {
		if id == blocker.ID {
			foundBlocker = true
		}
		if id == parent.ID || id == child.ID {
			t.Errorf("parent or child should be blocked: %v", readyIDs)
		}
	}
	if !foundBlocker {
		t.Errorf("blocker should be ready: %v", readyIDs)
	}
}

func TestStorage_RelatedDependency(t *testing.T) {
	setupTestDir(t)

	store, err := NewStore(":memory:")
	if err != nil {
		t.Fatalf("failed to create store: %v", err)
	}
	defer store.Close()

	// Create two related issues
	issueA := NewIssue("Task A")
	issueB := NewIssue("Task B")
	store.CreateIssue(issueA)
	store.CreateIssue(issueB)

	// Add related dependency (non-blocking, informational)
	err = store.AddDependency(issueA.ID, issueB.ID, DepRelated)
	if err != nil {
		t.Fatalf("failed to add related dep: %v", err)
	}

	// Verify dependency was created
	deps, err := store.GetDependencies(issueA.ID)
	if err != nil {
		t.Fatalf("failed to get deps: %v", err)
	}
	if len(deps) != 1 || deps[0].Type != DepRelated {
		t.Errorf("expected related dep, got: %v", deps)
	}

	// Related deps should NOT block - both issues should be ready
	ready, err := store.GetReadyWork()
	if err != nil {
		t.Fatalf("GetReadyWork failed: %v", err)
	}

	if len(ready) != 2 {
		t.Errorf("both issues should be ready (related deps don't block): got %d ready", len(ready))
	}
}

func TestCLI_Ready_DeepChain(t *testing.T) {
	setupTestDir(t)
	runCLI([]string{"init"})

	// Create a chain of 15 tasks, each blocking the next
	// This tests SQLite recursive CTE depth handling
	const chainDepth = 15
	var ids []string

	for i := 0; i < chainDepth; i++ {
		out, err := runCLI([]string{"create", fmt.Sprintf("Chain Task %d", i)})
		if err != nil {
			t.Fatalf("failed to create task %d: %v", i, err)
		}
		ids = append(ids, extractID(out))
	}

	// Create blocking chain: task 0 blocks task 1, task 1 blocks task 2, etc.
	for i := 1; i < chainDepth; i++ {
		_, err := runCLI([]string{"update", ids[i], "--blocked-by", ids[i-1]})
		if err != nil {
			t.Fatalf("failed to add blocker for task %d: %v", i, err)
		}
	}

	// Only task 0 should be ready
	readyOut, err := runCLI([]string{"ready"})
	if err != nil {
		t.Fatalf("ready failed on deep chain: %v", err)
	}
	if !strings.Contains(readyOut, "Chain Task 0") {
		t.Errorf("task 0 should be ready: %s", readyOut)
	}
	if strings.Contains(readyOut, "Chain Task 14") {
		t.Errorf("task 14 should be blocked: %s", readyOut)
	}

	// Close all but the last, verify chain unblocks correctly
	for i := 0; i < chainDepth-1; i++ {
		runCLI([]string{"close", ids[i]})
	}

	// Now task 14 should be ready
	readyOut2, _ := runCLI([]string{"ready"})
	if !strings.Contains(readyOut2, "Chain Task 14") {
		t.Errorf("task 14 should be ready after chain closed: %s", readyOut2)
	}
}
