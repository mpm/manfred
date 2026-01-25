# GitHub Integration Implementation Plan

This document breaks down the GitHub integration spec (`gh-plan.md`) into implementable phases. Each phase builds on the previous and delivers testable functionality.

## Overview

The GitHub integration adds these major capabilities:
1. Receive GitHub webhooks and trigger sessions
2. Manage session state (separate from tickets)
3. Interact with GitHub API (comments, PRs, labels)
4. Phase-based workflow (planning → approval → implementation → review)

## Phase 1: Session Store with SQLite

**Goal**: Replace/augment YAML file-based storage with SQLite, introduce the Session model.

### New Components

```
internal/
├── store/
│   ├── sqlite.go         # SQLite connection management
│   └── migrations.go     # Schema migrations
├── session/
│   ├── session.go        # Session model
│   ├── phase.go          # Phase enum & state machine
│   └── store.go          # Session CRUD operations
```

### Session Model

```go
type Phase string
const (
    PhasePlanning        Phase = "planning"
    PhaseAwaitingApproval Phase = "awaiting_approval"
    PhaseImplementing    Phase = "implementing"
    PhaseInReview        Phase = "in_review"
    PhaseRevising        Phase = "revising"
    PhaseCompleted       Phase = "completed"
    PhaseError           Phase = "error"
)

type Session struct {
    ID            string     // "owner-repo-issue-42"
    RepoOwner     string
    RepoName      string
    IssueNumber   int
    PRNumber      *int       // Set when PR is created
    Phase         Phase
    Branch        string     // "claude/issue-42"
    ContainerID   *string    // Active container (if any)
    PlanContent   *string    // Claude's plan (for implementation phase)
    CreatedAt     time.Time
    LastActivity  time.Time
}
```

### Database Schema

```sql
CREATE TABLE sessions (
    id TEXT PRIMARY KEY,
    repo_owner TEXT NOT NULL,
    repo_name TEXT NOT NULL,
    issue_number INTEGER NOT NULL,
    pr_number INTEGER,
    phase TEXT NOT NULL DEFAULT 'planning',
    branch TEXT NOT NULL,
    container_id TEXT,
    plan_content TEXT,
    created_at TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP,
    last_activity TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP,
    UNIQUE(repo_owner, repo_name, issue_number)
);

CREATE TABLE session_events (
    id INTEGER PRIMARY KEY AUTOINCREMENT,
    session_id TEXT NOT NULL REFERENCES sessions(id),
    event_type TEXT NOT NULL,  -- 'phase_change', 'comment', 'error', etc.
    payload TEXT,              -- JSON blob
    created_at TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP
);
```

### Session Store Interface

```go
type SessionStore interface {
    Create(ctx context.Context, s *Session) error
    Get(ctx context.Context, id string) (*Session, error)
    GetByIssue(ctx context.Context, owner, repo string, issue int) (*Session, error)
    Update(ctx context.Context, s *Session) error
    Delete(ctx context.Context, id string) error
    List(ctx context.Context, filter SessionFilter) ([]Session, error)
    RecordEvent(ctx context.Context, sessionID string, event SessionEvent) error
}
```

### Phase State Machine

```go
// ValidTransitions defines allowed phase transitions
var ValidTransitions = map[Phase][]Phase{
    PhasePlanning:         {PhaseAwaitingApproval, PhaseError},
    PhaseAwaitingApproval: {PhasePlanning, PhaseImplementing, PhaseError},
    PhaseImplementing:     {PhaseInReview, PhaseError},
    PhaseInReview:         {PhaseRevising, PhaseCompleted, PhaseError},
    PhaseRevising:         {PhaseInReview, PhaseError},
    PhaseCompleted:        {},  // Terminal
    PhaseError:            {PhasePlanning},  // Can retry
}

func (s *Session) CanTransitionTo(target Phase) bool
func (s *Session) TransitionTo(target Phase) error
```

