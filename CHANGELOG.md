# Changelog

All notable changes to this project will be documented in this file.

The format is based on [Keep a Changelog](https://keepachangelog.com/en/1.1.0/),
and this project adheres to [Semantic Versioning](https://semver.org/spec/v2.0.0.html).

## [Unreleased]

## [0.1.1] - 2026-01-04

Initial proof-of-concept release. This version demonstrates the core workflow but
should be considered experimental. Error handling is minimal and many edge cases
are not yet covered.

### Added

- Single Go binary CLI (`manfred`) with Cobra-based commands
- Job execution with two-phase Claude Code orchestration
  - Phase 1: Execute user prompt
  - Phase 2: Request commit message summary
- Docker Compose integration for project containers
  - Dynamic volume injection for job directories
  - Credential symlink setup inside containers
- Local filesystem ticket system
  - Create tickets from CLI or stdin
  - FIFO processing of pending tickets
  - Status tracking (pending → in_progress → completed/error)
- Project management
  - Initialize projects from Git repositories
  - Per-project `project.yml` configuration
- Configuration via Viper (YAML files + environment variables + flags)
- Structured logging with source prefixes ([MANFRED], [DOCKER], [CLAUDE])

### Known Limitations

- No web server or admin UI yet (`manfred serve` is stubbed)
- Git push and PR creation not implemented
- No job queue or background processing
- Error recovery is basic
- Only tested on Linux

### Infrastructure

- Go 1.22+ with modules
- Cross-platform release builds (linux/darwin, amd64/arm64)
- GitHub Actions for CI and releases
