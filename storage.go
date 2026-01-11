package beadslite

import (
	"database/sql"
	"errors"
	"fmt"
	"time"

	_ "github.com/ncruces/go-sqlite3/driver"
	_ "github.com/ncruces/go-sqlite3/embed"
)

// ErrIssueNotFound is returned when an issue does not exist in the database.
var ErrIssueNotFound = errors.New("issue not found")

// Store provides SQLite-backed storage for issues and dependencies.
type Store struct {
	db *sql.DB
}

// NewStore creates a new Store with the given database path.
// Use ":memory:" for an in-memory database.
func NewStore(dbPath string) (*Store, error) {
	db, err := sql.Open("sqlite3", dbPath)
	if err != nil {
		return nil, fmt.Errorf("open database %s: %w", dbPath, err)
	}

	store := &Store{db: db}
	if err := store.initSchema(); err != nil {
		db.Close()
		return nil, fmt.Errorf("init schema: %w", err)
	}

	return store, nil
}

// Close closes the database connection.
func (s *Store) Close() error {
	return s.db.Close()
}

// WithTransaction executes the given function within a database transaction.
// If fn returns an error, the transaction is rolled back. Otherwise, it is committed.
func (s *Store) WithTransaction(fn func() error) error {
	if _, err := s.db.Exec("BEGIN IMMEDIATE"); err != nil {
		return fmt.Errorf("begin transaction: %w", err)
	}

	if err := fn(); err != nil {
		s.db.Exec("ROLLBACK")
		return err
	}

	if _, err := s.db.Exec("COMMIT"); err != nil {
		s.db.Exec("ROLLBACK")
		return fmt.Errorf("commit transaction: %w", err)
	}
	return nil
}

func (s *Store) initSchema() error {
	schema := `
	CREATE TABLE IF NOT EXISTS issues (
		id TEXT PRIMARY KEY,
		title TEXT NOT NULL,
		description TEXT,
		status TEXT NOT NULL DEFAULT 'open',
		priority INTEGER NOT NULL DEFAULT 2,
		issue_type TEXT NOT NULL DEFAULT 'task',
		created_at DATETIME NOT NULL,
		updated_at DATETIME NOT NULL,
		closed_at DATETIME,
		resolution TEXT
	);

	CREATE TABLE IF NOT EXISTS dependencies (
		issue_id TEXT NOT NULL,
		depends_on_id TEXT NOT NULL,
		type TEXT NOT NULL DEFAULT 'blocks',
		created_at DATETIME NOT NULL,
		PRIMARY KEY (issue_id, depends_on_id, type),
		FOREIGN KEY (issue_id) REFERENCES issues(id),
		FOREIGN KEY (depends_on_id) REFERENCES issues(id)
	);

	CREATE INDEX IF NOT EXISTS idx_deps_type ON dependencies(type, depends_on_id);
	CREATE INDEX IF NOT EXISTS idx_issues_status ON issues(status);
	`
	if _, err := s.db.Exec(schema); err != nil {
		return fmt.Errorf("exec schema: %w", err)
	}
	return nil
}

// CreateIssue inserts a new issue into the database.
func (s *Store) CreateIssue(issue *Issue) error {
	if err := issue.Validate(); err != nil {
		return err
	}

	if _, err := s.db.Exec(`
		INSERT INTO issues (id, title, description, status, priority, issue_type, created_at, updated_at, closed_at, resolution)
		VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?)`,
		issue.ID, issue.Title, issue.Description, issue.Status, issue.Priority, issue.Type,
		issue.CreatedAt, issue.UpdatedAt, issue.ClosedAt, issue.Resolution); err != nil {
		return fmt.Errorf("insert issue: %w", err)
	}
	return nil
}

// GetIssue retrieves an issue by ID.
func (s *Store) GetIssue(id string) (*Issue, error) {
	issue := &Issue{}
	err := s.db.QueryRow(`
		SELECT id, title, description, status, priority, issue_type, created_at, updated_at, closed_at, COALESCE(resolution, '')
		FROM issues WHERE id = ?`, id).Scan(
		&issue.ID, &issue.Title, &issue.Description, &issue.Status, &issue.Priority,
		&issue.Type, &issue.CreatedAt, &issue.UpdatedAt, &issue.ClosedAt, &issue.Resolution)

	if err == sql.ErrNoRows {
		return nil, ErrIssueNotFound
	}
	return issue, err
}

// UpdateIssue updates an existing issue.
func (s *Store) UpdateIssue(issue *Issue) error {
	if err := issue.Validate(); err != nil {
		return err
	}

	issue.UpdatedAt = time.Now()
	if _, err := s.db.Exec(`
		UPDATE issues SET title = ?, description = ?, status = ?, priority = ?,
		issue_type = ?, updated_at = ?, closed_at = ?, resolution = ?
		WHERE id = ?`,
		issue.Title, issue.Description, issue.Status, issue.Priority,
		issue.Type, issue.UpdatedAt, issue.ClosedAt, issue.Resolution, issue.ID); err != nil {
		return fmt.Errorf("update issue: %w", err)
	}
	return nil
}

// CloseIssue marks an issue as closed with the given resolution.
func (s *Store) CloseIssue(id string, resolution Resolution) error {
	now := time.Now()
	if _, err := s.db.Exec(`
		UPDATE issues SET status = ?, updated_at = ?, closed_at = ?, resolution = ?
		WHERE id = ?`, StatusClosed, now, now, resolution, id); err != nil {
		return fmt.Errorf("close issue: %w", err)
	}
	return nil
}

