# Portable Claude Code Bundle

This directory contains the build system for creating a portable Claude Code bundle that can run in any Linux container without requiring Node.js to be installed.

## What It Does

Creates a self-contained directory (~218MB) containing:
- Standalone Node.js binary (from nodejs.org official releases)
- Claude Code (`@anthropic-ai/claude-code`) with all npm dependencies
- Wrapper script that uses the bundled Node.js

## Why It's Needed

MANFRED runs Claude Code inside project Docker containers. These containers are defined by the projects themselves and typically don't have Node.js installed. Instead of requiring projects to modify their Dockerfiles, we inject a portable Claude Code bundle at runtime.

## Building

```bash
# From the manfred root directory:
make bundle

# Or directly:
cd claude-bundle && ./build.sh
```

This creates `dist/claude-bundle-linux-amd64.tar.gz`.

## Installing

```bash
# Install to ~/.manfred/ (default location)
make bundle-install

# Or manually:
mkdir -p ~/.manfred
tar -xzf claude-bundle/dist/claude-bundle-linux-amd64.tar.gz -C ~/.manfred
mv ~/.manfred/claude-bundle-linux-amd64 ~/.manfred/claude-bundle
```

## How It Works

1. **Dockerfile** builds in two stages:
   - Stage 1 (builder): Uses Node.js image to install Claude Code globally, downloads standalone Node.js binary
   - Stage 2: Copies just the bundle to a minimal image

2. **Bundle structure:**
   ```
   claude-bundle/
   ├── node           # Standalone Node.js binary (~114MB)
   ├── node_modules/  # Claude Code + dependencies (~100MB)
   │   └── @anthropic-ai/
   │       └── claude-code/
   └── claude         # Wrapper script
   ```

3. **Wrapper script** (`claude`):
   ```bash
   #!/bin/sh
   SCRIPT_DIR="$(cd "$(dirname "$0")" && pwd)"
   exec "$SCRIPT_DIR/node" "$SCRIPT_DIR/node_modules/@anthropic-ai/claude-code/cli.js" "$@"
   ```

## Runtime Usage

MANFRED automatically:
1. Copies the bundle to each job directory (`~/.manfred/jobs/job_xxx/claude-bundle/`)
2. Mounts the job directory into the container at `/manfred-job/`
3. Executes `/manfred-job/claude-bundle/claude` instead of system `claude`

## Updating Claude Code Version

Rebuild the bundle to get the latest Claude Code:

```bash
make bundle
make bundle-install
```

The Dockerfile always installs the latest version from npm.

## Platform Support

Currently builds for:
- `linux/amd64` (x86_64)

ARM64 support (`linux/arm64`) can be added by uncommenting the arm64 build in `build.sh`.
