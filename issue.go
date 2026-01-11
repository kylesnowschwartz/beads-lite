package beadslite

import (
	"crypto/sha256"
	"errors"
	"fmt"
	"math/big"
	"strings"
	"time"
)

// Status represents the state of an issue.
type Status string

const (
	StatusOpen       Status = "open"
	StatusInProgress Status = "in_progress"
	StatusClosed     Status = "closed"
)

// Valid returns true if the status is a known valid status.
func (s Status) Valid() bool {
	switch s {
	case StatusOpen, StatusInProgress, StatusClosed:
		return true
	default:
		return false
	}
}

// IssueType represents the category of an issue.
type IssueType string

const (
	IssueTypeTask    IssueType = "task"
	IssueTypeBug     IssueType = "bug"
	IssueTypeFeature IssueType = "feature"
	IssueTypeEpic    IssueType = "epic"
)

// Valid returns true if the issue type is a known valid type.
func (t IssueType) Valid() bool {
	switch t {
	case IssueTypeTask, IssueTypeBug, IssueTypeFeature, IssueTypeEpic:
		return true
	default:
		return false
	}
}

// Resolution represents why an issue was closed.
type Resolution string

const (
	ResolutionDone      Resolution = "done"      // work completed (default)
	ResolutionWontfix   Resolution = "wontfix"   // intentionally rejected
	ResolutionDuplicate Resolution = "duplicate" // duplicate of another issue
)

// Valid returns true if the resolution is a known valid resolution.
// Empty string is valid (treated as "done" for backwards compatibility).
func (r Resolution) Valid() bool {
	switch r {
	case "", ResolutionDone, ResolutionWontfix, ResolutionDuplicate:
		return true
	default:
		return false
	}
}

// Issue represents a trackable work item with dependencies.
type Issue struct {
	ID          string     `json:"id"`
	Title       string     `json:"title"`
	Description string     `json:"description,omitempty"`
	Status      Status     `json:"status"`
	Priority    int        `json:"priority"` // 0-4 (P0 = critical, P4 = lowest)
	Type        IssueType  `json:"issue_type"`
	CreatedAt   time.Time  `json:"created_at"`
	UpdatedAt   time.Time  `json:"updated_at"`
	ClosedAt    *time.Time `json:"closed_at,omitempty"`
	Resolution  Resolution `json:"resolution,omitempty"`
}

// NewIssue creates a new issue with a hash-based ID and sensible defaults.
func NewIssue(title string) *Issue {
	now := time.Now()
	id := generateHashID("bl", title, "", now, 4)

	return &Issue{
		ID:        id,
		Title:     title,
		Status:    StatusOpen,
		Priority:  2, // Medium priority by default
		Type:      IssueTypeTask,
		CreatedAt: now,
		UpdatedAt: now,
	}
}

// Validate checks if the issue has valid field values.
func (i *Issue) Validate() error {
	if strings.TrimSpace(i.Title) == "" {
		return errors.New("title cannot be empty")
	}
	if !i.Status.Valid() {
		return fmt.Errorf("invalid status: %q", i.Status)
	}
	if !i.Type.Valid() {
		return fmt.Errorf("invalid issue type: %q", i.Type)
	}
	if i.Priority < 0 || i.Priority > 4 {
		return fmt.Errorf("priority must be 0-4, got %d", i.Priority)
	}
	if !i.Resolution.Valid() {
		return fmt.Errorf("invalid resolution: %q", i.Resolution)
	}
	return nil
}

// base36Alphabet is the character set for base36 encoding (0-9, a-z).
const base36Alphabet = "0123456789abcdefghijklmnopqrstuvwxyz"

// generateHashID creates a hash-based ID for an issue.
// Uses SHA256 + base36 encoding for compact, collision-resistant IDs.
func generateHashID(prefix, title, description string, timestamp time.Time, length int) string {
	// Include timestamp nanoseconds for uniqueness
	content := fmt.Sprintf("%s|%s|%d", title, description, timestamp.UnixNano())

	// Hash the content
	hash := sha256.Sum256([]byte(content))

	// Use enough bytes for the desired length
	numBytes := 3 // 3 bytes gives us ~4.6 base36 chars
	if length > 4 {
		numBytes = 4
	}

	shortHash := encodeBase36(hash[:numBytes], length)
	return fmt.Sprintf("%s-%s", prefix, shortHash)
}

// encodeBase36 converts a byte slice to a base36 string of specified length.
func encodeBase36(data []byte, length int) string {
	// Convert bytes to big integer
	num := new(big.Int).SetBytes(data)

	// Convert to base36
	var result strings.Builder
	base := big.NewInt(36)
	zero := big.NewInt(0)
	mod := new(big.Int)

	// Build the string in reverse
	chars := make([]byte, 0, length)
	for num.Cmp(zero) > 0 {
		num.DivMod(num, base, mod)
		chars = append(chars, base36Alphabet[mod.Int64()])
	}

	// Reverse the string
	for i := len(chars) - 1; i >= 0; i-- {
		result.WriteByte(chars[i])
	}

	// Pad with zeros if needed
	str := result.String()
	if len(str) < length {
		str = strings.Repeat("0", length-len(str)) + str
	}

	// Truncate to exact length if needed
	if len(str) > length {
		str = str[len(str)-length:]
	}

	return str
}