### Deliverables
- [x] SQLite connection manager with WAL mode
- [x] Schema migrations (up/down)
- [x] Session model with validation
- [x] SessionStore implementation
- [x] Phase state machine with validation
- [x] Unit tests for store and state machine
- [x] CLI: `manfred session list`, `manfred session show <id>`

### Migration Path
- Existing ticket system continues to work unchanged
- Sessions are a parallel concept for GitHub-driven workflows
- Future: Could migrate tickets to SQLite too

---

## Phase 2: GitHub Client

**Goal**: Wrap GitHub API operations needed for the integration.

### New Components

```
internal/
├── github/
│   ├── client.go         # HTTP client, auth, rate limiting
│   ├── issues.go         # Issue operations
│   ├── comments.go       # Comment operations
│   ├── pulls.go          # PR operations
│   └── webhooks.go       # Webhook signature validation
```

### GitHub Client Interface

```go
type Client interface {
    // Issues
    GetIssue(ctx context.Context, owner, repo string, number int) (*Issue, error)
    GetIssueComments(ctx context.Context, owner, repo string, number int) ([]Comment, error)
    AddIssueComment(ctx context.Context, owner, repo string, number int, body string) (*Comment, error)
    AddLabel(ctx context.Context, owner, repo string, number int, label string) error
    RemoveLabel(ctx context.Context, owner, repo string, number int, label string) error

    // Pull Requests
    CreatePullRequest(ctx context.Context, owner, repo string, pr *CreatePRRequest) (*PullRequest, error)
    GetPullRequest(ctx context.Context, owner, repo string, number int) (*PullRequest, error)
    GetPRComments(ctx context.Context, owner, repo string, number int) ([]Comment, error)
    GetPRReviewComments(ctx context.Context, owner, repo string, number int) ([]ReviewComment, error)

    // Webhooks
    ValidateWebhookSignature(payload []byte, signature string) error
}
```

### Comment Formatting

```go
// FormatComment creates a Manfred-identifiable comment
func FormatComment(sessionID string, phase Phase, content string) string {
    return fmt.Sprintf(`<!-- manfred:session:%s:phase:%s -->

%s

---

<sub>Reply with `+"`@claude approved`"+` to start implementation, or provide feedback.</sub>`,
        sessionID, phase, content)
}

// ParseManfredComment extracts metadata from a comment
func ParseManfredComment(body string) (*ManfredMeta, bool)
```

### Configuration

```yaml
github:
  # Option 1: Personal Access Token
  token: ${GITHUB_TOKEN}

  # Option 2: GitHub App (future)
  app_id: 12345
  private_key_path: ./github.pem
  installation_id: 67890

  # Rate limiting
  rate_limit_buffer: 100  # Stop when this many requests remain
```

### Deliverables
- [x] GitHub client with PAT authentication
- [x] Issue/comment/PR operations
- [x] Webhook signature validation (HMAC SHA-256)
- [x] Rate limit handling with backoff
- [x] Comment formatting/parsing helpers
- [x] Unit tests with mock responses
- [x] CLI: `manfred github test-auth` (verify credentials)

---

## Phase 3: Webhook Server & Event Router

**Goal**: HTTP server that receives GitHub webhooks and routes them to session handlers.

### New Components

```
internal/
├── server/
│   ├── server.go         # HTTP server setup
│   ├── handlers.go       # Route handlers
│   └── middleware.go     # Logging, recovery, etc.
├── webhook/
│   ├── router.go         # Event routing logic
│   ├── handlers.go       # Event-specific handlers
│   └── events.go         # Event type definitions
```

### Webhook Events to Handle

| Event | Handler |
|-------|---------|
| `issues.opened` | Start session if has trigger label |
| `issues.labeled` | Start session if trigger label added |
| `issues.closed` | Cleanup session |
| `issue_comment.created` | Process as input or approval |
| `pull_request.closed` | Cleanup session |
| `pull_request_review.submitted` | Process review feedback |
| `pull_request_review_comment.created` | Process review comment |

