package beadslite

import (
	"bufio"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"
	"sort"
	"time"
)

// IssueExport represents an issue with embedded dependencies for JSONL export.
// Uses a flat dependency structure for git-friendly diffs.
type IssueExport struct {
	ID           string             `json:"id"`
	Title        string             `json:"title"`
	Description  string             `json:"description,omitempty"`
	Status       Status             `json:"status"`
	Priority     int                `json:"priority"`
	Type         IssueType          `json:"issue_type"`
	CreatedAt    time.Time          `json:"created_at"`
	UpdatedAt    time.Time          `json:"updated_at"`
	ClosedAt     *time.Time         `json:"closed_at,omitempty"`
	Resolution   Resolution         `json:"resolution,omitempty"`
	Dependencies []DependencyExport `json:"dependencies"`
}

// DependencyExport represents a dependency relationship for JSONL export.
type DependencyExport struct {
	DependsOn string  `json:"depends_on"`
	Type      DepType `json:"type"`
}

// ImportStats tracks the results of an import operation.
type ImportStats struct {
	Created int
	Updated int
}

// toIssueExport converts an Issue and its dependencies to an IssueExport.
func toIssueExport(issue *Issue, deps []*Dependency) IssueExport {
	export := IssueExport{
		ID:           issue.ID,
		Title:        issue.Title,
		Description:  issue.Description,
		Status:       issue.Status,
		Priority:     issue.Priority,
		Type:         issue.Type,
		CreatedAt:    issue.CreatedAt,
		UpdatedAt:    issue.UpdatedAt,
		ClosedAt:     issue.ClosedAt,
		Resolution:   issue.Resolution,
		Dependencies: make([]DependencyExport, len(deps)),
	}
	for i, dep := range deps {
		export.Dependencies[i] = DependencyExport{
			DependsOn: dep.DependsOnID,
			Type:      dep.Type,
		}
	}
	return export
}

// WriteIssuesAsJSONL writes a slice of issues with their dependencies to a writer in JSONL format.
// This is the common implementation used by both export and list --json.
func WriteIssuesAsJSONL(issues []*Issue, allDeps map[string][]*Dependency, w io.Writer) error {
	encoder := json.NewEncoder(w)
	for _, issue := range issues {
		export := toIssueExport(issue, allDeps[issue.ID])
		if err := encoder.Encode(export); err != nil {
			return fmt.Errorf("encode issue %s: %w", issue.ID, err)
		}
	}
	return nil
}

// ExportToJSONL writes all issues to the writer in JSONL format.
// Issues are sorted by ID for deterministic output (git-friendly).
func ExportToJSONL(store *Store, w io.Writer) error {
	issues, err := store.ListIssues()
	if err != nil {
		return fmt.Errorf("list issues: %w", err)
	}

	// Batch-fetch all dependencies to avoid N+1 queries
	allDeps, err := store.GetAllDependencies()
	if err != nil {
		return fmt.Errorf("get all dependencies: %w", err)
	}

	// Sort by ID for deterministic output
	sort.Slice(issues, func(i, j int) bool {
		return issues[i].ID < issues[j].ID
	})

	return WriteIssuesAsJSONL(issues, allDeps, w)
}

// ExportToFile writes all issues to the specified file in JSONL format.
func ExportToFile(store *Store, path string) error {
	f, err := os.Create(path)
	if err != nil {
		return fmt.Errorf("create file: %w", err)
	}

	if err := ExportToJSONL(store, f); err != nil {
		f.Close()
		return err
	}

	return f.Close()
}

// ImportFromJSONL reads issues from the reader in JSONL format.
// Uses upsert semantics: updates existing issues, creates new ones.
// The entire import is wrapped in a transaction for consistency.
func ImportFromJSONL(store *Store, r io.Reader) (*ImportStats, error) {
	stats := &ImportStats{}

	// Pre-scan all lines to avoid transaction timeout during I/O
	var lines [][]byte
	scanner := bufio.NewScanner(r)
	for scanner.Scan() {
		line := scanner.Bytes()
		if len(line) > 0 {
			// Make a copy since scanner reuses buffer
			lineCopy := make([]byte, len(line))
			copy(lineCopy, line)
			lines = append(lines, lineCopy)
		}
	}
	if err := scanner.Err(); err != nil {
		return nil, fmt.Errorf("read error: %w", err)
	}

	// Process all lines within a transaction
	err := store.WithTransaction(func() error {
		for lineNum, line := range lines {
			var export IssueExport
			if err := json.Unmarshal(line, &export); err != nil {
				return fmt.Errorf("line %d: parse error: %w", lineNum+1, err)
			}

			// Check if issue exists
			existing, err := store.GetIssue(export.ID)
			if err != nil && !errors.Is(err, ErrIssueNotFound) {
				return fmt.Errorf("line %d: check existing: %w", lineNum+1, err)
			}

			issue := &Issue{
				ID:          export.ID,
				Title:       export.Title,
				Description: export.Description,
				Status:      export.Status,
				Priority:    export.Priority,
				Type:        export.Type,
				CreatedAt:   export.CreatedAt,
				UpdatedAt:   export.UpdatedAt,
				ClosedAt:    export.ClosedAt,
				Resolution:  export.Resolution,
			}

			if existing != nil {
				// Update existing issue
				if err := store.UpdateIssue(issue); err != nil {
					return fmt.Errorf("line %d: update issue: %w", lineNum+1, err)
				}
				stats.Updated++

				// Clear existing dependencies before re-adding
				if err := store.RemoveAllDependencies(issue.ID); err != nil {
					return fmt.Errorf("line %d: remove deps: %w", lineNum+1, err)
				}
			} else {
				// Create new issue
				if err := store.CreateIssue(issue); err != nil {
					return fmt.Errorf("line %d: create issue: %w", lineNum+1, err)
				}
				stats.Created++
			}

			// Add dependencies
			for _, dep := range export.Dependencies {
				if err := store.AddDependency(issue.ID, dep.DependsOn, dep.Type); err != nil {
					return fmt.Errorf("line %d: add dependency: %w", lineNum+1, err)
				}
			}
		}
		return nil
	})

	if err != nil {
		return nil, err
	}
	return stats, nil
}

// ImportFromFile reads issues from the specified file in JSONL format.
func ImportFromFile(store *Store, path string) (*ImportStats, error) {
	f, err := os.Open(path)
	if err != nil {
		return nil, fmt.Errorf("open file: %w", err)
	}
	defer f.Close()

	return ImportFromJSONL(store, f)
}
