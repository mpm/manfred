# Build a portable Claude Code bundle
# This creates a self-contained directory with Node.js + Claude Code
# that can be injected into any Linux container

FROM node:22-bookworm-slim AS builder

# Install Claude Code globally to get all dependencies
RUN npm install -g @anthropic-ai/claude-code

# Download standalone Node.js binary
ARG NODE_VERSION=22.12.0
ARG TARGETARCH

RUN apt-get update && apt-get install -y curl xz-utils \
    && ARCH=$([ "$TARGETARCH" = "arm64" ] && echo "arm64" || echo "x64") \
    && curl -fsSL "https://nodejs.org/dist/v${NODE_VERSION}/node-v${NODE_VERSION}-linux-${ARCH}.tar.xz" \
       -o /tmp/node.tar.xz \
    && mkdir -p /tmp/node \
    && tar -xJf /tmp/node.tar.xz -C /tmp/node --strip-components=1

# Create the portable bundle
RUN mkdir -p /bundle \
    && cp /tmp/node/bin/node /bundle/node \
    && cp -r /usr/local/lib/node_modules /bundle/node_modules

# Create wrapper script
RUN cat > /bundle/claude <<'EOF'
#!/bin/sh
SCRIPT_DIR="$(cd "$(dirname "$0")" && pwd)"
exec "$SCRIPT_DIR/node" "$SCRIPT_DIR/node_modules/@anthropic-ai/claude-code/cli.js" "$@"
EOF
RUN chmod +x /bundle/claude /bundle/node

# Verify it works
RUN /bundle/claude --version

# Final stage: minimal image for extraction
FROM busybox:latest
COPY --from=builder /bundle /claude-bundle
CMD ["true"]
