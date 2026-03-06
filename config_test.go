package beadslite

import (
	"os"
	"path/filepath"
	"testing"
)

func boolPtr(b bool) *bool { return &b }

func TestLoadConfig_MissingFile(t *testing.T) {
	cfg := LoadConfig(t.TempDir())
	if !cfg.SpecsRequiredForReview() {
		t.Error("missing file should default to true")
	}
}

func TestLoadConfig_ExplicitTrue(t *testing.T) {
	root := t.TempDir()
	writeTestConfig(t, root, `{"require_specs_for_review": true}`)

	cfg := LoadConfig(root)
	if !cfg.SpecsRequiredForReview() {
		t.Error("explicit true should return true")
	}
}

func TestLoadConfig_ExplicitFalse(t *testing.T) {
	root := t.TempDir()
	writeTestConfig(t, root, `{"require_specs_for_review": false}`)

	cfg := LoadConfig(root)
	if cfg.SpecsRequiredForReview() {
		t.Error("explicit false should return false")
	}
}

func TestLoadConfig_MalformedJSON(t *testing.T) {
	root := t.TempDir()
	writeTestConfig(t, root, `not valid json`)

	cfg := LoadConfig(root)
	if !cfg.SpecsRequiredForReview() {
		t.Error("malformed JSON should default to true")
	}
}

func TestTransitionPolicy_FromConfig(t *testing.T) {
	tests := []struct {
		name string
		cfg  Config
		want bool
	}{
		{"nil defaults true", Config{}, true},
		{"explicit true", Config{RequireSpecsForReview: boolPtr(true)}, true},
		{"explicit false", Config{RequireSpecsForReview: boolPtr(false)}, false},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			policy := tt.cfg.TransitionPolicy()
			if policy.RequireSpecsForReview != tt.want {
				t.Errorf("TransitionPolicy().RequireSpecsForReview = %v, want %v",
					policy.RequireSpecsForReview, tt.want)
			}
		})
	}
}

func writeTestConfig(t *testing.T, root, content string) {
	t.Helper()
	dir := filepath.Join(root, ".beads-lite")
	if err := os.MkdirAll(dir, 0755); err != nil {
		t.Fatalf("MkdirAll: %v", err)
	}
	if err := os.WriteFile(filepath.Join(dir, "config.json"), []byte(content), 0644); err != nil {
		t.Fatalf("WriteFile: %v", err)
	}
}
