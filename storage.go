package beadslite

import (
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"time"

	_ "github.com/ncruces/go-sqlite3/driver"
	_ "github.com/ncruces/go-sqlite3/embed"
)

// ErrIssueNotFound is returned when an issue does not exist in the database.
var ErrIssueNotFound = errors.New("issue not found")

// AutoCloseResult reports that a parent epic was automatically closed
// because all of its children reached status done.
type AutoCloseResult struct {
	ID    string
	Title string
}

// Storage is the interface that wraps all persistence operations for issues and dependencies.
// *Store is the concrete SQLite implementation; callers that only need the interface
// can depend on Storage to allow alternative implementations in tests or other backends.
type Storage interface {
	// Lifecycle
	Close() error
	WithTransaction(fn func() error) error

	// Issue CRUD
	CreateIssue(issue *Issue) error
	GetIssue(id string) (*Issue, error)
	UpdateIssue(issue *Issue) error
	ListIssues() ([]*Issue, error)
	DeleteIssue(id string) error

	// Issue workflow
	CloseIssue(id string, resolution Resolution) (*AutoCloseResult, error)
	ClaimIssue(id, agent string) (bool, error)
	UnclaimIssue(id string) error

	// Agent state
	SetAgentState(issueID string, state AgentState, activity *time.Time) error
	GetAgentsByState(state AgentState) ([]*Issue, error)

	// Role-scoped assignments (worker / reviewer / oracle).
	// ClaimRole atomically reserves a (issue, role) slot for an agent.
	// Returns true if the claim succeeded, false if the slot is already
	// held in a live state (running / stuck) by another agent.
	ClaimRole(issueID string, role Role, agent string) (bool, error)
	UnclaimRole(issueID string, role Role) error
	SetRoleState(issueID string, role Role, state AgentState, activity *time.Time) error
	GetAssignments(issueID string) ([]Assignment, error)
	GetAllAssignments() (map[string][]Assignment, error)

	// Dependencies
	AddDependency(issueID, dependsOnID string, depType DepType) error
	RemoveDependency(issueID, dependsOnID string, depType DepType) error
	RemoveAllDependencies(issueID string) error
	GetDependencies(issueID string) ([]*Dependency, error)
	GetAllDependencies() (map[string][]*Dependency, error)

	// Scheduling
	GetReadyWork() ([]*Issue, error)
}