// ListIssues returns all issues.
func (s *Store) ListIssues() ([]*Issue, error) {
	rows, err := s.db.Query(`
		SELECT id, title, description, status, priority, issue_type, created_at, updated_at, closed_at, COALESCE(resolution, '')
		FROM issues ORDER BY priority ASC, created_at ASC`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	return scanIssues(rows)
}

// AddDependency creates a dependency between two issues.
func (s *Store) AddDependency(issueID, dependsOnID string, depType DepType) error {
	dep := NewDependency(issueID, dependsOnID, depType)
	if err := dep.Validate(); err != nil {
		return err
	}

	_, err := s.db.Exec(`
		INSERT INTO dependencies (issue_id, depends_on_id, type, created_at)
		VALUES (?, ?, ?, ?)`,
		dep.IssueID, dep.DependsOnID, dep.Type, dep.CreatedAt)
	return err
}

// RemoveDependency removes a dependency.
func (s *Store) RemoveDependency(issueID, dependsOnID string, depType DepType) error {
	_, err := s.db.Exec(`
		DELETE FROM dependencies WHERE issue_id = ? AND depends_on_id = ? AND type = ?`,
		issueID, dependsOnID, depType)
	return err
}

// RemoveAllDependencies removes all dependencies where the issue is the dependent.
func (s *Store) RemoveAllDependencies(issueID string) error {
	_, err := s.db.Exec(`DELETE FROM dependencies WHERE issue_id = ?`, issueID)
	return err
}

// GetDependencies returns all dependencies for an issue.
func (s *Store) GetDependencies(issueID string) ([]*Dependency, error) {
	rows, err := s.db.Query(`
		SELECT issue_id, depends_on_id, type, created_at
		FROM dependencies WHERE issue_id = ?`, issueID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var deps []*Dependency
	for rows.Next() {
		dep := &Dependency{}
		if err := rows.Scan(&dep.IssueID, &dep.DependsOnID, &dep.Type, &dep.CreatedAt); err != nil {
			return nil, err
		}
		deps = append(deps, dep)
	}
	return deps, rows.Err()
}

// GetReadyWork returns issues that are open and not blocked.
// Uses recursive CTE to find directly blocked issues and transitively blocked children.
func (s *Store) GetReadyWork() ([]*Issue, error) {
	query := `
		SELECT i.id, i.title, i.description, i.status, i.priority, i.issue_type,
		       i.created_at, i.updated_at, i.closed_at, COALESCE(i.resolution, '')
		FROM issues i
		WHERE i.status IN ('open', 'in_progress')
		AND i.id NOT IN (
			WITH RECURSIVE blocked AS (
				-- Directly blocked: has 'blocks' dependency on non-closed issue
				SELECT DISTINCT d.issue_id
				FROM dependencies d
				JOIN issues blocker ON d.depends_on_id = blocker.id
				WHERE d.type = 'blocks'
				  AND blocker.status != 'closed'

				UNION

				-- Transitively blocked: parent is blocked (parent-child relationship)
				SELECT d.issue_id
				FROM blocked b
				JOIN dependencies d ON d.depends_on_id = b.issue_id
				WHERE d.type = 'parent-child'
			)
			SELECT issue_id FROM blocked
		)
		ORDER BY i.priority ASC, i.created_at ASC
	`

	rows, err := s.db.Query(query)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	return scanIssues(rows)
}

func scanIssues(rows *sql.Rows) ([]*Issue, error) {
	var issues []*Issue
	for rows.Next() {
		issue := &Issue{}
		if err := rows.Scan(&issue.ID, &issue.Title, &issue.Description, &issue.Status,
			&issue.Priority, &issue.Type, &issue.CreatedAt, &issue.UpdatedAt, &issue.ClosedAt, &issue.Resolution); err != nil {
			return nil, err
		}
		issues = append(issues, issue)
	}
	return issues, rows.Err()
}

// GetAllDependencies returns all dependencies in the database, keyed by issue_id.
// Used for efficient tree building without N+1 queries.
func (s *Store) GetAllDependencies() (map[string][]*Dependency, error) {
	rows, err := s.db.Query(`
		SELECT issue_id, depends_on_id, type, created_at
		FROM dependencies`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	result := make(map[string][]*Dependency)
	for rows.Next() {
		dep := &Dependency{}
		if err := rows.Scan(&dep.IssueID, &dep.DependsOnID, &dep.Type, &dep.CreatedAt); err != nil {
			return nil, err
		}
		result[dep.IssueID] = append(result[dep.IssueID], dep)
	}
	return result, rows.Err()
}

// DeleteIssue removes an issue and all its dependencies from the database.
func (s *Store) DeleteIssue(id string) error {
	tx, err := s.db.Begin()
	if err != nil {
		return err
	}
	defer tx.Rollback()

	// Delete dependencies where this issue is involved (either side)
	_, err = tx.Exec(`DELETE FROM dependencies WHERE issue_id = ? OR depends_on_id = ?`, id, id)
	if err != nil {
		return err
	}

	// Delete the issue itself
	result, err := tx.Exec(`DELETE FROM issues WHERE id = ?`, id)
	if err != nil {
		return err
	}

	rows, err := result.RowsAffected()
	if err != nil {
		return err
	}
	if rows == 0 {
		return errors.New("issue not found")
	}

	return tx.Commit()
}
