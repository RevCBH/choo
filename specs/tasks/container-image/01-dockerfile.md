---
task: 1
status: complete
backpressure: "docker build --target builder -t test-builder ."
depends_on: []
---

# Multi-Stage Dockerfile

**Parent spec**: Container Image Setup
**Task**: #1 - Multi-Stage Dockerfile

## Objective

Create a multi-stage Dockerfile for the choo Go application that separates the build environment from the runtime environment, resulting in a minimal production image.

## Dependencies

### Task Dependencies (within this unit)
- None

### External Dependencies
- Docker must be installed and running

## Deliverables

### Files to Create/Modify
```
Dockerfile    # CREATE: Multi-stage Dockerfile
```

### Content

```dockerfile
# Build stage
FROM golang:1.24-alpine AS builder

WORKDIR /app

# Install git for go mod download (some dependencies may use git)
RUN apk add --no-cache git

# Copy go mod files first for better layer caching
COPY go.mod go.sum ./
RUN go mod download

# Copy source code
COPY . .

# Build the binary with optimizations
RUN CGO_ENABLED=0 GOOS=linux go build -ldflags="-s -w" -o /choo ./cmd/choo

# Runtime stage
FROM alpine:3.21

# Add ca-certificates for HTTPS and git for runtime git operations
RUN apk add --no-cache ca-certificates git

WORKDIR /app

# Copy the binary from builder
COPY --from=builder /choo /usr/local/bin/choo

# Run as non-root user
RUN adduser -D -u 1000 choo
USER choo

ENTRYPOINT ["choo"]
```

## Backpressure

### Validation Command
```bash
docker build --target builder -t test-builder .
```

### Success Criteria
- Dockerfile exists in the repository root
- Builder stage compiles successfully
- Binary is produced at `/choo` in the builder stage

## NOT In Scope
- Docker Compose configuration
- CI/CD pipeline integration for container builds
- Container registry push configuration
- Kubernetes deployment manifests
