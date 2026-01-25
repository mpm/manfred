# MANFRED - Claude Code Agent Runner

MANFRED is a CLI tool that orchestrates Claude Code to work on coding tasks inside Docker containers.

## Project Overview

MANFRED automates coding workflows by:
1. Managing tickets (task prompts) via CLI
2. Starting a project's Docker environment
3. Running Claude Code inside the container with full permissions
4. Collecting commit messages summarizing the changes
5. (Future) Creating commits and opening PRs automatically

## Architecture

```
┌─────────────────────────────────────────────────────────────┐
│                    Host Machine                              │
│                                                              │
│  ┌──────────────────────────────────────────────────────┐   │
│  │  manfred (single Go binary)                          │   │
│  │                                                       │   │
│  │  ┌─────────┐  ┌─────────┐  ┌─────────┐  ┌─────────┐  │   │
│  │  │   CLI   │  │  Jobs   │  │ Tickets │  │ Docker  │  │   │
│  │  │ (cobra) │  │ Runner  │  │  Store  │  │   SDK   │  │   │
│  │  └─────────┘  └─────────┘  └─────────┘  └─────────┘  │   │
│  └──────────────────────────────────────────────────────┘   │
│           │                                                  │
│           │ Docker SDK                                       │
│           ▼                                                  │
│  ┌──────────────────────────────────────────────────────┐   │
│  │                Project Container                      │   │
│  │  - Built from project's Dockerfile                   │   │
│  │  - Claude Code installed via npm                     │   │
│  │  - Job directory mounted at /manfred-job             │   │
│  └──────────────────────────────────────────────────────┘   │
└─────────────────────────────────────────────────────────────┘
```

**Key principle**: MANFRED runs as a single binary on the host and uses Docker to run Claude Code in isolated project containers.

## Directory Structure

```
manfred/
├── cmd/
│   └── manfred/
│       └── main.go              # CLI entrypoint
├── internal/                    # Private packages
│   ├── cli/
│   │   ├── root.go              # Cobra CLI dispatcher
│   │   ├── job.go               # 'job' command
│   │   ├── ticket.go            # 'ticket' subcommands
│   │   ├── session.go           # 'session' subcommands (GitHub sessions)
│   │   ├── github.go            # 'github' subcommands (test-auth, webhook-url)
│   │   ├── project.go           # 'project' subcommands
│   │   └── serve.go             # 'serve' command (web server, future)
│   ├── config/
│   │   └── config.go            # Configuration loading (viper)
│   ├── docker/
│   │   └── client.go            # Docker SDK wrapper
│   ├── store/
│   │   ├── sqlite.go            # SQLite connection manager (WAL mode)
│   │   └── migrations.go        # Schema migrations
│   ├── session/
│   │   ├── phase.go             # Phase enum and state machine
│   │   ├── session.go           # Session model for GitHub workflows
│   │   └── store.go             # SQLiteStore implementation
│   ├── github/
│   │   ├── client.go            # GitHub API client (HTTP, auth, rate limiting)
│   │   ├── types.go             # API types (Issue, Comment, PullRequest, etc.)
│   │   ├── issues.go            # Issue operations
│   │   ├── pulls.go             # Pull request operations
│   │   ├── comments.go          # Comment formatting/parsing helpers
│   │   └── webhooks.go          # Webhook signature validation, event parsing
│   ├── job/
│   │   ├── job.go               # Job model
│   │   ├── runner.go            # Job execution orchestration
│   │   └── logger.go            # Prefixed stdout logging
│   ├── ticket/
│   │   ├── ticket.go            # Ticket model
│   │   ├── store.go             # FileStore implementation
│   │   └── processor.go         # Ticket → Job orchestration
│   └── project/
│       └── initializer.go       # Project setup
├── web/                         # Static assets (future)
│   ├── static/
│   └── templates/
├── dev/                         # Development fixtures
│   ├── projects/
│   │   └── example-project/     # Example project for testing
│   ├── config/
│   │   ├── manfred.yaml         # Development configuration
│   │   └── .credentials.json    # Claude credentials (not committed)
│   └── prompts/
│       └── test-prompt.txt      # Example prompt
├── config/
│   └── config.example.yaml      # Configuration template
├── docs/                        # Documentation
├── jobs/                        # Runtime job artifacts
├── go.mod                       # Go module definition
├── go.sum                       # Go dependency checksums
├── Makefile                     # Build and release tasks
└── CHANGELOG.md                 # Version history
```

## CLI Commands

