# MANFRED - **M**ultiple **A**gent **N**odes **F**reeing **R**esources from **E**ngineering **D**istraction

> ⚠️ **Early Development** — Barely usable; will likely break if your setup differs from mine.

MANFRED runs Claude Code in isolated Docker containers, letting you:

- Grant full agent permissions safely (sandboxed)
- Work multiple branches in parallel (each containerized)
- Trigger runs via GitHub issues or prompt files
- Get auto-generated commits *(PRs coming soon)*

**Setup:** Just Docker + the `manfred` binary.

## Installation

### Download Binary

Download the latest release from [GitHub Releases](https://github.com/mpm/manfred/releases):

```bash
# For Linux x86_64:
curl -LO https://github.com/mpm/manfred/releases/download/v0.2.0/manfred-linux-amd64
chmod +x manfred-linux-amd64
sudo mv manfred-linux-amd64 /usr/local/bin/manfred

# For Linux ARM64:
curl -LO https://github.com/mpm/manfred/releases/download/v0.2.0/manfred-linux-arm64
chmod +x manfred-linux-arm64
sudo mv manfred-linux-arm64 /usr/local/bin/manfred

# For macOS:
curl -LO https://github.com/mpm/manfred/releases/download/v0.2.0/manfred-darwin-arm64
chmod +x manfred-darwin-arm64
sudo mv manfred-darwin-arm64 /usr/local/bin/manfred
```

### Set Up

```bash
# Create directories
mkdir -p ~/.manfred/{projects,jobs,config,tickets}

# Copy Claude credentials (required)
cp ~/.claude/.credentials.json ~/.manfred/config/.credentials.json

# Verify installation
manfred version
```

### Build from Source

```bash
git clone https://github.com/mpm/manfred.git
cd manfred
make build

# Run directly
./bin/manfred help
```

## Quick Start

### 1. Initialize a project

```bash
manfred project init my-project --repo git@github.com:you/my-project.git
```

### 2. Create a ticket

```bash
manfred ticket new my-project "Add a logout button to the navbar"
```

### 3. Process the ticket

```bash
manfred ticket process my-project
```

Or run a job directly with a prompt file:

```bash
manfred job my-project prompts/my-prompt.txt
```

## CLI Commands

```bash
# Job execution
manfred job <project> <prompt-file>

# Project management
manfred project init <name> --repo <git-url>
manfred project list
manfred project show <name>

# Ticket management
manfred ticket new <project> [prompt]
manfred ticket list <project> [--status pending]
manfred ticket show <project> <ticket-id>
manfred ticket process <project> [ticket-id]
manfred ticket stats [project]

# Utilities
manfred version
manfred help
```

## Configuration

Config file location: `~/.manfred/config.yaml` (or use `--config`)

```yaml
data_dir: ~/.manfred

credentials:
  claude_credentials_file: ~/.manfred/config/.credentials.json

# Optional overrides
projects_dir: ~/.manfred/projects
jobs_dir: ~/.manfred/jobs
tickets_dir: ~/.manfred/tickets
```

Environment variables:
- `ANTHROPIC_API_KEY` - Anthropic API key
- `MANFRED_DATA_DIR` - Base data directory

## Project Configuration

Each project needs a `project.yml` file:

```yaml
name: my-project
repo: git@github.com:you/my-project.git
default_branch: main

docker:
  compose_file: docker-compose.yml
  main_service: app
  workdir: /app
```

## Log Output

MANFRED outputs logs with prefixed sources:

```
[2025-01-04T10:15:30] [MANFRED ] Starting job job_20250104_101530_a1b2
[2025-01-04T10:15:31] [DOCKER  ] Starting docker compose...
[2025-01-04T10:15:46] [CLAUDE  ] Analyzing the codebase...
```

## Architecture

See [CLAUDE.md](CLAUDE.md) for detailed architecture documentation.

## Requirements

- Docker with Compose v2
- Go 1.22+ (for building from source)

## Origin of the name

Named after Manfred Macx from Charles Stross's *Accelerando*, who distributed
his consciousness into a flock of doves to parallelize his existence—only to
find the fragmented attention overwhelming.

I found myself in a similar situation: juggling multiple coding agent sessions,
trapped in constant context switching.

MANFRED decouples you from this interaction. Dump a batch of implementation
tickets whenever convenient, then review the results when you're ready.

## License

MIT
