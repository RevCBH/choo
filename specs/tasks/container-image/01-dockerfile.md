---
task: 1
status: pending
backpressure: "docker build --target builder -t test-builder ."
depends_on: []
---

# Multi-Stage Dockerfile

**Parent spec**: `/specs/CONTAINER-IMAGE.md`
**Task**: #1 of 2 in implementation plan

## Objective

Create a multi-stage Dockerfile that compiles the choo binary for linux/amd64 and produces a minimal Alpine-based runtime image with all required development tools.

## Dependencies

### Task Dependencies (within this unit)
- None

### External Dependencies
- Docker must be installed and running
- Go 1.22 base image available from Docker Hub

## Deliverables

### Files to Create/Modify
```
/
└── Dockerfile    # CREATE: Multi-stage container image definition
```

### Content

```dockerfile
# Dockerfile
# Multi-stage build for choo container image
# Build: docker build -t choo:latest .
# Run: docker run --rm choo:latest --version

# =============================================================================
# Builder stage: Compile Go binary for linux/amd64
# =============================================================================
FROM golang:1.22-alpine AS builder

# Install git for go mod download (some deps may use git)
RUN apk add --no-cache git

WORKDIR /build

# Cache dependencies
COPY go.mod go.sum ./
RUN go mod download

# Copy source and build
COPY . .
RUN CGO_ENABLED=0 GOOS=linux GOARCH=amd64 go build -o /choo ./cmd/choo

# =============================================================================
# Runtime stage: Minimal Alpine with required tools
# =============================================================================
FROM alpine:3.19

# Install runtime dependencies
# - git: Clone repositories, commit changes
# - openssh-client: SSH authentication for git operations
# - ca-certificates: HTTPS connections to APIs
# - bash: Shell scripting support
# - curl: HTTP requests, downloading Claude CLI
# - github-cli: PR creation, issue management
RUN apk add --no-cache \
    git \
    openssh-client \
    ca-certificates \
    bash \
    curl \
    github-cli

# Install Claude CLI
# Note: Installation method may need adjustment based on Claude CLI distribution
RUN curl -fsSL https://claude.ai/install.sh | sh

# Copy compiled binary from builder
COPY --from=builder /choo /usr/local/bin/choo

# Set working directory for repository operations
WORKDIR /repo

# Default entrypoint
ENTRYPOINT ["choo"]
```

### Stage Breakdown

| Stage | Base Image | Size | Purpose |
|-------|------------|------|---------|
| builder | golang:1.22-alpine | ~300MB | Compile Go binary with all dependencies |
| runtime | alpine:3.19 | ~50MB base | Minimal production image with tools |

### Installed Packages

| Package | Purpose |
|---------|---------|
| git | Clone repositories, commit changes |
| openssh-client | SSH authentication for git operations |
| ca-certificates | HTTPS connections to APIs |
| bash | Shell scripting support |
| curl | HTTP requests, downloading Claude CLI |
| github-cli | PR creation, issue management |

## Backpressure

### Validation Command
```bash
docker build --target builder -t test-builder .
```

### Success Criteria
- Dockerfile syntax is valid
- Builder stage compiles successfully
- Go binary is produced at `/choo` in builder stage

### Full Build Verification (optional)
```bash
# Full build including runtime stage
docker build -t choo:test .

# Verify binary executes
docker run --rm choo:test --version

# Verify tools are available
docker run --rm --entrypoint git choo:test --version
docker run --rm --entrypoint gh choo:test --version

# Check image size (target: < 200 MB)
docker images choo:test --format "{{.Size}}"
```

## NOT In Scope

- Build script (task 2)
- Container runtime configuration
- Volume mounts or credential injection
- Non-root user configuration (future enhancement)
- Multi-architecture builds (future enhancement)
- Image signing or verification
- Any Go code changes
