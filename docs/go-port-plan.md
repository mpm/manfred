# MANFRED Go Port Plan

> **Status (2026-01-04):** Phase 1 complete. Core functionality working.
> See `go-implementation-notes.md` for detailed implementation documentation.

## Current Implementation Status

| Component | Status | Notes |
|-----------|--------|-------|
| CLI framework (Cobra) | ✅ Done | All commands defined |
| Config loading (Viper) | ✅ Done | YAML + env vars |
| Docker client | ✅ Done | Compose via exec, SDK for inspect |
| Job model + runner | ✅ Done | Full execution flow |
| Claude bundle | ✅ Done | Portable Node.js + Claude Code |
| Ticket model + store | ✅ Done | FileStore implementation |
| Project initializer | ✅ Done | Git clone + config generation |
| `manfred job` | ✅ Done | Working end-to-end |
| `manfred ticket *` | ✅ Done | new, list, show, stats |
| `manfred project *` | ✅ Done | init, list, show |
| `manfred ticket process` | ⏳ Pending | Ticket → Job orchestration |
| `manfred serve` | ⏳ Pending | Web server + admin UI |
| Git push / PR creation | ⏳ Pending | Currently local only |

## Overview

Port MANFRED from Ruby (containerized) + Go bridge (host) to a single Go binary.

**Benefits:**
- Single binary deployment (no Ruby runtime, no container for MANFRED itself)
- Direct Docker SDK integration (no bridge IPC)
- Built-in web server with embedded admin UI
- Cross-compilation for easy distribution
- Simpler operational model

## New Architecture

```
┌─────────────────────────────────────────────────────────────┐
│                    Host Machine                              │
│                                                              │
│  ┌──────────────────────────────────────────────────────┐   │
│  │  manfred (single Go binary)                          │   │
│  │                                                       │   │
│  │  ┌─────────┐  ┌─────────┐  ┌─────────┐  ┌─────────┐  │   │
│  │  │   CLI   │  │   API   │  │  Jobs   │  │ Docker  │  │   │
│  │  │ (cobra) │  │ (http)  │  │ Runner  │  │   SDK   │  │   │
│  │  └─────────┘  └─────────┘  └─────────┘  └─────────┘  │   │
│  │                                                       │   │
│  │  ┌─────────┐  ┌─────────┐  ┌─────────┐               │   │
│  │  │ Tickets │  │Projects │  │ Config  │               │   │
│  │  │ Store   │  │  Store  │  │  (viper)│               │   │
│  │  └─────────┘  └─────────┘  └─────────┘               │   │
│  └──────────────────────────────────────────────────────┘   │
│           │                                                  │
│           │ Docker SDK                                       │
│           ▼                                                  │
│  ┌──────────────────────────────────────────────────────┐   │
│  │                Project Container                      │   │
│  │  - Built from project's Dockerfile                   │   │
│  │  - Claude Code installed                             │   │
│  │  - Job directory mounted at /manfred-job             │   │
│  └──────────────────────────────────────────────────────┘   │
└─────────────────────────────────────────────────────────────┘
```

## Directory Structure (Go conventions)

```
manfred/
├── cmd/
│   └── manfred/
│       └── main.go              # Entry point, wires everything together
├── internal/                    # Private packages (not importable by others)
│   ├── cli/
│   │   ├── root.go              # Root command, global flags
│   │   ├── job.go               # job command
│   │   ├── ticket.go            # ticket subcommands
│   │   ├── project.go           # project subcommands
│   │   └── serve.go             # serve command (web server)
│   ├── config/
│   │   └── config.go            # Configuration loading (viper)
│   ├── docker/
│   │   └── client.go            # Docker SDK wrapper
│   ├── job/
│   │   ├── job.go               # Job model
│   │   ├── runner.go            # Job execution orchestration
│   │   └── logger.go            # Structured logging
│   ├── ticket/
│   │   ├── ticket.go            # Ticket model
│   │   ├── entry.go             # Entry model (prompt/comment)
│   │   ├── store.go             # Store interface
│   │   └── filestore.go         # Filesystem implementation
│   ├── project/
│   │   ├── project.go           # Project model
│   │   └── store.go             # Project loading/management
│   └── server/
│       ├── server.go            # HTTP server setup
│       ├── api.go               # REST API handlers
│       └── handlers.go          # Page handlers
├── web/                         # Static assets (embedded)
│   ├── static/
│   │   ├── css/
│   │   └── js/
│   └── templates/
│       ├── layout.html
│       ├── dashboard.html
│       ├── jobs.html
│       ├── tickets.html
│       └── projects.html
├── config.example.yaml          # Example config
├── go.mod
├── go.sum
├── Makefile                     # Build, test, release
├── .goreleaser.yaml             # Release automation
└── README.md
```

## Component Mapping (Ruby → Go)

### Config (`lib/manfred/config.rb` → `internal/config/config.go`)

