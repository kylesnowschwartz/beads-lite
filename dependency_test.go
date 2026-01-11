package beadslite

import (
	"testing"
	"time"
)

func TestDepTypeConstants(t *testing.T) {
	if DepBlocks != "blocks" {
		t.Errorf("DepBlocks = %q, want %q", DepBlocks, "blocks")
	}
}

func TestDepTypeValid(t *testing.T) {
	tests := []struct {
		depType DepType
		want    bool
	}{
		{DepBlocks, true},
		{"parent-child", false},
		{"related", false},
		{"invalid", false},
		{"", false},
	}

	for _, tt := range tests {
		got := tt.depType.Valid()
		if got != tt.want {
			t.Errorf("DepType(%q).Valid() = %v, want %v", tt.depType, got, tt.want)
		}
	}
}

func TestNewDependency(t *testing.T) {
	dep := NewDependency("bl-1234", "bl-5678", DepBlocks)

	if dep.IssueID != "bl-1234" {
		t.Errorf("IssueID = %q, want %q", dep.IssueID, "bl-1234")
	}
	if dep.DependsOnID != "bl-5678" {
		t.Errorf("DependsOnID = %q, want %q", dep.DependsOnID, "bl-5678")
	}
	if dep.Type != DepBlocks {
		t.Errorf("Type = %q, want %q", dep.Type, DepBlocks)
	}
	if dep.CreatedAt.IsZero() {
		t.Error("CreatedAt should not be zero")
	}
}

func TestDependencyValidate(t *testing.T) {
	now := time.Now()
	tests := []struct {
		name    string
		dep     Dependency
		wantErr bool
	}{
		{
			name: "valid dependency",
			dep: Dependency{
				IssueID:     "bl-1234",
				DependsOnID: "bl-5678",
				Type:        DepBlocks,
				CreatedAt:   now,
			},
			wantErr: false,
		},
		{
			name: "empty issue_id",
			dep: Dependency{
				IssueID:     "",
				DependsOnID: "bl-5678",
				Type:        DepBlocks,
				CreatedAt:   now,
			},
			wantErr: true,
		},
		{
			name: "empty depends_on_id",
			dep: Dependency{
				IssueID:     "bl-1234",
				DependsOnID: "",
				Type:        DepBlocks,
				CreatedAt:   now,
			},
			wantErr: true,
		},
		{
			name: "invalid type",
			dep: Dependency{
				IssueID:     "bl-1234",
				DependsOnID: "bl-5678",
				Type:        "invalid",
				CreatedAt:   now,
			},
			wantErr: true,
		},
		{
			name: "self-reference",
			dep: Dependency{
				IssueID:     "bl-1234",
				DependsOnID: "bl-1234",
				Type:        DepBlocks,
				CreatedAt:   now,
			},
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := tt.dep.Validate()
			if (err != nil) != tt.wantErr {
				t.Errorf("Validate() error = %v, wantErr %v", err, tt.wantErr)
			}
		})
	}
}
