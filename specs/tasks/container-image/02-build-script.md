---
task: 2
status: pending
backpressure: "./scripts/build-image.sh && docker images choo:latest"
depends_on: [1]
---

# Build Image Script

**Parent spec**: `/specs/CONTAINER-IMAGE.md`
**Task**: #2 of 2 in implementation plan

## Objective

Create a build script that handles cross-compilation from any host platform (including macOS) and invokes Docker to build the container image with optional version tagging.

## Dependencies

### Task Dependencies (within this unit)
- Task 1 (Dockerfile) must be complete

### External Dependencies
- Docker must be installed and running
- Go 1.22+ must be installed for cross-compilation

## Deliverables

### Files to Create/Modify
```
scripts/
└── build-image.sh    # CREATE: Cross-compile and build image script
```

### Content

```bash
#!/bin/bash
# scripts/build-image.sh
# Build the choo container image with cross-compilation support
#
# Usage:
#   ./scripts/build-image.sh           # Build with 'latest' tag
#   ./scripts/build-image.sh v0.4.0    # Build with specific version tag
#
# This script:
# 1. Cross-compiles choo for linux/amd64 (works from macOS or Linux)
# 2. Builds the Docker image using the multi-stage Dockerfile
# 3. Tags the image with the specified version (default: latest)

set -e

VERSION=${1:-latest}

echo "Building choo container image..."
echo "  Version: ${VERSION}"
echo "  Target:  linux/amd64"
echo ""

# Ensure we're in the repo root
SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
REPO_ROOT="$(cd "${SCRIPT_DIR}/.." && pwd)"
cd "${REPO_ROOT}"

# Create bin directory if it doesn't exist
mkdir -p bin

# Cross-compile for linux/amd64
# CGO_ENABLED=0 ensures pure Go compilation without C dependencies
# This is critical for cross-compilation from macOS to Linux
echo "Cross-compiling choo for linux/amd64..."
CGO_ENABLED=0 GOOS=linux GOARCH=amd64 go build -o bin/choo-linux-amd64 ./cmd/choo

echo "Binary compiled: bin/choo-linux-amd64"
echo ""

# Build Docker image
echo "Building Docker image..."
docker build -t "choo:${VERSION}" .

# Also tag as latest if building a specific version
if [ "${VERSION}" != "latest" ]; then
    docker tag "choo:${VERSION}" "choo:latest"
    echo "Tagged choo:${VERSION} as choo:latest"
fi

echo ""
echo "Build complete!"
echo ""
docker images choo --format "table {{.Repository}}\t{{.Tag}}\t{{.Size}}\t{{.CreatedAt}}"
```

### Usage Examples

```bash
# Build with default 'latest' tag
./scripts/build-image.sh

# Build with specific version tag
./scripts/build-image.sh v0.4.0

# Verify image was created
docker images choo

# Test the built image
docker run --rm choo:latest --version
```

## Backpressure

### Validation Command
```bash
./scripts/build-image.sh && docker images choo:latest
```

### Success Criteria
- Script is executable
- Cross-compilation produces `bin/choo-linux-amd64` binary
- Docker image `choo:latest` is created
- Image appears in `docker images` output

### Extended Verification
```bash
# Verify image contents
docker run --rm choo:latest --version
docker run --rm --entrypoint git choo:latest --version
docker run --rm --entrypoint gh choo:latest --version
docker run --rm --entrypoint claude choo:latest --version

# Check image size (target: < 200 MB)
docker images choo:latest --format "{{.Size}}"

# Test version tagging
./scripts/build-image.sh v0.5.0
docker images choo --format "{{.Tag}}"
# Should show: v0.5.0, latest
```

## NOT In Scope

- Dockerfile modifications (task 1)
- Multi-architecture builds (linux/arm64)
- Image push to registry (local-only per PRD)
- Build caching optimization
- CI/CD integration (local-only feature)
- Container runtime or execution
- Any Go code changes