```go
package config

type Config struct {
    DataDir      string        `mapstructure:"data_dir"`
    ProjectsDir  string        `mapstructure:"projects_dir"`
    JobsDir      string        `mapstructure:"jobs_dir"`
    TicketsDir   string        `mapstructure:"tickets_dir"`
    Credentials  Credentials   `mapstructure:"credentials"`
    Server       ServerConfig  `mapstructure:"server"`
}

type Credentials struct {
    AnthropicAPIKey     string `mapstructure:"anthropic_api_key"`
    ClaudeCredentials   string `mapstructure:"claude_credentials_file"`
}

type ServerConfig struct {
    Addr string `mapstructure:"addr"`
    Port int    `mapstructure:"port"`
}

func Load() (*Config, error)
func (c *Config) ProjectConfig(name string) (*ProjectConfig, error)
```

**Changes from Ruby:**
- Use viper for config loading (YAML + env vars + flags)
- Simpler path resolution (no dev/prod split - just use config)
- All paths configurable via single config file

### Docker Client (`lib/manfred/docker/*.rb` → `internal/docker/client.go`)

```go
package docker

import (
    "github.com/docker/docker/client"
)

type Client struct {
    docker *client.Client
}

func New() (*Client, error)

// Compose operations
func (c *Client) ComposeUp(ctx context.Context, opts ComposeOptions) error
func (c *Client) ComposeDown(ctx context.Context, projectName string) error

// Container operations
func (c *Client) Exec(ctx context.Context, container string, cmd []string, opts ExecOptions) error
func (c *Client) ExecCapture(ctx context.Context, container string, cmd []string) (string, error)
func (c *Client) CopyToContainer(ctx context.Context, container, src, dst string) error
func (c *Client) IsRunning(ctx context.Context, container string) (bool, error)

// Git operations (run on host, not in container)
func (c *Client) CloneRepo(ctx context.Context, url, dest, branch string) error
```

**Changes from Ruby:**
- Direct Docker SDK instead of shelling out or bridge
- No bridge_client.rb / direct_client.rb split
- Compose operations via docker/compose SDK or exec

### Job (`lib/manfred/jobs/*.rb` → `internal/job/`)

```go
package job

type Job struct {
    ID           string
    ProjectName  string
    Prompt       string
    Status       Status
    CreatedAt    time.Time
    StartedAt    *time.Time
    CompletedAt  *time.Time
    Error        string
    CommitMsg    string
    BranchName   string
    BaseSHA      string
}

type Status string
const (
    StatusPending   Status = "pending"
    StatusRunning   Status = "running"
    StatusCompleted Status = "completed"
    StatusFailed    Status = "failed"
)

type Runner struct {
    config *config.Config
    docker *docker.Client
    logger *Logger
}

func NewRunner(cfg *config.Config) (*Runner, error)
func (r *Runner) Run(ctx context.Context, projectName, prompt string) (*Job, error)
```

**Changes from Ruby:**
- Job state could optionally be persisted to SQLite (future)
- Context-based cancellation
- Structured logging with slog

### Ticket (`lib/manfred/tickets/*.rb` → `internal/ticket/`)

```go
package ticket

type Ticket struct {
    ID        string
    Project   string
    Status    Status
    CreatedAt time.Time
    JobID     string
    Entries   []Entry
}

type Entry struct {
    Type      EntryType
    Author    string
    Timestamp time.Time
    Content   string
}

// Store interface for different backends
type Store interface {
    List(ctx context.Context, status *Status) ([]Ticket, error)
    Get(ctx context.Context, id string) (*Ticket, error)
    Create(ctx context.Context, project, prompt string) (*Ticket, error)
    Update(ctx context.Context, ticket *Ticket) error
    Stats(ctx context.Context) (map[Status]int, error)
    NextPending(ctx context.Context) (*Ticket, error)
}

// FileStore implements Store using filesystem
type FileStore struct {
    baseDir string
}
```

**Changes from Ruby:**
- Store interface allows future backends (SQLite, PostgreSQL)
- Same YAML format for compatibility during migration

### CLI (`lib/manfred/cli/*.rb` → `internal/cli/`)

```go
// internal/cli/root.go
package cli

func NewRootCmd() *cobra.Command {
    root := &cobra.Command{
        Use:   "manfred",
        Short: "Claude Code agent runner",
    }

    root.AddCommand(
        newJobCmd(),
        newTicketCmd(),
        newProjectCmd(),
        newServeCmd(),
        newVersionCmd(),
    )

    return root
}

// internal/cli/job.go
func newJobCmd() *cobra.Command {
    return &cobra.Command{
        Use:   "job <project> <prompt-file>",
        Short: "Run a job",
        RunE:  runJob,
    }
}

// internal/cli/ticket.go
func newTicketCmd() *cobra.Command {
    cmd := &cobra.Command{
        Use:   "ticket",
        Short: "Ticket management",
    }
    cmd.AddCommand(
        newTicketNewCmd(),
        newTicketListCmd(),
        newTicketShowCmd(),
        newTicketStatsCmd(),
        newTicketProcessCmd(),
    )
    return cmd
}
```

### Web Server (`new` → `internal/server/`)

