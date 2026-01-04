# MANFRED - Future Vision

This document describes the complete vision for MANFRED, an agent runner service that automates coding tasks via Claude Code.

## Overview

MANFRED is designed to:
1. Listen for tickets/issues from systems like GitHub
2. Spin up isolated Docker environments for each project
3. Run Claude Code autonomously to complete the task
4. Create commits and open PRs with the results
5. Update the original ticket with status

## Architecture Principles

### The Agent Runner Never Understands the Project

MANFRED only understands:
- **Repos**: Git repositories to clone/update
- **Containers**: Docker environments to spin up
- **Issues**: Task descriptions from ticket systems
- **Diffs**: Changes made by Claude Code

This keeps it small, reusable, and project-agnostic.

### Boring, Scriptable, Unix-y

- Ruby for glue code, CLIs, SQLite, long-running services
- Docker/docker-compose per project (agent runner doesn't care about app internals)
- SQLite for job state
- Self-hosted with full trust inside containers
- GitHub-triggered (no custom UI needed initially)

## Complete Directory Structure

```
manfred/
├── bin/
│   └── manfred                   # CLI entrypoint
│
├── lib/
│   └── manfred/
│       ├── app.rb                # Boot / dependency wiring
│       ├── config.rb             # Config loading & defaults
│       │
│       ├── cli/
│       │   ├── root.rb           # CLI command dispatcher
│       │   ├── init.rb           # Setup wizard
│       │   ├── projects.rb       # Register / list projects
│       │   └── jobs.rb           # Inspect / retry jobs
│       │
│       ├── web/
│       │   ├── server.rb         # Webhook server (Sinatra/Roda)
│       │   └── routes.rb         # Webhook endpoints
│       │
│       ├── jobs/
│       │   ├── job.rb            # Job model
│       │   ├── queue.rb          # Job queue management
│       │   ├── runner.rb         # Executes a job
│       │   └── logger.rb         # Logging utilities
│       │
│       ├── git/
│       │   ├── client.rb         # Git operations
│       │   └── pr.rb             # PR creation via GitHub API
│       │
│       ├── claude/
│       │   ├── client.rb         # Claude Code invocation
│       │   └── prompt.rb         # Prompt templates
│       │
│       └── docker/
│           ├── container.rb      # Container management
│           └── compose.rb        # Compose operations
│
├── projects/
│   └── <project-name>/
│       ├── project.yml           # Metadata + settings
│       ├── secrets.env           # Encrypted or chmod 600
│       └── repository/           # Cloned git repository
│
├── jobs/
│   └── job_XXXXXX/
│       ├── job.yml               # Job metadata
│       ├── logs/                 # Execution logs
│       ├── repo/                 # Cloned repo snapshot
│       └── artifacts/            # Output files
│
├── db/
│   └── manfred.sqlite3           # Job state database
│
├── config/
│   ├── config.yml                # Global settings
│   └── credentials.env           # API keys (not in git)
│
├── scripts/
│   ├── run_job.sh                # Job execution helper
│   └── setup_container.sh        # Container setup helper
│
├── Gemfile
└── README.md
```

## Components to Build

### 1. CLI Commands

```bash
manfred init                      # Setup wizard
manfred projects add              # Register a new project
manfred projects list             # List all projects
manfred jobs list                 # List recent jobs
manfred jobs retry <job-id>       # Retry a failed job
manfred job <project> <prompt>    # Run a job (current POC)
```

### 2. Web Server (Webhooks)

Small Sinatra/Roda server that:
- Receives GitHub webhook events
- Validates webhook signatures
- Enqueues jobs
- Responds with 200 OK

No UI, no auth beyond webhook secret.

### 3. Job Queue

- SQLite-backed job queue
- Background worker loop
- Retry logic for failed jobs
- Job state persistence

### 4. Git Integration

```ruby
# lib/manfred/git/client.rb
class Git::Client
  def clone(repo_url, destination)
  def checkout(branch)
  def create_branch(name)
  def add_all
  def commit(message)
  def push(branch)
  def diff
end

# lib/manfred/git/pr.rb
class Git::PR
  def create(title:, body:, base:, head:)
  def add_comment(pr_number, comment)
end
```

### 5. GitHub Integration

Using `octokit` gem:
- Receive issue/PR webhooks
- Create branches and PRs
- Comment on issues with results
- Update issue labels/status

## Job Execution Flow (Complete)

```
┌─────────────────────────────────────────────────────────────┐
│ 1. Trigger                                                  │
│    - GitHub webhook receives new issue with "ready" label   │
│    - OR CLI command: manfred job <project> <prompt>         │
└─────────────────────────────────────────────────────────────┘
                              │
                              ▼
┌─────────────────────────────────────────────────────────────┐
│ 2. Enqueue                                                  │
│    - Create job row in SQLite (status: pending)             │
│    - Store issue text as prompt                             │
└─────────────────────────────────────────────────────────────┘
                              │
                              ▼
┌─────────────────────────────────────────────────────────────┐
│ 3. Initialize Job                                           │
│    - Create jobs/job_xxx/ directory                         │
│    - Clone/update repository                                │
│    - Create new branch: manfred/job_xxx                     │
└─────────────────────────────────────────────────────────────┘
                              │
                              ▼
┌─────────────────────────────────────────────────────────────┐
│ 4. Start Docker Environment                                 │
│    - docker compose -p manfred_<job-id> up -d               │
│    - Pass API keys and secrets                              │
│    - Mount repository at workdir                            │
└─────────────────────────────────────────────────────────────┘
                              │
                              ▼
┌─────────────────────────────────────────────────────────────┐
│ 5. Execute Claude Code (Phase 1)                            │
│    - docker exec <container> claude \                       │
│        --dangerously-skip-permissions -p "<prompt>"         │
│    - Stream and log all output                              │
└─────────────────────────────────────────────────────────────┘
                              │
                              ▼
┌─────────────────────────────────────────────────────────────┐
│ 6. Execute Claude Code (Phase 2)                            │
│    - Ask Claude to summarize and create commit message      │
│    - --continue flag to maintain context                    │
│    - Read commit message from file                          │
└─────────────────────────────────────────────────────────────┘
                              │
                              ▼
┌─────────────────────────────────────────────────────────────┐
│ 7. Git Operations                                           │
│    - git add -A                                             │
│    - git commit -m "<message>"                              │
│    - git push origin manfred/job_xxx                        │
└─────────────────────────────────────────────────────────────┘
                              │
                              ▼
┌─────────────────────────────────────────────────────────────┐
│ 8. Create PR                                                │
│    - Open PR via GitHub API                                 │
│    - Link to original issue                                 │
│    - Add summary as PR description                          │
└─────────────────────────────────────────────────────────────┘
                              │
                              ▼
┌─────────────────────────────────────────────────────────────┐
│ 9. Update Issue                                             │
│    - Comment with result (success/failure)                  │
│    - Link to PR if successful                               │
│    - Update labels                                          │
└─────────────────────────────────────────────────────────────┘
                              │
                              ▼
┌─────────────────────────────────────────────────────────────┐
│ 10. Cleanup                                                 │
│    - docker compose down                                    │
│    - Update job status in SQLite                            │
│    - Archive logs and artifacts                             │
└─────────────────────────────────────────────────────────────┘
```

## Configuration

### Global Credentials

**`config/.credentials.json`** (required):
Claude OAuth credentials from interactive login. Copied into containers at runtime.

```bash
cp ~/.claude/.credentials.json config/.credentials.json
```

**`config/credentials.env`** (optional):
```env
ANTHROPIC_API_KEY=sk-ant-...
GITHUB_TOKEN=ghp_...
WEBHOOK_SECRET=...
```

Stored globally, not per-project. Keeps agent identity consistent.

### Project Configuration (`projects/<name>/project.yml`)

```yaml
name: my-app
repo: git@github.com:you/my-app.git
default_branch: main

docker:
  compose_file: docker-compose.yml
  main_service: app
  workdir: /app

# Optional settings
claude:
  model: claude-sonnet-4-20250514

# Trigger configuration
triggers:
  github:
    labels: ["ready-for-agent"]
```

### Project Secrets (`projects/<name>/secrets.env`)

```env
DATABASE_URL=postgres://...
REDIS_URL=redis://...
RAILS_MASTER_KEY=...
```

Injected only into the job container, never exposed to the host.

## Tech Stack

Ruby gems needed:
- `thor` or `dry-cli` (CLI)
- `sqlite3` (database)
- `sequel` or `activerecord` (ORM)
- `sinatra` or `roda` (web server)
- `octokit` (GitHub API)
- `dotenv` (env file loading)

No background job framework initially - simple loop with SQLite polling.

## Implementation Phases

### Phase 1: CLI POC ✅ (Current)

- [x] CLI with `job` command
- [x] Docker compose execution
- [x] Claude Code invocation with `IS_SANDBOX=1`
- [x] Credentials copying into containers (`docker cp`)
- [x] Two-phase execution (task + commit message)
- [x] Prefixed logging
- [x] Dummy finalize

### Phase 2: Git Integration

- [ ] Git clone/pull operations
- [ ] Branch creation
- [ ] Commit and push
- [ ] Basic PR creation via `gh` CLI

### Phase 3: GitHub Webhooks

- [ ] Webhook server
- [ ] Issue event handling
- [ ] PR commenting
- [ ] Label management

### Phase 4: Job Queue

- [ ] SQLite schema
- [ ] Job persistence
- [ ] Background worker
- [ ] Retry logic

### Phase 5: Multi-project

- [ ] Project registration CLI
- [ ] Parallel job execution
- [ ] Resource limits

## Design Considerations

### Container Naming

Use unique compose project names to avoid conflicts:
```
manfred_job_20250104_101530_a1b2
```

This allows multiple jobs to run in parallel for different projects.

### Error Handling

- Docker compose fails → Log error, cleanup, mark failed
- Claude Code fails (Phase 1) → Log error, cleanup, mark failed
- Claude Code fails (Phase 2) → Log warning, continue without commit message
- Git operations fail → Log error, mark failed, keep artifacts for debugging

### Job Artifacts

Each job preserves:
- Full execution logs
- Repository state (before/after)
- Claude Code output
- Commit message
- Any generated files

Allows debugging and reproducibility.

### Security

- API keys only in credentials.env (chmod 600)
- Project secrets only passed to containers
- Webhook signature validation
- No sensitive data in logs
