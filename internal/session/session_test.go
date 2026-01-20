package session

import (
	"testing"
	"time"
)

func TestNewSession(t *testing.T) {
	sess := NewSession("owner", "repo", 42)

	if sess.ID != "owner-repo-issue-42" {
		t.Errorf("ID = %q, want %q", sess.ID, "owner-repo-issue-42")
	}
	if sess.RepoOwner != "owner" {
		t.Errorf("RepoOwner = %q, want %q", sess.RepoOwner, "owner")
	}
	if sess.RepoName != "repo" {
		t.Errorf("RepoName = %q, want %q", sess.RepoName, "repo")
	}
	if sess.IssueNumber != 42 {
		t.Errorf("IssueNumber = %d, want %d", sess.IssueNumber, 42)
	}
	if sess.Phase != PhasePlanning {
		t.Errorf("Phase = %q, want %q", sess.Phase, PhasePlanning)
	}
	if sess.Branch != "claude/issue-42" {
		t.Errorf("Branch = %q, want %q", sess.Branch, "claude/issue-42")
	}
	if sess.PRNumber != nil {
		t.Errorf("PRNumber = %v, want nil", sess.PRNumber)
	}
	if sess.ContainerID != nil {
		t.Errorf("ContainerID = %v, want nil", sess.ContainerID)
	}
}

func TestGenerateSessionID(t *testing.T) {
	tests := []struct {
		owner       string
		repo        string
		issueNumber int
		want        string
	}{
		{"owner", "repo", 1, "owner-repo-issue-1"},
		{"my-org", "my-repo", 123, "my-org-my-repo-issue-123"},
		{"user", "project", 999, "user-project-issue-999"},
	}

	for _, tt := range tests {
		t.Run(tt.want, func(t *testing.T) {
			got := GenerateSessionID(tt.owner, tt.repo, tt.issueNumber)
			if got != tt.want {
				t.Errorf("GenerateSessionID(%q, %q, %d) = %q, want %q",
					tt.owner, tt.repo, tt.issueNumber, got, tt.want)
			}
		})
	}
}

func TestGenerateBranchName(t *testing.T) {
	tests := []struct {
		issueNumber int
		want        string
	}{
		{1, "claude/issue-1"},
		{42, "claude/issue-42"},
		{999, "claude/issue-999"},
	}

	for _, tt := range tests {
		t.Run(tt.want, func(t *testing.T) {
			got := GenerateBranchName(tt.issueNumber)
			if got != tt.want {
				t.Errorf("GenerateBranchName(%d) = %q, want %q", tt.issueNumber, got, tt.want)
			}
		})
	}
}

func TestSessionRepoFullName(t *testing.T) {
	sess := NewSession("owner", "repo", 1)
	want := "owner/repo"
	if got := sess.RepoFullName(); got != want {
		t.Errorf("RepoFullName() = %q, want %q", got, want)
	}
}

func TestSessionTransitionTo(t *testing.T) {
	sess := NewSession("owner", "repo", 1)

	// Valid transition
	err := sess.TransitionTo(PhaseAwaitingApproval)
	if err != nil {
		t.Errorf("TransitionTo(awaiting_approval) = %v, want nil", err)
	}
	if sess.Phase != PhaseAwaitingApproval {
		t.Errorf("Phase = %q, want %q", sess.Phase, PhaseAwaitingApproval)
	}

	// Invalid transition
	err = sess.TransitionTo(PhaseCompleted)
	if err == nil {
		t.Error("TransitionTo(completed) = nil, want error")
	}
	if sess.Phase != PhaseAwaitingApproval {
		t.Errorf("Phase changed to %q after invalid transition", sess.Phase)
	}
}

func TestSessionSetPlan(t *testing.T) {
	sess := NewSession("owner", "repo", 1)
	plan := "This is the implementation plan"

	err := sess.SetPlan(plan)
	if err != nil {
		t.Errorf("SetPlan() = %v, want nil", err)
	}
	if sess.Phase != PhaseAwaitingApproval {
		t.Errorf("Phase = %q, want %q", sess.Phase, PhaseAwaitingApproval)
	}
	if sess.PlanContent == nil || *sess.PlanContent != plan {
		t.Errorf("PlanContent = %v, want %q", sess.PlanContent, plan)
	}
}

