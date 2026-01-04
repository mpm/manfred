#!/bin/bash
# Build the portable Claude Code bundle

set -e

SCRIPT_DIR="$(cd "$(dirname "$0")" && pwd)"
cd "$SCRIPT_DIR"

echo "Building portable Claude Code bundle..."

# Build the Docker image
docker build --platform linux/amd64 -t manfred-claude-bundle:amd64 .
docker build --platform linux/arm64 -t manfred-claude-bundle:arm64 . || true  # arm64 optional

# Extract the bundle from the image
echo "Extracting bundle..."
rm -rf dist
mkdir -p dist

# Create a temporary container and copy files out
CONTAINER=$(docker create manfred-claude-bundle:amd64)
docker cp "$CONTAINER:/claude-bundle" dist/claude-bundle-linux-amd64
docker rm "$CONTAINER"

# Create tarball
cd dist
tar -czf claude-bundle-linux-amd64.tar.gz claude-bundle-linux-amd64
rm -rf claude-bundle-linux-amd64

echo "Bundle created: dist/claude-bundle-linux-amd64.tar.gz"

# Show size
ls -lh dist/

# Test: extract and run
echo ""
echo "Testing bundle..."
mkdir -p test
tar -xzf claude-bundle-linux-amd64.tar.gz -C test
./test/claude-bundle-linux-amd64/claude --version
rm -rf test

echo ""
echo "Done! Bundle is ready for use."
