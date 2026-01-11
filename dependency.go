package beadslite

import (
	"errors"
	"fmt"
	"time"
)

// DepType represents the type of dependency between issues.
type DepType string

const (
	// DepBlocks indicates the depended-on issue must close before this issue is ready.
	DepBlocks DepType = "blocks"
)

// Valid returns true if the dependency type is a known valid type.
func (d DepType) Valid() bool {
	return d == DepBlocks
}

// Dependency represents an edge in the issue dependency graph.
type Dependency struct {
	IssueID     string    `json:"issue_id"`      // The issue that has the dependency
	DependsOnID string    `json:"depends_on_id"` // The issue being depended on
	Type        DepType   `json:"type"`
	CreatedAt   time.Time `json:"created_at"`
}

// NewDependency creates a new dependency with the current timestamp.
func NewDependency(issueID, dependsOnID string, depType DepType) *Dependency {
	return &Dependency{
		IssueID:     issueID,
		DependsOnID: dependsOnID,
		Type:        depType,
		CreatedAt:   time.Now(),
	}
}

// Validate checks if the dependency has valid field values.
func (d *Dependency) Validate() error {
	if d.IssueID == "" {
		return errors.New("issue_id cannot be empty")
	}
	if d.DependsOnID == "" {
		return errors.New("depends_on_id cannot be empty")
	}
	if !d.Type.Valid() {
		return fmt.Errorf("invalid dependency type: %q", d.Type)
	}
	if d.IssueID == d.DependsOnID {
		return errors.New("issue cannot depend on itself")
	}
	return nil
}