// Compile-time check: *Store must implement Storage.
var _ Storage = (*Store)(nil)

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

	// WAL mode: concurrent readers + single writer (no reader blocking).
	// busy_timeout: retry writes for 5s instead of immediate SQLITE_BUSY.
	db.Exec("PRAGMA journal_mode=wal")
	db.Exec("PRAGMA busy_timeout=5000")

	store := &Store{db: db}
	if err := store.initSchema(); err != nil {
		db.Close()
		return nil, fmt.Errorf("init schema: %w", err)
	}
	if err := store.migrateSchema(); err != nil {
		db.Close()
		return nil, fmt.Errorf("migrate schema: %w", err)
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
		status TEXT NOT NULL DEFAULT 'backlog',
		priority INTEGER NOT NULL DEFAULT 2,
		issue_type TEXT NOT NULL DEFAULT 'task',
		assigned_to TEXT,
		created_at DATETIME NOT NULL,
		updated_at DATETIME NOT NULL,
		closed_at DATETIME,
		resolution TEXT,
		agent_state TEXT,
		last_activity DATETIME,
		specifications TEXT
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

	CREATE TABLE IF NOT EXISTS assignments (
		issue_id TEXT NOT NULL,
		role TEXT NOT NULL,
		agent TEXT NOT NULL,
		state TEXT NOT NULL DEFAULT 'idle',
		started_at DATETIME NOT NULL,
		last_activity DATETIME,
		PRIMARY KEY (issue_id, role),
		FOREIGN KEY (issue_id) REFERENCES issues(id) ON DELETE CASCADE
	);

	CREATE INDEX IF NOT EXISTS idx_assignments_state ON assignments(state);
	`
	if _, err := s.db.Exec(schema); err != nil {
		return fmt.Errorf("exec schema: %w", err)
	}
	return nil
}

// migrateSchema adds columns that may not exist in older databases.
// Each ALTER TABLE is idempotent: "duplicate column name" errors are ignored.
// Status migration: open→todo, in_progress→doing, closed→done.
func (s *Store) migrateSchema() error {
	s.db.Exec("ALTER TABLE issues ADD COLUMN assigned_to TEXT")

	// Migrate legacy 3-status values to 5-column kanban statuses.
	// Each UPDATE is idempotent: if no rows match, nothing happens.
	s.db.Exec("UPDATE issues SET status = 'todo' WHERE status = 'open'")
	s.db.Exec("UPDATE issues SET status = 'doing' WHERE status = 'in_progress'")
	s.db.Exec("UPDATE issues SET status = 'done' WHERE status = 'closed'")

	// Add agent state tracking columns. Errors are ignored because SQLite
	// returns "duplicate column name" if the column already exists.
	s.db.Exec("ALTER TABLE issues ADD COLUMN agent_state TEXT")
	s.db.Exec("ALTER TABLE issues ADD COLUMN last_activity DATETIME")
	s.db.Exec("ALTER TABLE issues ADD COLUMN specifications TEXT")

	// Backfill assignments table from legacy issues columns. Idempotent: the
	// PRIMARY KEY (issue_id, role) means re-running this on a database that
	// already has worker rows is a no-op via INSERT OR IGNORE. The first
	// invocation against an upgraded database copies any actively-claimed
	// issue into a worker-role assignment so existing claims remain visible.
	s.db.Exec(`
		INSERT OR IGNORE INTO assignments (issue_id, role, agent, state, started_at, last_activity)
		SELECT id, 'worker', assigned_to,
		       COALESCE(NULLIF(agent_state, ''), 'idle'),
		       updated_at, last_activity
		FROM issues
		WHERE assigned_to IS NOT NULL AND assigned_to != ''
	`)

	return nil
}

// CreateIssue inserts a new issue into the database.
func (s *Store) CreateIssue(issue *Issue) error {
	if err := issue.Validate(); err != nil {
		return err
	}

	if _, err := s.db.Exec(`
		INSERT INTO issues (id, title, description, status, priority, issue_type, assigned_to, created_at, updated_at, closed_at, resolution, agent_state, last_activity, specifications)
		VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)`,
		issue.ID, issue.Title, issue.Description, issue.Status, issue.Priority, issue.Type,
		nilIfEmpty(issue.AssignedTo), issue.CreatedAt, issue.UpdatedAt, issue.ClosedAt, issue.Resolution,
		nilIfEmptyAgentState(issue.AgentState), issue.LastActivity, specsToJSON(issue.Specifications)); err != nil {
		return fmt.Errorf("insert issue: %w", err)
	}
	return nil
}

// GetIssue retrieves an issue by ID.
func (s *Store) GetIssue(id string) (*Issue, error) {
	issue := &Issue{}
	var specsJSON string
	err := s.db.QueryRow(`
		SELECT id, title, description, status, priority, issue_type,
		       COALESCE(assigned_to, ''), created_at, updated_at, closed_at, COALESCE(resolution, ''),
		       COALESCE(agent_state, ''), last_activity, COALESCE(specifications, '')
		FROM issues WHERE id = ?`, id).Scan(
		&issue.ID, &issue.Title, &issue.Description, &issue.Status, &issue.Priority,
		&issue.Type, &issue.AssignedTo, &issue.CreatedAt, &issue.UpdatedAt, &issue.ClosedAt, &issue.Resolution,
		&issue.AgentState, &issue.LastActivity, &specsJSON)

	if err == sql.ErrNoRows {
		return nil, ErrIssueNotFound
	}
	issue.Specifications = specsFromJSON(specsJSON)
	if err == nil {
		if assigns, aerr := s.GetAssignments(id); aerr == nil {
			issue.Assignments = assigns
		}
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
		issue_type = ?, assigned_to = ?, updated_at = ?, closed_at = ?, resolution = ?,
		agent_state = ?, last_activity = ?, specifications = ?
		WHERE id = ?`,
		issue.Title, issue.Description, issue.Status, issue.Priority,
		issue.Type, nilIfEmpty(issue.AssignedTo), issue.UpdatedAt, issue.ClosedAt, issue.Resolution,
		nilIfEmptyAgentState(issue.AgentState), issue.LastActivity, specsToJSON(issue.Specifications), issue.ID); err != nil {
		return fmt.Errorf("update issue: %w", err)
	}
	return nil
}