```bash
# Job execution (direct prompt file)
manfred job <project-name> <prompt-file>

# Project management
manfred project init <name> --repo <git-url>  # Clone repo, generate project.yml
manfred project list                          # List all projects
manfred project show <name>                   # Show project config

# Ticket management (CLI-driven workflows)
manfred ticket new <project> [prompt]         # Create ticket (or read stdin)
manfred ticket list <project> [--status X]    # List tickets
manfred ticket show <project> <ticket-id>     # Show ticket details
manfred ticket stats [project]                # Count by status
manfred ticket process <project> [ticket-id]  # Process next/specific ticket

# Session management (GitHub-driven workflows)
manfred session list [--repo X] [--phase X] [--active]  # List sessions
manfred session show <session-id> [--events]            # Show session details
manfred session delete <session-id>                     # Delete a session
manfred session stats                                   # Count by phase

# GitHub integration
manfred github test-auth                                # Verify GitHub credentials
manfred github webhook-url                              # Print webhook URL for setup

# Utilities
manfred version
manfred help
```

## Job Execution Flow

1. **Initialize**: Create job directory, read prompt, load project config
2. **Git Clone** (optional): If `repo:` set in project.yml, clone to job workspace
3. **Prepare**: Write credentials and prompt to job directory
4. **Docker Start**: Run `docker compose` with job directory mounted at `/manfred-job`
5. **Setup**: Create symlinks for credentials inside container
6. **Phase 1**: Execute Claude Code with the main task prompt
7. **Phase 2**: Ask Claude to summarize changes and write commit message
8. **Verify**: Check git state (branch, uncommitted changes, commits made)
9. **Finalize**: Read commit message, log what would happen (push/PR deferred)
10. **Cleanup**: Stop and remove containers

## Ticket System

Tickets are task prompts stored as YAML files, organized by status:

```
tickets/<project>/
├── pending/           # Waiting to be processed
├── in_progress/       # Currently being worked on
├── error/             # Job failed
└── completed/         # Successfully processed
```

**Ticket lifecycle:**
1. Create via `ticket new` → status: `pending`
2. Process via `ticket process` → creates job, status: `in_progress`
3. Job completes → status: `completed` or `error`

**Ticket YAML format:**
```yaml
id: ticket_20260104_123456_abcd
project: example-project
created_at: 2026-01-04T12:34:56Z
status: pending
job_id: ""
entries:
  - type: prompt
    author: user
    timestamp: 2026-01-04T12:34:56Z
    content: |
      Add a new feature...
```

## Configuration

**Config file** (`~/.manfred/config.yaml` or `--config`):

```yaml
data_dir: ~/.manfred

# Override individual paths
projects_dir: ~/.manfred/projects
jobs_dir: ~/.manfred/jobs
tickets_dir: ~/.manfred/tickets

database:
  path: ~/.manfred/manfred.db    # SQLite database for sessions

credentials:
  anthropic_api_key: ${ANTHROPIC_API_KEY}
  claude_credentials_file: ~/.manfred/config/.credentials.json

github:
  token: ${GITHUB_TOKEN}         # Personal Access Token
  webhook_secret: ""             # Webhook signature secret
  rate_limit_buffer: 100         # Stop when this many requests remain

server:
  addr: 127.0.0.1
  port: 8080

logging:
  level: info
  format: text
```

**Environment variables:**
- `ANTHROPIC_API_KEY` - Anthropic API key
- `GITHUB_TOKEN` - GitHub Personal Access Token
- `MANFRED_WEBHOOK_SECRET` - GitHub webhook signature secret
- `MANFRED_DATA_DIR` - Base data directory
- `MANFRED_PROJECTS_DIR` - Projects directory
- `MANFRED_JOBS_DIR` - Jobs directory
- `MANFRED_TICKETS_DIR` - Tickets directory
- `MANFRED_DATABASE_PATH` - SQLite database path

**Claude Credentials:**

Copy your Claude Code credentials from your local machine:

```bash
cp ~/.claude/.credentials.json ~/.manfred/config/.credentials.json
```

**Project Config** (`projects/<name>/project.yml`):

```yaml
name: example-project
repo: git@github.com:you/example-project.git
default_branch: main

docker:
  compose_file: docker-compose.yml
  main_service: app
  workdir: /app
```

## Development

```bash
# Build
make build

# Run with dev config
./bin/manfred --config dev/config/manfred.yaml job example-project dev/prompts/test-prompt.txt

# Run tests
make test

# Format and lint
make lint

# Build release binaries
make release
```

## Key Design Decisions

1. **Single binary**: No runtime dependencies, easy deployment.

2. **Direct Docker SDK**: No bridge process needed, simpler architecture.

3. **Host execution**: MANFRED runs on the host, not in a container.

