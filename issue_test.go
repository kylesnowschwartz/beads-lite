package beadslite

import (
	"errors"
	"strings"
	"testing"
	"time"
)

func TestStatusConstants(t *testing.T) {
	// Verify 5-column kanban status constants
	if StatusBacklog != "backlog" {
		t.Errorf("StatusBacklog = %q, want %q", StatusBacklog, "backlog")
	}
	if StatusTodo != "todo" {
		t.Errorf("StatusTodo = %q, want %q", StatusTodo, "todo")
	}
	if StatusDoing != "doing" {
		t.Errorf("StatusDoing = %q, want %q", StatusDoing, "doing")
	}
	if StatusReview != "review" {
		t.Errorf("StatusReview = %q, want %q", StatusReview, "review")
	}
	if StatusDone != "done" {
		t.Errorf("StatusDone = %q, want %q", StatusDone, "done")
	}
}

func TestIssueTypeConstants(t *testing.T) {
	// Verify issue type constants are defined
	if IssueTypeTask != "task" {
		t.Errorf("IssueTypeTask = %q, want %q", IssueTypeTask, "task")
	}
	if IssueTypeBug != "bug" {
		t.Errorf("IssueTypeBug = %q, want %q", IssueTypeBug, "bug")
	}
	if IssueTypeFeature != "feature" {
		t.Errorf("IssueTypeFeature = %q, want %q", IssueTypeFeature, "feature")
	}
	if IssueTypeEpic != "epic" {
		t.Errorf("IssueTypeEpic = %q, want %q", IssueTypeEpic, "epic")
	}
}

func TestValidStatus(t *testing.T) {
	tests := []struct {
		status Status
		want   bool
	}{
		{StatusBacklog, true},
		{StatusTodo, true},
		{StatusDoing, true},
		{StatusReview, true},
		{StatusDone, true},
		{"open", false},        // legacy status no longer valid
		{"in_progress", false}, // legacy status no longer valid
		{"closed", false},      // legacy status no longer valid
		{"invalid", false},
		{"", false},
	}

	for _, tt := range tests {
		got := tt.status.Valid()
		if got != tt.want {
			t.Errorf("Status(%q).Valid() = %v, want %v", tt.status, got, tt.want)
		}
	}
}

func TestValidIssueType(t *testing.T) {
	tests := []struct {
		issueType IssueType
		want      bool
	}{
		{IssueTypeTask, true},
		{IssueTypeBug, true},
		{IssueTypeFeature, true},
		{IssueTypeEpic, true},
		{"invalid", false},
		{"", false},
	}

	for _, tt := range tests {
		got := tt.issueType.Valid()
		if got != tt.want {
			t.Errorf("IssueType(%q).Valid() = %v, want %v", tt.issueType, got, tt.want)
		}
	}
}

func TestResolutionConstants(t *testing.T) {
	if ResolutionDone != "done" {
		t.Errorf("ResolutionDone = %q, want %q", ResolutionDone, "done")
	}
	if ResolutionWontfix != "wontfix" {
		t.Errorf("ResolutionWontfix = %q, want %q", ResolutionWontfix, "wontfix")
	}
	if ResolutionDuplicate != "duplicate" {
		t.Errorf("ResolutionDuplicate = %q, want %q", ResolutionDuplicate, "duplicate")
	}
}

func TestValidResolution(t *testing.T) {
	tests := []struct {
		resolution Resolution
		want       bool
	}{
		{ResolutionDone, true},
		{ResolutionWontfix, true},
		{ResolutionDuplicate, true},
		{"", true}, // empty is valid (backwards compat)
		{"invalid", false},
		{"wontdo", false}, // typo should fail
	}

	for _, tt := range tests {
		got := tt.resolution.Valid()
		if got != tt.want {
			t.Errorf("Resolution(%q).Valid() = %v, want %v", tt.resolution, got, tt.want)
		}
	}
}

func TestNewIssue(t *testing.T) {
	title := "Test Issue"
	issue := NewIssue(title)

	// Check ID format: bl-XXXX (4 char hash)
	if !strings.HasPrefix(issue.ID, "bl-") {
		t.Errorf("ID = %q, want prefix 'bl-'", issue.ID)
	}
	if len(issue.ID) != 7 { // "bl-" + 4 chars
		t.Errorf("ID length = %d, want 7", len(issue.ID))
	}

	// Check defaults
	if issue.Title != title {
		t.Errorf("Title = %q, want %q", issue.Title, title)
	}
	if issue.Status != StatusBacklog {
		t.Errorf("Status = %q, want %q", issue.Status, StatusBacklog)
	}
	if issue.Priority != 2 {
		t.Errorf("Priority = %d, want 2 (medium)", issue.Priority)
	}
	if issue.Type != IssueTypeTask {
		t.Errorf("Type = %q, want %q", issue.Type, IssueTypeTask)
	}
	if issue.CreatedAt.IsZero() {
		t.Error("CreatedAt should not be zero")
	}
	if issue.UpdatedAt.IsZero() {
		t.Error("UpdatedAt should not be zero")
	}
}

func TestNewIssueUniqueIDs(t *testing.T) {
	// Same title should generate different IDs (due to timestamp/nonce)
	issue1 := NewIssue("Same Title")
	time.Sleep(time.Millisecond) // Ensure different timestamp
	issue2 := NewIssue("Same Title")

	if issue1.ID == issue2.ID {
		t.Errorf("Expected different IDs, got %q for both", issue1.ID)
	}
}