func TestSessionApprove(t *testing.T) {
	sess := NewSession("owner", "repo", 1)

	// Can't approve from planning phase
	err := sess.Approve()
	if err == nil {
		t.Error("Approve() from planning = nil, want error")
	}

	// Set plan first
	sess.SetPlan("plan")

	// Now approve should work
	err = sess.Approve()
	if err != nil {
		t.Errorf("Approve() = %v, want nil", err)
	}
	if sess.Phase != PhaseImplementing {
		t.Errorf("Phase = %q, want %q", sess.Phase, PhaseImplementing)
	}
}

func TestSessionSetPRNumber(t *testing.T) {
	sess := NewSession("owner", "repo", 1)
	prNum := 123

	sess.SetPRNumber(prNum)

	if sess.PRNumber == nil || *sess.PRNumber != prNum {
		t.Errorf("PRNumber = %v, want %d", sess.PRNumber, prNum)
	}
}

func TestSessionSetError(t *testing.T) {
	sess := NewSession("owner", "repo", 1)
	msg := "Something went wrong"

	err := sess.SetError(msg)
	if err != nil {
		t.Errorf("SetError() = %v, want nil", err)
	}
	if sess.Phase != PhaseError {
		t.Errorf("Phase = %q, want %q", sess.Phase, PhaseError)
	}
	if sess.ErrorMessage == nil || *sess.ErrorMessage != msg {
		t.Errorf("ErrorMessage = %v, want %q", sess.ErrorMessage, msg)
	}
}

func TestSessionSetContainerID(t *testing.T) {
	sess := NewSession("owner", "repo", 1)
	containerID := "abc123"

	sess.SetContainerID(containerID)

	if sess.ContainerID == nil || *sess.ContainerID != containerID {
		t.Errorf("ContainerID = %v, want %q", sess.ContainerID, containerID)
	}

	sess.ClearContainerID()
	if sess.ContainerID != nil {
		t.Errorf("ContainerID = %v after clear, want nil", sess.ContainerID)
	}
}

func TestSessionTouch(t *testing.T) {
	sess := NewSession("owner", "repo", 1)
	originalTime := sess.LastActivity

	// Sleep a tiny bit to ensure time difference
	time.Sleep(time.Millisecond)
	sess.Touch()

	if !sess.LastActivity.After(originalTime) {
		t.Errorf("LastActivity not updated: %v vs %v", sess.LastActivity, originalTime)
	}
}

func TestSessionValidate(t *testing.T) {
	tests := []struct {
		name    string
		session *Session
		wantErr bool
	}{
		{
			name:    "valid session",
			session: NewSession("owner", "repo", 1),
			wantErr: false,
		},
		{
			name: "missing ID",
			session: &Session{
				RepoOwner:   "owner",
				RepoName:    "repo",
				IssueNumber: 1,
				Phase:       PhasePlanning,
				Branch:      "branch",
			},
			wantErr: true,
		},
		{
			name: "missing owner",
			session: &Session{
				ID:          "id",
				RepoName:    "repo",
				IssueNumber: 1,
				Phase:       PhasePlanning,
				Branch:      "branch",
			},
			wantErr: true,
		},
		{
			name: "missing repo",
			session: &Session{
				ID:          "id",
				RepoOwner:   "owner",
				IssueNumber: 1,
				Phase:       PhasePlanning,
				Branch:      "branch",
			},
			wantErr: true,
		},
		{
			name: "invalid issue number",
			session: &Session{
				ID:          "id",
				RepoOwner:   "owner",
				RepoName:    "repo",
				IssueNumber: 0,
				Phase:       PhasePlanning,
				Branch:      "branch",
			},
			wantErr: true,
		},
		{
			name: "invalid phase",
			session: &Session{
				ID:          "id",
				RepoOwner:   "owner",
				RepoName:    "repo",
				IssueNumber: 1,
				Phase:       Phase("invalid"),
				Branch:      "branch",
			},
			wantErr: true,
		},
		{
			name: "missing branch",
			session: &Session{
				ID:          "id",
				RepoOwner:   "owner",
				RepoName:    "repo",
				IssueNumber: 1,
				Phase:       PhasePlanning,
			},
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := tt.session.Validate()
			if (err != nil) != tt.wantErr {
				t.Errorf("Validate() error = %v, wantErr %v", err, tt.wantErr)
			}
		})
	}
}
