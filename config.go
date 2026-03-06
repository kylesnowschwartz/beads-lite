package beadslite

import (
	"encoding/json"
	"os"
	"path/filepath"
)

// Config holds beads-lite project configuration.
// All fields use pointers so absent values are distinguishable from
// explicit false. Defaults are applied by the accessor methods.
type Config struct {
	RequireSpecsForReview *bool `json:"require_specs_for_review,omitempty"`
}

// LoadConfig reads .beads-lite/config.json from the given root directory.
// Returns zero Config if the file is absent or malformed — config is
// optional, so a broken file should never prevent the CLI from working.
func LoadConfig(root string) Config {
	dir := ".beads-lite"
	if root != "" {
		dir = filepath.Join(root, ".beads-lite")
	}
	data, err := os.ReadFile(filepath.Join(dir, "config.json"))
	if err != nil {
		return Config{}
	}
	var cfg Config
	if err := json.Unmarshal(data, &cfg); err != nil {
		return Config{}
	}
	return cfg
}

// SpecsRequiredForReview returns whether all specs must be checked
// before a card can move to Review. Defaults to true when not configured.
func (c Config) SpecsRequiredForReview() bool {
	if c.RequireSpecsForReview == nil {
		return true
	}
	return *c.RequireSpecsForReview
}

// TransitionPolicy returns a TransitionPolicy derived from this config.
func (c Config) TransitionPolicy() TransitionPolicy {
	return TransitionPolicy{
		RequireSpecsForReview: c.SpecsRequiredForReview(),
	}
}