// CloseIssue marks an issue as closed with the given resolution.
// After closing, it checks whether the issue has a parent epic whose children
// are now all done. If so, it auto-closes the parent with resolution "done" and
// returns an AutoCloseResult. Auto-close does not cascade to grandparents.
func (s *Store) CloseIssue(id string, resolution Resolution) (*AutoCloseResult, error) {
	now := time.Now()
	if _, err := s.db.Exec(`
		UPDATE issues SET status = ?, assigned_to = NULL, updated_at = ?, closed_at = ?, resolution = ?
		WHERE id = ?`, StatusDone, now, now, resolution, id); err != nil {
		return nil, fmt.Errorf("close issue: %w", err)
	}

	result, err := s.tryAutoCloseParent(id, now)
	if err != nil {
		// Best-effort: log and continue (spec 6)
		fmt.Fprintf(os.Stderr, "auto-close check failed: %v\n", err)
		return nil, nil
	}
	return result, nil
}

// tryAutoCloseParent checks whether the closed issue's parent epic should be
// auto-closed. Returns nil if there's no parent or open children remain.
func (s *Store) tryAutoCloseParent(childID string, now time.Time) (*AutoCloseResult, error) {
	// Find this issue's parent epic via DepParent dependency
	var parentID string
	err := s.db.QueryRow(`
		SELECT depends_on_id FROM dependencies
		WHERE issue_id = ? AND type = ?`, childID, DepParent).Scan(&parentID)
	if err != nil {
		if err == sql.ErrNoRows {
			return nil, nil // no parent, nothing to do
		}
		return nil, fmt.Errorf("find parent: %w", err)
	}

	// Count children of the parent that are not yet done
	var openCount int
	err = s.db.QueryRow(`
		SELECT COUNT(*) FROM dependencies d
		JOIN issues i ON d.issue_id = i.id
		WHERE d.depends_on_id = ? AND d.type = ? AND i.status != ?`,
		parentID, DepParent, StatusDone).Scan(&openCount)
	if err != nil {
		return nil, fmt.Errorf("count open children: %w", err)
	}

	if openCount > 0 {
		return nil, nil
	}

	// All children done — close the parent epic
	var title string
	err = s.db.QueryRow(`SELECT title FROM issues WHERE id = ?`, parentID).Scan(&title)
	if err != nil {
		return nil, fmt.Errorf("get parent title: %w", err)
	}

	if _, err := s.db.Exec(`
		UPDATE issues SET status = ?, assigned_to = NULL, updated_at = ?, closed_at = ?, resolution = ?
		WHERE id = ?`, StatusDone, now, now, ResolutionDone, parentID); err != nil {
		return nil, fmt.Errorf("auto-close parent: %w", err)
	}

	return &AutoCloseResult{ID: parentID, Title: title}, nil
}

