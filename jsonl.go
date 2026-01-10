package beadslite

import (
	"bufio"
	"encoding/json"
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

// ExportToJSONL writes all issues to the writer in JSONL format.
// Issues are sorted by ID for deterministic output (git-friendly).
func ExportToJSONL(store *Store, w io.Writer) error {
	issues, err := store.ListIssues()
	if err != nil {
		return fmt.Errorf("list issues: %w", err)
	}

	// Sort by ID for deterministic output
	sort.Slice(issues, func(i, j int) bool {
		return issues[i].ID < issues[j].ID
	})

	encoder := json.NewEncoder(w)
	for _, issue := range issues {
		// Get dependencies for this issue
		deps, err := store.GetDependencies(issue.ID)
		if err != nil {
			return fmt.Errorf("get dependencies for %s: %w", issue.ID, err)
		}

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
			Dependencies: make([]DependencyExport, len(deps)),
		}

		for i, dep := range deps {
			export.Dependencies[i] = DependencyExport{
				DependsOn: dep.DependsOnID,
				Type:      dep.Type,
			}
		}

		if err := encoder.Encode(export); err != nil {
			return fmt.Errorf("encode issue %s: %w", issue.ID, err)
		}
	}

	return nil
}

// ExportToFile writes all issues to the specified file in JSONL format.
func ExportToFile(store *Store, path string) error {
	f, err := os.Create(path)
	if err != nil {
		return fmt.Errorf("create file: %w", err)
	}
	defer f.Close()

	if err := ExportToJSONL(store, f); err != nil {
		return err
	}

	return f.Close()
}

// ImportFromJSONL reads issues from the reader in JSONL format.
// Uses upsert semantics: updates existing issues, creates new ones.
func ImportFromJSONL(store *Store, r io.Reader) (*ImportStats, error) {
	stats := &ImportStats{}
	scanner := bufio.NewScanner(r)
	lineNum := 0

	for scanner.Scan() {
		lineNum++
		line := scanner.Bytes()
		if len(line) == 0 {
			continue // skip empty lines
		}

		var export IssueExport
		if err := json.Unmarshal(line, &export); err != nil {
			return nil, fmt.Errorf("line %d: parse error: %w", lineNum, err)
		}

		// Check if issue exists
		existing, err := store.GetIssue(export.ID)
		if err != nil && err.Error() != "issue not found" {
			return nil, fmt.Errorf("line %d: check existing: %w", lineNum, err)
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
		}

		if existing != nil {
			// Update existing issue
			if err := store.UpdateIssue(issue); err != nil {
				return nil, fmt.Errorf("line %d: update issue: %w", lineNum, err)
			}
			stats.Updated++

			// Clear existing dependencies before re-adding
			existingDeps, err := store.GetDependencies(issue.ID)
			if err != nil {
				return nil, fmt.Errorf("line %d: get deps: %w", lineNum, err)
			}
			for _, dep := range existingDeps {
				if err := store.RemoveDependency(dep.IssueID, dep.DependsOnID, dep.Type); err != nil {
					return nil, fmt.Errorf("line %d: remove dep: %w", lineNum, err)
				}
			}
		} else {
			// Create new issue
			if err := store.CreateIssue(issue); err != nil {
				return nil, fmt.Errorf("line %d: create issue: %w", lineNum, err)
			}
			stats.Created++
		}

		// Add dependencies
		for _, dep := range export.Dependencies {
			if err := store.AddDependency(issue.ID, dep.DependsOn, dep.Type); err != nil {
				return nil, fmt.Errorf("line %d: add dependency: %w", lineNum, err)
			}
		}
	}

	if err := scanner.Err(); err != nil {
		return nil, fmt.Errorf("read error: %w", err)
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