```go
package server

//go:embed web/static web/templates
var webFS embed.FS

type Server struct {
    config  *config.Config
    runner  *job.Runner
    tickets ticket.Store
    router  *http.ServeMux
}

func New(cfg *config.Config) (*Server, error)
func (s *Server) Start(ctx context.Context) error

// API endpoints
// GET  /api/jobs           - list jobs
// GET  /api/jobs/:id       - get job
// POST /api/jobs           - create job
// GET  /api/tickets        - list tickets
// POST /api/tickets        - create ticket
// POST /api/tickets/:id/process - process ticket
// GET  /api/projects       - list projects
```

## Migration Steps

### Phase 1: Core Infrastructure
1. Set up Go module and directory structure
2. Implement config loading with viper
3. Implement Docker client with SDK
4. Port job model and basic runner (no Claude yet)

### Phase 2: Job Execution
5. Port job runner with Claude Code execution
6. Port logging system
7. Port git clone/branch logic
8. Test end-to-end job execution

### Phase 3: Ticket System
9. Port ticket model and entry types
10. Port file-based ticket store
11. Implement ticket processor

### Phase 4: CLI
12. Set up Cobra root command
13. Port job command
14. Port ticket subcommands
15. Port project subcommands

### Phase 5: Web Server
16. Set up basic HTTP server
17. Implement REST API
18. Create admin UI templates
19. Embed static assets

### Phase 6: Polish
20. Add graceful shutdown
21. Add health checks
22. Create Makefile and release automation
23. Update documentation
24. Create migration guide

## Config File Format (new)

```yaml
# config.yaml
data_dir: /var/lib/manfred      # Base directory for all data

# Can override individual paths
projects_dir: /var/lib/manfred/projects
jobs_dir: /var/lib/manfred/jobs
tickets_dir: /var/lib/manfred/tickets

credentials:
  anthropic_api_key: ${ANTHROPIC_API_KEY}  # env var expansion
  claude_credentials_file: /var/lib/manfred/config/.credentials.json

server:
  addr: 127.0.0.1
  port: 8080

logging:
  level: info    # debug, info, warn, error
  format: text   # text, json
```

## Dependencies

```go
// go.mod
module github.com/mpm/manfred

go 1.22

require (
    github.com/spf13/cobra v1.8.0      // CLI framework
    github.com/spf13/viper v1.18.0     // Configuration
    github.com/docker/docker v25.0.0   // Docker SDK
    gopkg.in/yaml.v3 v3.0.1            // YAML parsing
)
```

## Build and Release

```makefile
# Makefile
VERSION ?= $(shell git describe --tags --always)
LDFLAGS := -ldflags "-X main.version=$(VERSION)"

.PHONY: build
build:
	go build $(LDFLAGS) -o bin/manfred ./cmd/manfred

.PHONY: release
release:
	GOOS=linux GOARCH=amd64 go build $(LDFLAGS) -o dist/manfred-linux-amd64 ./cmd/manfred
	GOOS=linux GOARCH=arm64 go build $(LDFLAGS) -o dist/manfred-linux-arm64 ./cmd/manfred
	GOOS=darwin GOARCH=amd64 go build $(LDFLAGS) -o dist/manfred-darwin-amd64 ./cmd/manfred
	GOOS=darwin GOARCH=arm64 go build $(LDFLAGS) -o dist/manfred-darwin-arm64 ./cmd/manfred

.PHONY: test
test:
	go test -v ./...
```

## What Gets Simpler

1. **Deployment**: One binary, copy anywhere, run
2. **No bridge**: Direct Docker SDK calls
3. **No container for MANFRED**: Runs directly on host
4. **Credentials**: Just point config at file, no symlink dance
5. **Web server**: Built-in, no separate process
6. **Cross-platform**: Easy to build for any OS/arch

## What Gets More Verbose

1. **Error handling**: Every call returns error
2. **Struct definitions**: Explicit types everywhere
3. **No Thor magic**: Manual flag/arg parsing (but Cobra helps)

## Estimated Effort

| Phase | Components | Complexity |
|-------|------------|------------|
| 1 | Config, Docker client | Low |
| 2 | Job runner | Medium |
| 3 | Ticket system | Low |
| 4 | CLI commands | Low |
| 5 | Web server + UI | Medium |
| 6 | Polish | Low |

Total: ~2500-3000 lines of Go (vs ~2100 Ruby + ~575 Go bridge)

## Files to Delete After Port

- `lib/` (all Ruby code)
- `spec/` (Ruby tests)
- `bridge/` (Go bridge - merged into main)
- `docker/` (MANFRED Dockerfile - no longer needed)
- `exe/` (Ruby entrypoint)
- `Gemfile`, `Gemfile.lock`, `manfred.gemspec`
- `Rakefile`

## Files to Keep/Update

- `projects/` (project configs - same format)
- `tickets/` (ticket YAML files - same format)
- `jobs/` (job artifacts)
- `config/` (rename config.yml, update format)
- `docs/` (update for Go)
- `.github/workflows/` (update for Go build)
- `misc/installer.sh` (simplify - just download binary)
