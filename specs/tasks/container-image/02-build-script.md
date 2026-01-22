---
task: 2
status: complete
backpressure: "./scripts/build-image.sh && docker images choo:latest"
depends_on: [1]
---

# Build Image Script

**Parent spec**: `/specs/CONTAINER-IMAGE.md`
**Task**: #2 - Build Image Script

## Objective

Create a shell script that builds the Docker image for the choo application with proper tagging and optional version support.

## Dependencies

### Task Dependencies (within this unit)
- Task #1: Multi-Stage Dockerfile must be complete

### External Dependencies
- Docker must be installed and running
- Dockerfile must exist in repository root

## Deliverables

### Files to Create/Modify
```
scripts/
└── build-image.sh    # CREATE: Build script for Docker image
```

### Content

```bash
#!/bin/bash
set -euo pipefail

# Build the choo Docker image
# Usage: ./scripts/build-image.sh [TAG]

TAG="${1:-latest}"
IMAGE_NAME="choo"

echo "Building ${IMAGE_NAME}:${TAG}..."

docker build \
    -t "${IMAGE_NAME}:${TAG}" \
    -f Dockerfile \
    .

echo "Successfully built ${IMAGE_NAME}:${TAG}"
```

## Backpressure

### Validation Command
```bash
./scripts/build-image.sh && docker images choo:latest
```

### Success Criteria
- Script exists at `scripts/build-image.sh`
- Script is executable
- Script successfully builds Docker image
- Image `choo:latest` appears in `docker images`

## NOT In Scope
- Multi-architecture builds
- Container registry push
- Docker Compose integration
- CI/CD pipeline integration