func TestIssueValidate(t *testing.T) {
	tests := []struct {
		name    string
		issue   Issue
		wantErr bool
	}{
		{
			name: "valid issue",
			issue: Issue{
				ID:     "bl-test",
				Title:  "Valid Title",
				Status: StatusTodo,
				Type:   IssueTypeTask,
			},
			wantErr: false,
		},
		{
			name: "empty title",
			issue: Issue{
				ID:     "bl-test",
				Title:  "",
				Status: StatusTodo,
				Type:   IssueTypeTask,
			},
			wantErr: true,
		},
		{
			name: "whitespace only title",
			issue: Issue{
				ID:     "bl-test",
				Title:  "   ",
				Status: StatusTodo,
				Type:   IssueTypeTask,
			},
			wantErr: true,
		},
		{
			name: "invalid status",
			issue: Issue{
				ID:     "bl-test",
				Title:  "Valid Title",
				Status: "invalid",
				Type:   IssueTypeTask,
			},
			wantErr: true,
		},
		{
			name: "invalid type",
			issue: Issue{
				ID:     "bl-test",
				Title:  "Valid Title",
				Status: StatusTodo,
				Type:   "invalid",
			},
			wantErr: true,
		},
		{
			name: "invalid priority (negative)",
			issue: Issue{
				ID:       "bl-test",
				Title:    "Valid Title",
				Status:   StatusTodo,
				Type:     IssueTypeTask,
				Priority: -1,
			},
			wantErr: true,
		},
		{
			name: "invalid priority (too high)",
			issue: Issue{
				ID:       "bl-test",
				Title:    "Valid Title",
				Status:   StatusTodo,
				Type:     IssueTypeTask,
				Priority: 5,
			},
			wantErr: true,
		},
		{
			name: "valid priority P0",
			issue: Issue{
				ID:       "bl-test",
				Title:    "Valid Title",
				Status:   StatusTodo,
				Type:     IssueTypeTask,
				Priority: 0,
			},
			wantErr: false,
		},
		{
			name: "valid priority P4",
			issue: Issue{
				ID:       "bl-test",
				Title:    "Valid Title",
				Status:   StatusTodo,
				Type:     IssueTypeTask,
				Priority: 4,
			},
			wantErr: false,
		},
		{
			name: "valid resolution done",
			issue: Issue{
				ID:         "bl-test",
				Title:      "Valid Title",
				Status:     StatusDone,
				Type:       IssueTypeTask,
				Resolution: ResolutionDone,
			},
			wantErr: false,
		},
		{
			name: "valid resolution wontfix",
			issue: Issue{
				ID:         "bl-test",
				Title:      "Valid Title",
				Status:     StatusDone,
				Type:       IssueTypeTask,
				Resolution: ResolutionWontfix,
			},
			wantErr: false,
		},
		{
			name: "valid resolution empty (backwards compat)",
			issue: Issue{
				ID:         "bl-test",
				Title:      "Valid Title",
				Status:     StatusDone,
				Type:       IssueTypeTask,
				Resolution: "",
			},
			wantErr: false,
		},
		{
			name: "invalid resolution",
			issue: Issue{
				ID:         "bl-test",
				Title:      "Valid Title",
				Status:     StatusDone,
				Type:       IssueTypeTask,
				Resolution: "invalid",
			},
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := tt.issue.Validate()
			if (err != nil) != tt.wantErr {
				t.Errorf("Validate() error = %v, wantErr %v", err, tt.wantErr)
			}
		})
	}
}

func TestValidateTransition(t *testing.T) {
	policyOn := TransitionPolicy{RequireSpecsForReview: true}
	policyOff := TransitionPolicy{RequireSpecsForReview: false}

	tests := []struct {
		name      string
		issue     *Issue
		newStatus Status
		policy    TransitionPolicy
		wantErr   bool
	}{
		{
			name: "blocks review with unchecked specs",
			issue: &Issue{
				Specifications: []Spec{
					{Text: "done", Checked: true},
					{Text: "not done", Checked: false},
				},
			},
			newStatus: StatusReview,
			policy:    policyOn,
			wantErr:   true,
		},
		{
			name: "allows review when all checked",
			issue: &Issue{
				Specifications: []Spec{
					{Text: "done", Checked: true},
					{Text: "also done", Checked: true},
				},
			},
			newStatus: StatusReview,
			policy:    policyOn,
			wantErr:   false,
		},
		{
			name:      "allows review with no specs (vacuous truth)",
			issue:     &Issue{},
			newStatus: StatusReview,
			policy:    policyOn,
			wantErr:   false,
		},
		{
			name: "allows review when policy disabled",
			issue: &Issue{
				Specifications: []Spec{
					{Text: "not done", Checked: false},
				},
			},
			newStatus: StatusReview,
			policy:    policyOff,
			wantErr:   false,
		},
		{
			name: "allows non-review transitions regardless",
			issue: &Issue{
				Specifications: []Spec{
					{Text: "not done", Checked: false},
				},
			},
			newStatus: StatusDoing,
			policy:    policyOn,
			wantErr:   false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := ValidateTransition(tt.issue, tt.newStatus, tt.policy)
			if (err != nil) != tt.wantErr {
				t.Errorf("ValidateTransition() error = %v, wantErr %v", err, tt.wantErr)
			}
		})
	}

	// Verify errors.Is works with ErrSpecsIncomplete
	t.Run("errors.Is works with ErrSpecsIncomplete", func(t *testing.T) {
		issue := &Issue{
			Specifications: []Spec{{Text: "unchecked", Checked: false}},
		}
		err := ValidateTransition(issue, StatusReview, policyOn)
		if err == nil {
			t.Fatal("expected error, got nil")
		}
		if !errors.Is(err, ErrSpecsIncomplete) {
			t.Errorf("errors.Is(err, ErrSpecsIncomplete) = false, want true; err = %v", err)
		}
	})
}