### Event Router

```go
type EventRouter struct {
    sessionStore  session.Store
    githubClient  github.Client
    config        *WebhookConfig
    phaseHandlers map[Phase]PhaseHandler
}

func (r *EventRouter) HandleEvent(ctx context.Context, event WebhookEvent) error

// Check if comment contains approval keyword
func (r *EventRouter) isApproval(comment string) bool

// Check if issue has trigger label
func (r *EventRouter) hasTriggerLabel(labels []Label) bool
```

### Server Endpoints

```
POST /webhook/github     # GitHub webhook receiver
GET  /health             # Health check
GET  /sessions           # List active sessions (admin)
GET  /sessions/:id       # Session details (admin)
```

### Configuration

```yaml
server:
  addr: 0.0.0.0
  port: 8080
  webhook_path: /webhook/github

triggers:
  labels:
    - claude
    - manfred
  approval_keywords:
    - "@claude approved"
    - "@claude lgtm"
    - "@claude go ahead"
    - "/approve"
```

### Deliverables
- [ ] HTTP server with graceful shutdown
- [ ] Webhook endpoint with signature validation
- [ ] Event parsing and routing
- [ ] Session creation on trigger events
- [ ] Approval detection from comments
- [ ] Health check endpoint
- [ ] Integration tests with sample payloads
- [ ] CLI: `manfred serve` (start webhook server)

---

## Phase 4: Prompt Builder & Phase Handlers

**Goal**: Build prompts for each phase, integrate with existing job runner.

### New Components

```
internal/
├── prompt/
│   ├── builder.go        # Prompt construction
│   └── templates.go      # Prompt templates
├── orchestrator/
│   ├── orchestrator.go   # Main workflow coordinator
│   ├── planning.go       # Planning phase handler
│   ├── implementing.go   # Implementation phase handler
│   └── revising.go       # Revision phase handler
```

### Prompt Builder

```go
type PromptBuilder struct {
    templates map[Phase]*template.Template
}

type PromptContext struct {
    Session      *session.Session
    Issue        *github.Issue
    Comments     []github.Comment
    PlanContent  string              // For implementation phase
    NewFeedback  []github.Comment    // For revision phase
}

func (b *PromptBuilder) Build(phase Phase, ctx *PromptContext) (string, error)
```

### Prompt Templates

**Planning**:
```
You are working on GitHub issue #{{.Issue.Number}} in repository {{.Session.RepoOwner}}/{{.Session.RepoName}}.

Title: {{.Issue.Title}}

Description:
{{.Issue.Body}}

Previous comments:
{{range .Comments}}
---
@{{.User.Login}} ({{.CreatedAt.Format "2006-01-02"}}):
{{.Body}}
{{end}}

---

Create a detailed implementation plan for this issue. Include:
1. Your understanding of the requirements
2. Files that need to be created or modified
3. Step-by-step implementation approach
4. Any questions or clarifications needed

Do NOT implement yet. Only plan.
```

**Implementation**: (similar structure, includes approved plan)

**Revision**: (includes PR feedback)

### Phase Handlers

```go
type PhaseHandler interface {
    Handle(ctx context.Context, session *session.Session, input PhaseInput) (*PhaseOutput, error)
}

type PlanningHandler struct {
    jobRunner     *job.Runner
    githubClient  github.Client
    promptBuilder *prompt.Builder
}

func (h *PlanningHandler) Handle(ctx context.Context, session *session.Session, input PhaseInput) (*PhaseOutput, error) {
    // 1. Fetch issue details
    // 2. Build planning prompt
    // 3. Start container, run Claude
    // 4. Post plan as comment
    // 5. Transition to awaiting_approval
}
```

### Orchestrator

