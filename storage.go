package beadslite

import (
	"database/sql"
	"errors"
	"time"

	_ "github.com/mattn/go-sqlite3"
)

// Store provides SQLite-backed storage for issues and dependencies.
type Store struct {
	db *sql.DB
}

// NewStore creates a new Store with the given database path.
// Use ":memory:" for an in-memory database.
func NewStore(dbPath string) (*Store, error) {
	db, err := sql.Open("sqlite3", dbPath)
	if err != nil {
		return nil, err
	}

	store := &Store{db: db}
	if err := store.initSchema(); err != nil {
		db.Close()
		return nil, err
	}

	return store, nil
}

// Close closes the database connection.
func (s *Store) Close() error {
	return s.db.Close()
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
		closed_at DATETIME
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

	CREATE TABLE IF NOT EXISTS labels (
		issue_id TEXT NOT NULL,
		label TEXT NOT NULL,
		PRIMARY KEY (issue_id, label),
		FOREIGN KEY (issue_id) REFERENCES issues(id)
	);

	CREATE INDEX IF NOT EXISTS idx_deps_type ON dependencies(type, depends_on_id);
	CREATE INDEX IF NOT EXISTS idx_issues_status ON issues(status);
	`
	_, err := s.db.Exec(schema)
	return err
}

// CreateIssue inserts a new issue into the database.
func (s *Store) CreateIssue(issue *Issue) error {
	if err := issue.Validate(); err != nil {
		return err
	}

	_, err := s.db.Exec(`
		INSERT INTO issues (id, title, description, status, priority, issue_type, created_at, updated_at)
		VALUES (?, ?, ?, ?, ?, ?, ?, ?)`,
		issue.ID, issue.Title, issue.Description, issue.Status, issue.Priority, issue.Type,
		issue.CreatedAt, issue.UpdatedAt)
	return err
}

// GetIssue retrieves an issue by ID.
func (s *Store) GetIssue(id string) (*Issue, error) {
	issue := &Issue{}
	err := s.db.QueryRow(`
		SELECT id, title, description, status, priority, issue_type, created_at, updated_at, closed_at
		FROM issues WHERE id = ?`, id).Scan(
		&issue.ID, &issue.Title, &issue.Description, &issue.Status, &issue.Priority,
		&issue.Type, &issue.CreatedAt, &issue.UpdatedAt, &issue.ClosedAt)

	if err == sql.ErrNoRows {
		return nil, errors.New("issue not found")
	}
	return issue, err
}

// UpdateIssue updates an existing issue.
func (s *Store) UpdateIssue(issue *Issue) error {
	if err := issue.Validate(); err != nil {
		return err
	}

	issue.UpdatedAt = time.Now()
	_, err := s.db.Exec(`
		UPDATE issues SET title = ?, description = ?, status = ?, priority = ?,
		issue_type = ?, updated_at = ?, closed_at = ?
		WHERE id = ?`,
		issue.Title, issue.Description, issue.Status, issue.Priority,
		issue.Type, issue.UpdatedAt, issue.ClosedAt, issue.ID)
	return err
}

// CloseIssue marks an issue as closed.
func (s *Store) CloseIssue(id string) error {
	now := time.Now()
	_, err := s.db.Exec(`
		UPDATE issues SET status = ?, updated_at = ?, closed_at = ?
		WHERE id = ?`, StatusClosed, now, now, id)
	return err
}

// ListIssues returns all issues.
func (s *Store) ListIssues() ([]*Issue, error) {
	rows, err := s.db.Query(`
		SELECT id, title, description, status, priority, issue_type, created_at, updated_at, closed_at
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
		       i.created_at, i.updated_at, i.closed_at
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
			&issue.Priority, &issue.Type, &issue.CreatedAt, &issue.UpdatedAt, &issue.ClosedAt); err != nil {
			return nil, err
		}
		issues = append(issues, issue)
	}
	return issues, rows.Err()
}