// ListIssues returns all issues with their assignments populated.
func (s *Store) ListIssues() ([]*Issue, error) {
	rows, err := s.db.Query(`
		SELECT id, title, description, status, priority, issue_type,
		       COALESCE(assigned_to, ''), created_at, updated_at, closed_at, COALESCE(resolution, ''),
		       COALESCE(agent_state, ''), last_activity, COALESCE(specifications, '')
		FROM issues ORDER BY priority ASC, created_at ASC`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	issues, err := scanIssues(rows)
	if err != nil {
		return nil, err
	}
	s.populateAssignments(issues)
	return issues, nil
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
func (s *Store) GetReadyWork() ([]*Issue, error) {
	query := `
		SELECT i.id, i.title, i.description, i.status, i.priority, i.issue_type,
		       COALESCE(i.assigned_to, ''), i.created_at, i.updated_at, i.closed_at, COALESCE(i.resolution, ''),
		       COALESCE(i.agent_state, ''), i.last_activity, COALESCE(i.specifications, '')
		FROM issues i
		WHERE i.status IN ('todo', 'doing', 'review')
		AND i.id NOT IN (
			SELECT DISTINCT d.issue_id
			FROM dependencies d
			JOIN issues blocker ON d.depends_on_id = blocker.id
			WHERE d.type = 'blocks'
			  AND blocker.status != 'done'
		)
		ORDER BY i.priority ASC, i.created_at ASC
	`

	rows, err := s.db.Query(query)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	issues, err := scanIssues(rows)
	if err != nil {
		return nil, err
	}
	s.populateAssignments(issues)
	return issues, nil
}

func scanIssues(rows *sql.Rows) ([]*Issue, error) {
	var issues []*Issue
	for rows.Next() {
		issue := &Issue{}
		var specsJSON string
		if err := rows.Scan(&issue.ID, &issue.Title, &issue.Description, &issue.Status,
			&issue.Priority, &issue.Type, &issue.AssignedTo,
			&issue.CreatedAt, &issue.UpdatedAt, &issue.ClosedAt, &issue.Resolution,
			&issue.AgentState, &issue.LastActivity, &specsJSON); err != nil {
			return nil, err
		}
		issue.Specifications = specsFromJSON(specsJSON)
		issues = append(issues, issue)
	}
	return issues, rows.Err()
}

// populateAssignments attaches each issue's assignments to its Assignments
// slice. Single batch query — N+1 free. Best-effort: a query failure leaves
// the slices empty rather than aborting, since assignment data is metadata,
// not authoritative for issue identity.
func (s *Store) populateAssignments(issues []*Issue) {
	if len(issues) == 0 {
		return
	}
	all, err := s.GetAllAssignments()
	if err != nil {
		return
	}
	for _, issue := range issues {
		if rows, ok := all[issue.ID]; ok {
			issue.Assignments = rows
		}
	}
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

// ClaimIssue atomically assigns an issue to a worker. Equivalent to
// ClaimRole(id, RoleWorker, agent) plus the legacy side-effect of moving
// the issue to status='doing' and writing assigned_to. Returns true if the
// claim succeeded, false if the worker slot is already held in a live state.
// Uses BEGIN IMMEDIATE to serialize concurrent claim attempts.
func (s *Store) ClaimIssue(id, agent string) (bool, error) {
	var claimed bool
	err := s.WithTransaction(func() error {
		ok, err := s.claimRoleTx(id, RoleWorker, agent)
		if err != nil {
			return err
		}
		if !ok {
			claimed = false
			return nil
		}
		// Mirror to legacy columns so existing readers stay correct.
		if _, err := s.db.Exec(`
			UPDATE issues SET assigned_to = ?, status = 'doing', updated_at = ?
			WHERE id = ?`, agent, time.Now(), id); err != nil {
			return err
		}
		claimed = true
		return nil
	})
	return claimed, err
}

// UnclaimIssue removes the worker assignment from an issue and resets it
// to todo. Mirrors UnclaimRole(id, RoleWorker) plus the legacy reset.
func (s *Store) UnclaimIssue(id string) error {
	return s.WithTransaction(func() error {
		if _, err := s.db.Exec(`
			DELETE FROM assignments WHERE issue_id = ? AND role = ?`,
			id, RoleWorker); err != nil {
			return err
		}
		_, err := s.db.Exec(`
			UPDATE issues SET assigned_to = NULL, status = 'todo', updated_at = ?
			WHERE id = ?`, time.Now(), id)
		return err
	})
}

// claimRoleTx is the inner-transaction body of ClaimRole and ClaimIssue.
// Caller must already be inside WithTransaction. The PRIMARY KEY on
// (issue_id, role) plus a state-aware upsert is the atomicity gate: a slot
// can be reclaimed only when its previous occupant is idle/done/dead.
func (s *Store) claimRoleTx(issueID string, role Role, agent string) (bool, error) {
	if !role.Valid() {
		return false, fmt.Errorf("invalid role: %q (valid: worker, reviewer, oracle)", role)
	}
	var existingState string
	err := s.db.QueryRow(
		"SELECT state FROM assignments WHERE issue_id = ? AND role = ?",
		issueID, string(role),
	).Scan(&existingState)
	if err != nil && err != sql.ErrNoRows {
		return false, err
	}
	// Live states block reclaim by another agent.
	if err == nil && (existingState == string(AgentStateRunning) || existingState == string(AgentStateStuck)) {
		return false, nil
	}
	now := time.Now()
	if _, err := s.db.Exec(`
		INSERT INTO assignments (issue_id, role, agent, state, started_at, last_activity)
		VALUES (?, ?, ?, ?, ?, ?)
		ON CONFLICT(issue_id, role) DO UPDATE SET
			agent = excluded.agent,
			state = excluded.state,
			started_at = excluded.started_at,
			last_activity = excluded.last_activity`,
		issueID, string(role), agent, string(AgentStateRunning), now, now); err != nil {
		return false, fmt.Errorf("insert assignment: %w", err)
	}
	return true, nil
}

// ClaimRole atomically reserves a (issue, role) slot for an agent. Returns
// true on success, false if the slot is held in a live state by someone else.
// For RoleWorker, prefer ClaimIssue — it also updates the legacy columns.
func (s *Store) ClaimRole(issueID string, role Role, agent string) (bool, error) {
	var claimed bool
	err := s.WithTransaction(func() error {
		ok, err := s.claimRoleTx(issueID, role, agent)
		if err != nil {
			return err
		}
		claimed = ok
		return nil
	})
	return claimed, err
}

// UnclaimRole removes a (issue, role) assignment outright. For RoleWorker,
// also clears the legacy assigned_to column and resets status to todo so the
// behaviour matches UnclaimIssue.
func (s *Store) UnclaimRole(issueID string, role Role) error {
	if !role.Valid() {
		return fmt.Errorf("invalid role: %q (valid: worker, reviewer, oracle)", role)
	}
	if role == RoleWorker {
		return s.UnclaimIssue(issueID)
	}
	_, err := s.db.Exec(`
		DELETE FROM assignments WHERE issue_id = ? AND role = ?`,
		issueID, string(role))
	return err
}

// SetRoleState updates the state and last_activity of an existing (issue, role)
// assignment. For RoleWorker, also mirrors to the legacy issues.agent_state
// column so consumers reading either source see the same value.
func (s *Store) SetRoleState(issueID string, role Role, state AgentState, activity *time.Time) error {
	if !role.Valid() {
		return fmt.Errorf("invalid role: %q (valid: worker, reviewer, oracle)", role)
	}
	if !state.Valid() {
		return fmt.Errorf("invalid agent state: %q (valid: idle, running, stuck, done, dead)", state)
	}
	return s.WithTransaction(func() error {
		if _, err := s.db.Exec(`
			UPDATE assignments SET state = ?, last_activity = ?
			WHERE issue_id = ? AND role = ?`,
			string(state), activity, issueID, string(role)); err != nil {
			return fmt.Errorf("update assignment state: %w", err)
		}
		if role == RoleWorker {
			if _, err := s.db.Exec(`
				UPDATE issues SET agent_state = ?, last_activity = ?, updated_at = ?
				WHERE id = ?`,
				nilIfEmptyAgentState(state), activity, time.Now(), issueID); err != nil {
				return fmt.Errorf("mirror agent_state: %w", err)
			}
		}
		return nil
	})
}

// GetAssignments returns every assignment row for one issue, ordered by role.
func (s *Store) GetAssignments(issueID string) ([]Assignment, error) {
	rows, err := s.db.Query(`
		SELECT issue_id, role, agent, state, started_at, last_activity
		FROM assignments WHERE issue_id = ?
		ORDER BY role ASC`, issueID)
	if err != nil {
		return nil, fmt.Errorf("query assignments: %w", err)
	}
	defer rows.Close()
	return scanAssignments(rows)
}

// GetAllAssignments returns every assignment row in the database, keyed by
// issue ID. Mirrors GetAllDependencies — single query, batch population.
func (s *Store) GetAllAssignments() (map[string][]Assignment, error) {
	rows, err := s.db.Query(`
		SELECT issue_id, role, agent, state, started_at, last_activity
		FROM assignments
		ORDER BY issue_id ASC, role ASC`)
	if err != nil {
		return nil, fmt.Errorf("query all assignments: %w", err)
	}
	defer rows.Close()
	all, err := scanAssignments(rows)
	if err != nil {
		return nil, err
	}
	out := make(map[string][]Assignment)
	for _, a := range all {
		out[a.IssueID] = append(out[a.IssueID], a)
	}
	return out, nil
}

func scanAssignments(rows *sql.Rows) ([]Assignment, error) {
	var out []Assignment
	for rows.Next() {
		var a Assignment
		if err := rows.Scan(&a.IssueID, &a.Role, &a.Agent, &a.State, &a.StartedAt, &a.LastActivity); err != nil {
			return nil, err
		}
		out = append(out, a)
	}
	return out, rows.Err()
}

// nilIfEmpty returns nil for empty strings, allowing SQLite to store NULL
// instead of empty string. This keeps assigned_to semantically clean.
func nilIfEmpty(s string) any {
	if s == "" {
		return nil
	}
	return s
}

// nilIfEmptyAgentState returns nil for the zero AgentState value so SQLite
// stores NULL rather than an empty string.
func nilIfEmptyAgentState(a AgentState) any {
	if a == "" {
		return nil
	}
	return string(a)
}

// specsToJSON marshals specs to a JSON string for storage.
// Returns nil (SQL NULL) when the slice is empty, keeping the column clean.
func specsToJSON(specs []Spec) any {
	if len(specs) == 0 {
		return nil
	}
	b, err := json.Marshal(specs)
	if err != nil {
		return nil
	}
	return string(b)
}

// specsFromJSON unmarshals a JSON string into a Spec slice.
// Returns nil for empty/null input — callers never see an error.
func specsFromJSON(s string) []Spec {
	if s == "" {
		return nil
	}
	var specs []Spec
	if err := json.Unmarshal([]byte(s), &specs); err != nil {
		return nil
	}
	return specs
}

// SetAgentState updates the worker-role agent_state and last_activity for an
// issue. Mirrors to both the legacy issues.agent_state column and the
// assignments table so either reader sees consistent state. Equivalent to
// SetRoleState(issueID, RoleWorker, state, activity).
// activity may be nil to clear the timestamp; pass time.Now() for normal updates.
func (s *Store) SetAgentState(issueID string, state AgentState, activity *time.Time) error {
	return s.SetRoleState(issueID, RoleWorker, state, activity)
}

// GetAgentsByState returns all issues with the given agent_state.
func (s *Store) GetAgentsByState(state AgentState) ([]*Issue, error) {
	rows, err := s.db.Query(`
		SELECT id, title, description, status, priority, issue_type,
		       COALESCE(assigned_to, ''), created_at, updated_at, closed_at, COALESCE(resolution, ''),
		       COALESCE(agent_state, ''), last_activity, COALESCE(specifications, '')
		FROM issues
		WHERE agent_state = ?
		ORDER BY priority ASC, created_at ASC`, string(state))
	if err != nil {
		return nil, fmt.Errorf("get agents by state: %w", err)
	}
	defer rows.Close()

	issues, err := scanIssues(rows)
	if err != nil {
		return nil, err
	}
	s.populateAssignments(issues)
	return issues, nil
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
