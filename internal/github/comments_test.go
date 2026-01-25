package github

import (
	"testing"
)

func TestFormatComment(t *testing.T) {
	comment := FormatComment("owner-repo-issue-42", "planning", "This is the plan.")

	// Check that it contains the metadata
	meta := ParseManfredComment(comment)
	if meta == nil {
		t.Fatal("expected to parse Manfred comment metadata")
	}
	if meta.SessionID != "owner-repo-issue-42" {
		t.Errorf("expected session ID 'owner-repo-issue-42', got %q", meta.SessionID)
	}
	if meta.Phase != "planning" {
		t.Errorf("expected phase 'planning', got %q", meta.Phase)
	}

	// Check that it contains the content
	if !IsManfredComment(comment) {
		t.Error("expected IsManfredComment to return true")
	}
}

func TestFormatPlanComment(t *testing.T) {
	comment := FormatPlanComment("test-session", "1. Do this\n2. Do that")

	meta := ParseManfredComment(comment)
	if meta == nil {
		t.Fatal("expected to parse metadata")
	}
	if meta.Phase != "planning" {
		t.Errorf("expected phase 'planning', got %q", meta.Phase)
	}
}

func TestParseManfredComment(t *testing.T) {
	tests := []struct {
		name      string
		body      string
		wantNil   bool
		sessionID string
		phase     string
	}{
		{
			name:      "valid comment",
			body:      "<!-- manfred:session:my-session:phase:planning -->\n\nContent here",
			wantNil:   false,
			sessionID: "my-session",
			phase:     "planning",
		},
		{
			name:    "not a manfred comment",
			body:    "Just a regular comment",
			wantNil: true,
		},
		{
			name:    "empty body",
			body:    "",
			wantNil: true,
		},
		{
			name:      "complex session ID",
			body:      "<!-- manfred:session:owner-repo-issue-123:phase:implementing -->",
			wantNil:   false,
			sessionID: "owner-repo-issue-123",
			phase:     "implementing",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			meta := ParseManfredComment(tt.body)
			if tt.wantNil {
				if meta != nil {
					t.Errorf("expected nil, got %+v", meta)
				}
				return
			}
			if meta == nil {
				t.Fatal("expected non-nil meta")
			}
			if meta.SessionID != tt.sessionID {
				t.Errorf("sessionID = %q, want %q", meta.SessionID, tt.sessionID)
			}
			if meta.Phase != tt.phase {
				t.Errorf("phase = %q, want %q", meta.Phase, tt.phase)
			}
		})
	}
}

func TestIsApproval(t *testing.T) {
	tests := []struct {
		body string
		want bool
	}{
		{"@claude approved", true},
		{"@claude approve", true},
		{"@CLAUDE APPROVED", true},
		{"I think @claude approved this", true},
		{"@claude lgtm", true},
		{"@claude go ahead", true},
		{"/approve", true},
		{"not an approval", false},
		{"approved by someone", false},
		{"claude approved", false}, // missing @
	}

	for _, tt := range tests {
		t.Run(tt.body, func(t *testing.T) {
			if got := IsApproval(tt.body); got != tt.want {
				t.Errorf("IsApproval(%q) = %v, want %v", tt.body, got, tt.want)
			}
		})
	}
}

func TestIsRetryRequest(t *testing.T) {
	tests := []struct {
		body string
		want bool
	}{
		{"@claude retry", true},
		{"@CLAUDE RETRY", true},
		{"/retry", true},
		{"please @claude retry this", true},
		{"just retry it", false},
		{"claude retry", false}, // missing @
	}

	for _, tt := range tests {
		t.Run(tt.body, func(t *testing.T) {
			if got := IsRetryRequest(tt.body); got != tt.want {
				t.Errorf("IsRetryRequest(%q) = %v, want %v", tt.body, got, tt.want)
			}
		})
	}
}

func TestExtractFeedback(t *testing.T) {
	tests := []struct {
		name string
		body string
		want string
	}{
		{
			name: "removes html comments",
			body: "<!-- manfred:session:x:phase:y -->\n\nActual feedback here",
			want: "Actual feedback here",
		},
		{
			name: "removes footer",
			body: "Feedback\n\n---\n\n<sub>Reply with something</sub>",
			want: "Feedback",
		},
		{
			name: "plain text unchanged",
			body: "Just some feedback",
			want: "Just some feedback",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := ExtractFeedback(tt.body)
			if got != tt.want {
				t.Errorf("ExtractFeedback() = %q, want %q", got, tt.want)
			}
		})
	}
}
