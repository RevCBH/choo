---
unit: container-image
depends_on: []
---

# CONTAINER-IMAGE Implementation Plan

## Overview

The CONTAINER-IMAGE component provides the Dockerfile and build infrastructure for creating the `choo` container image. This image enables isolated workflow execution by packaging the choo binary alongside essential development tools (git, Claude CLI, GitHub CLI) in a minimal Alpine-based container.

This is infrastructure-only work with no Go code. Verification is via Docker build commands rather than Go tests.

## Task Sequence

| # | Task Spec | Description | Dependencies | Backpressure |
|---|-----------|-------------|--------------|--------------|
| 1 | 01-dockerfile.md | Multi-stage Dockerfile with builder and runtime stages | None | docker build --target builder -t test-builder . |
| 2 | 02-build-script.md | Cross-compilation and image build script | #1 | ./scripts/build-image.sh && docker images choo:latest |

## CI Compatibility

Container isolation is a local-only feature per the PRD. Tests are not run in CI because:
- CI environments typically don't have Docker-in-Docker capabilities
- Container images are built and used locally only (no registry)
- Verification requires manual testing on developer machines

## Baseline Checks

```bash
# No Go code in this unit - verification is via Docker
docker --version
```

## Completion Criteria

- [ ] Dockerfile builds successfully with `docker build .`
- [ ] Build script produces `choo:latest` image
- [ ] Image contains all required tools (git, gh, claude, choo)
- [ ] Image size is under 200 MB target
- [ ] PR created and merged

## Reference

- Design spec: `/specs/CONTAINER-IMAGE.md`
- Related: `/specs/CONTAINER-ISOLATION.md`