4. **Docker Compose per project**: Each project brings its own `docker-compose.yml`.

5. **Unique compose project names**: Jobs use `manfred_<job-id>` for parallel runs.

6. **Two-phase Claude execution**:
   - Phase 1: Main task with user's prompt
   - Phase 2: `--continue` to summarize changes and create commit message

7. **Git branch per job** (optional): When `repo:` is configured, MANFRED clones
   the repository and creates a feature branch for Claude to work on.

8. **Viper configuration**: Unified config from files, environment, and flags.

## Session System (GitHub Integration)

Sessions track GitHub-triggered workflows with a phase-based state machine:

```
planning → awaiting_approval → implementing → in_review ⟷ revising → completed
    ↓             ↓                ↓              ↓           ↓
  error ←←←←←←←←←←←←←←←←←←←←←←←←←←←←←←←←←←←←←←←←←←←←←←←←←←←←←┘
```

**Session model** (`internal/session/session.go`):
- `ID`: `{owner}-{repo}-issue-{number}`
- `Phase`: Current workflow state
- `Branch`: `claude/issue-{number}`
- `PlanContent`: Claude's implementation plan
- `PRNumber`: Set after PR creation

**SQLite tables** (`internal/store/migrations.go`):
- `sessions`: Session state and metadata
- `session_events`: Audit log (phase changes, comments, errors)
- `schema_migrations`: Migration tracking

Sessions are separate from tickets. Tickets are for CLI-driven workflows (YAML files);
sessions are for GitHub-driven workflows (SQLite).

## GitHub Client (`internal/github/`)

HTTP client for GitHub API with PAT authentication and rate limiting.

**Key types** (`types.go`):
- `Issue`, `Comment`, `ReviewComment`, `PullRequest`, `User`, `Label`

**Client operations:**
```go
client := github.NewClient(token, github.WithRateLimitBuffer(100))

// Issues
issue, _ := client.GetIssue(ctx, "owner", "repo", 42)
comments, _ := client.GetIssueComments(ctx, "owner", "repo", 42)
client.AddIssueComment(ctx, "owner", "repo", 42, "Comment body")
client.AddLabel(ctx, "owner", "repo", 42, "claude")

// Pull Requests
pr, _ := client.CreatePullRequest(ctx, "owner", "repo", &github.CreatePullRequestInput{
    Title: "PR title", Body: "PR body", Head: "feature-branch", Base: "main",
})
client.GetPRReviewComments(ctx, "owner", "repo", 1)
```

**Comment helpers** (`comments.go`):
```go
// Format comments with session metadata
body := github.FormatPlanComment("session-id", "Implementation plan...")

// Parse Manfred comments
meta := github.ParseManfredComment(body) // Returns *ManfredMeta{SessionID, Phase}

// Detect approvals/retries
github.IsApproval("@claude approved")  // true
github.IsRetryRequest("@claude retry") // true
```

**Webhook validation** (`webhooks.go`):
```go
// Validate signature (X-Hub-Signature-256 header)
err := github.ValidateWebhookSignature(payload, signature, secret)

// Parse events
event, _ := github.ParseWebhookEvent("issues", payload)
issueEvent, _ := event.AsIssueEvent()
```

## What's NOT Implemented Yet

- Web server with admin UI (`manfred serve`)
- Git push and PR creation
- GitHub webhook server and event routing (Phase 3)
- Prompt builder for phase-specific prompts (Phase 4)
- Session orchestrator connecting phases to job runner (Phase 4)

See `docs/github-integration-plan.md` for the full implementation roadmap.

## Release Process

```bash
# 1. Update CHANGELOG.md with release notes

# 2. Commit and tag
git add CHANGELOG.md
git commit -m 'Release v0.2.0'
git tag v0.2.0

# 3. Push (triggers automated release)
git push origin main v0.2.0
```

## Installation

```bash
# Download binary for your platform
curl -LO https://github.com/mpm/manfred/releases/download/v0.2.0/manfred-linux-amd64
chmod +x manfred-linux-amd64
sudo mv manfred-linux-amd64 /usr/local/bin/manfred

# Set up directories
mkdir -p ~/.manfred/{projects,jobs,config,tickets}
cp ~/.claude/.credentials.json ~/.manfred/config/.credentials.json

# Run
manfred job my-project prompt.txt
```

## Dependencies

**Runtime:**
- Docker with Compose v2

**Build:**
- Go >= 1.24

**Key Go dependencies:**
- `github.com/spf13/cobra` - CLI framework
- `github.com/spf13/viper` - Configuration
- `github.com/docker/docker` - Docker SDK
- `modernc.org/sqlite` - Pure Go SQLite (no CGO)