```go
type Orchestrator struct {
    sessionStore  session.Store
    githubClient  github.Client
    jobRunner     *job.Runner
    handlers      map[Phase]PhaseHandler
}

// ProcessSession runs the appropriate phase handler
func (o *Orchestrator) ProcessSession(ctx context.Context, sessionID string) error

// HandleApproval transitions from awaiting_approval to implementing
func (o *Orchestrator) HandleApproval(ctx context.Context, sessionID string) error

// HandleFeedback processes user comments during planning/review
func (o *Orchestrator) HandleFeedback(ctx context.Context, sessionID string, comment string) error
```

### Deliverables
- [ ] Prompt templates for all phases
- [ ] PromptBuilder with template rendering
- [ ] PlanningHandler (creates plan, posts comment)
- [ ] ImplementingHandler (runs job, creates PR)
- [ ] RevisingHandler (processes feedback, pushes updates)
- [ ] Orchestrator coordinating handlers
- [ ] Integration with existing job runner
- [ ] Tests for prompt building and phase handling

---

## Phase 5: End-to-End Integration

**Goal**: Wire everything together, add error handling, polish.

### Remaining Work

1. **Git Operations**
   - Push to remote (currently stubbed)
   - PR creation after implementation
   - Handle force-push for revisions

2. **Error Handling**
   - Container startup failures → post error comment
   - Max turns exceeded → partial progress comment
   - Git push failures → await user intervention
   - Rate limit → queue and retry

3. **Session Lifecycle**
   - Idle timeout cleanup
   - Post-merge cleanup delay
   - Max concurrent sessions limit

4. **Observability**
   - Structured logging
   - Metrics (sessions active, jobs run, errors)
   - Session event history

5. **Configuration Consolidation**
   - Merge all new config into `manfred.yaml`
   - Environment variable support for secrets
   - Config validation on startup

### Updated CLI Commands

```bash
# GitHub-specific commands
manfred serve                    # Start webhook server
manfred github test-auth         # Verify GitHub credentials
manfred github webhook-url       # Print webhook URL for setup

# Session management
manfred session list [--repo owner/repo]
manfred session show <session-id>
manfred session retry <session-id>   # Restart from error
manfred session cleanup              # Remove stale sessions
```

### Deliverables
- [ ] Git push implementation
- [ ] PR creation flow
- [ ] Error handling for all failure modes
- [ ] Session cleanup (idle, merged, closed)
- [ ] Configuration validation
- [ ] End-to-end integration tests
- [ ] Documentation updates

---

## Implementation Order

```
Phase 1 (Foundation)     ████████████████████  ~2-3 sessions
     └── SQLite + Session model + State machine

Phase 2 (GitHub Client)  ████████████████      ~2 sessions
     └── API wrapper + Comment formatting

Phase 3 (Webhooks)       ████████████████      ~2 sessions
     └── Server + Event routing

Phase 4 (Orchestration)  ████████████████████  ~3 sessions
     └── Prompts + Phase handlers + Job integration

Phase 5 (Polish)         ████████████████████  ~2-3 sessions
     └── Error handling + Git push + PR creation
```

Each phase is independently testable and adds incremental value.

---

## Notes

### dworm Integration (Deferred)

The spec mentions using dworm for devcontainer management. For now, we'll continue using the existing Docker Compose approach. dworm can be integrated later by:
1. Replacing `docker.Client.ComposeUp/Down` with dworm equivalents
2. Using dworm's built-in git/gpg key handling
3. Leveraging dworm's port exposure features

### Ticket System Coexistence

The existing ticket system (`manfred ticket *`) continues to work for CLI-driven workflows. Sessions are a parallel concept specifically for GitHub-triggered work. Future work could:
- Migrate tickets to SQLite
- Allow manual ticket→session promotion
- Unify the models

### Security Considerations (From Spec)

- Webhook signature validation (Phase 3)
- Repository allowlist in config
- Prompt content sanitization (best effort)
- Container isolation (existing)
- Scoped API tokens
