package session

import (
	"fmt"
	"time"
)

// Session represents a GitHub-triggered workflow session.
// Each session is tied to a specific issue and tracks the entire lifecycle
// from planning through implementation to PR merge.
type Session struct {
	// ID is the unique identifier, formatted as "owner-repo-issue-N"
	ID string

	// RepoOwner is the GitHub repository owner (user or organization)
	RepoOwner string

	// RepoName is the GitHub repository name
	RepoName string

	// IssueNumber is the GitHub issue number that triggered this session
	IssueNumber int

	// PRNumber is the pull request number, set after PR is created
	PRNumber *int

	// Phase is the current workflow phase
	Phase Phase

	// Branch is the git branch name for this session's work
	Branch string

	// ContainerID is the Docker container ID if a container is running
	ContainerID *string

	// PlanContent stores Claude's implementation plan for the implementation phase
	PlanContent *string

	// ErrorMessage stores the error message if phase is Error
	ErrorMessage *string

	// CreatedAt is when the session was created
	CreatedAt time.Time

	// LastActivity is the timestamp of the last activity on this session
	LastActivity time.Time
}

// NewSession creates a new session for a GitHub issue.
func NewSession(owner, repo string, issueNumber int) *Session {
	now := time.Now().UTC()
	return &Session{
		ID:           GenerateSessionID(owner, repo, issueNumber),
		RepoOwner:    owner,
		RepoName:     repo,
		IssueNumber:  issueNumber,
		Phase:        PhasePlanning,
		Branch:       GenerateBranchName(issueNumber),
		CreatedAt:    now,
		LastActivity: now,
	}
}

// GenerateSessionID creates a unique session ID from repo and issue info.
func GenerateSessionID(owner, repo string, issueNumber int) string {
	return fmt.Sprintf("%s-%s-issue-%d", owner, repo, issueNumber)
}

// GenerateBranchName creates a branch name for a session.
func GenerateBranchName(issueNumber int) string {
	return fmt.Sprintf("claude/issue-%d", issueNumber)
}

// RepoFullName returns the full repository name (owner/repo).
func (s *Session) RepoFullName() string {
	return fmt.Sprintf("%s/%s", s.RepoOwner, s.RepoName)
}

// TransitionTo attempts to transition the session to a new phase.
// Returns an error if the transition is not valid.
func (s *Session) TransitionTo(target Phase) error {
	if err := ValidateTransition(s.Phase, target); err != nil {
		return err
	}
	s.Phase = target
	s.LastActivity = time.Now().UTC()
	return nil
}

// SetPlan stores the implementation plan and transitions to awaiting approval.
func (s *Session) SetPlan(plan string) error {
	if err := s.TransitionTo(PhaseAwaitingApproval); err != nil {
		return err
	}
	s.PlanContent = &plan
	return nil
}

// Approve transitions the session from awaiting approval to implementing.
func (s *Session) Approve() error {
	return s.TransitionTo(PhaseImplementing)
}

// SetPRNumber sets the PR number after PR creation.
func (s *Session) SetPRNumber(prNumber int) {
	s.PRNumber = &prNumber
	s.LastActivity = time.Now().UTC()
}

// SetError transitions the session to error state with a message.
func (s *Session) SetError(msg string) error {
	if err := s.TransitionTo(PhaseError); err != nil {
		// Force transition to error even if not normally allowed
		s.Phase = PhaseError
	}
	s.ErrorMessage = &msg
	s.LastActivity = time.Now().UTC()
	return nil
}

// SetContainerID sets the active container ID.
func (s *Session) SetContainerID(containerID string) {
	s.ContainerID = &containerID
	s.LastActivity = time.Now().UTC()
}

// ClearContainerID clears the container ID (e.g., after container stops).
func (s *Session) ClearContainerID() {
	s.ContainerID = nil
	s.LastActivity = time.Now().UTC()
}

// Touch updates the last activity timestamp.
func (s *Session) Touch() {
	s.LastActivity = time.Now().UTC()
}

// Validate checks if the session has all required fields.
func (s *Session) Validate() error {
	if s.ID == "" {
		return fmt.Errorf("session ID is required")
	}
	if s.RepoOwner == "" {
		return fmt.Errorf("repository owner is required")
	}
	if s.RepoName == "" {
		return fmt.Errorf("repository name is required")
	}
	if s.IssueNumber <= 0 {
		return fmt.Errorf("issue number must be positive")
	}
	if !s.Phase.IsValid() {
		return fmt.Errorf("invalid phase: %s", s.Phase)
	}
	if s.Branch == "" {
		return fmt.Errorf("branch name is required")
	}
	return nil
}

// EventType represents the type of session event.
type EventType string

const (
	EventTypePhaseChange   EventType = "phase_change"
	EventTypeCommentPosted EventType = "comment_posted"
	EventTypeCommentReceived EventType = "comment_received"
	EventTypePRCreated     EventType = "pr_created"
	EventTypeError         EventType = "error"
	EventTypeContainerStart EventType = "container_start"
	EventTypeContainerStop EventType = "container_stop"
)

// SessionEvent represents an event in the session's history.
type SessionEvent struct {
	ID        int64
	SessionID string
	EventType EventType
	Payload   string // JSON-encoded event data
	CreatedAt time.Time
}

// SessionFilter defines criteria for filtering sessions.
type SessionFilter struct {
	// RepoOwner filters by repository owner
	RepoOwner string

	// RepoName filters by repository name
	RepoName string

	// Phase filters by current phase
	Phase *Phase

	// ActiveOnly returns only non-terminal sessions
	ActiveOnly bool

	// Limit limits the number of results
	Limit int

	// Offset skips the first N results
	Offset int
}
