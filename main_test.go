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

	// Check .beads directory created
	if _, statErr := os.Stat(".beads"); os.IsNotExist(statErr) {
		t.Error(".beads directory not created")
	}

	// Check database file created
	if _, statErr := os.Stat(".beads/beads.db"); os.IsNotExist(statErr) {
		t.Error(".beads/beads.db not created")
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
	return filepath.Join(".beads", "beads.db")
}
