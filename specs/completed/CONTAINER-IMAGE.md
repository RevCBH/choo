# CONTAINER-IMAGE — Dockerfile and Build Script for Choo Container Image

## Overview

The Container Image spec defines the Dockerfile and build infrastructure for creating the `choo` container image. This image enables isolated workflow execution by packaging the choo binary alongside essential development tools (git, Claude CLI, GitHub CLI) in a minimal Alpine-based container.

The build process uses multi-stage Docker builds with cross-compilation to produce a `linux/amd64` binary from any host platform (including macOS). Images are built locally only—there is no container registry involved. This keeps the setup simple and avoids the complexity of registry authentication and image distribution.

```
┌─────────────────────────────────────────────────────────────┐
│                     Build Process                           │
│                                                             │
│  ┌──────────────┐    ┌──────────────┐    ┌──────────────┐  │
│  │  Go Source   │───▶│ Cross-compile│───▶│ Docker Build │  │
│  │  (any host)  │    │ linux/amd64  │    │  Alpine image│  │
│  └──────────────┘    └──────────────┘    └──────────────┘  │
│                                                             │
│  Result: choo:latest image with:                           │
│    - /usr/local/bin/choo (linux/amd64)                     │
│    - git, ssh-client, bash, curl                           │
│    - GitHub CLI (gh)                                       │
│    - Claude CLI (baked in)                                 │
└─────────────────────────────────────────────────────────────┘
```

## Requirements

### Functional Requirements

1. Multi-stage Dockerfile that builds the `choo` binary for `linux/amd64` target
2. Include git and SSH client for repository operations
3. Include GitHub CLI (`gh`) for PR and issue management
4. Include Claude CLI baked into the image during build
5. Use minimal Alpine-based runtime image to reduce attack surface and size
6. Provide build script that handles cross-compilation and image creation
7. Support version tagging via build script argument

### Performance Requirements

| Metric | Target |
|--------|--------|
| Runtime image size | < 200 MB |
| Build time (cached) | < 30 seconds |
| Build time (cold) | < 5 minutes |

### Constraints

1. No container registry—images are built and used locally only
2. No CI testing for container isolation (this is a local development feature)
3. Claude CLI must be baked into the container image, not mounted at runtime
4. Cross-compilation required since developers may build on macOS for linux/amd64 target

## Design

### Module Structure

```
/
├── Dockerfile              # Multi-stage container image definition
└── scripts/
    └── build-image.sh      # Cross-compile and build image script
```

### Core Types

#### Dockerfile

```dockerfile
# Build stage
FROM golang:1.22-alpine AS builder
WORKDIR /build
COPY go.mod go.sum ./
RUN go mod download
COPY . .
RUN CGO_ENABLED=0 GOOS=linux GOARCH=amd64 go build -o /choo ./cmd/choo

# Runtime stage
FROM alpine:3.19
RUN apk add --no-cache git openssh-client ca-certificates bash curl github-cli
COPY --from=builder /choo /usr/local/bin/
WORKDIR /repo
ENTRYPOINT ["choo"]
```

**Stage breakdown:**

| Stage | Base Image | Purpose |
|-------|------------|---------|
| builder | golang:1.22-alpine | Compile Go binary with all dependencies |
| runtime | alpine:3.19 | Minimal production image with tools |

**Installed packages:**

| Package | Purpose |
|---------|---------|
| git | Clone repositories, commit changes |
| openssh-client | SSH authentication for git operations |
| ca-certificates | HTTPS connections to APIs |
| bash | Shell scripting support |
| curl | HTTP requests, downloading Claude CLI |
| github-cli | PR creation, issue management |

#### Build Script

```bash
#!/bin/bash
# scripts/build-image.sh
set -e

VERSION=${1:-latest}

# Cross-compile for linux/amd64
CGO_ENABLED=0 GOOS=linux GOARCH=amd64 go build -o bin/choo-linux-amd64 ./cmd/choo

# Build Docker image
docker build -t "choo:${VERSION}" .

echo "Built choo:${VERSION}"
```

### CLI Interface

```bash
# Build with default 'latest' tag
./scripts/build-image.sh

# Build with specific version tag
./scripts/build-image.sh v0.4.0

# Verify image was created
docker images choo
```

## Implementation Notes

### Cross-Compilation

The build script explicitly sets `CGO_ENABLED=0` to ensure pure Go compilation without C dependencies. This is critical for cross-compilation from macOS to Linux. The `GOOS=linux GOARCH=amd64` flags target the container's runtime environment.

### Claude CLI Installation

The Claude CLI must be baked into the image during the Docker build process. This requires adding an installation step to the runtime stage:

```dockerfile
# Add to runtime stage after apk install
RUN curl -fsSL https://claude.ai/install.sh | sh
```

The exact installation method depends on Claude CLI's distribution mechanism. If Claude CLI requires authentication during install, consider:
- Pre-downloading the binary and COPYing it into the image
- Using a build argument for any required tokens

### Security Considerations

1. **No secrets in image**: Never bake API keys or credentials into the image. These should be passed as environment variables at runtime.
2. **Minimal packages**: Only install packages required for operation to reduce attack surface.
3. **Non-root user**: Consider adding a non-root user for production use (not in initial implementation).

### Platform Compatibility

The Dockerfile uses `GOARCH=amd64` which works on:
- Intel/AMD x86_64 hosts (native)
- Apple Silicon hosts (via Rosetta/QEMU emulation in Docker)

For Apple Silicon developers, Docker Desktop handles the emulation transparently.

## Testing Strategy

### Unit Tests

Not applicable. Dockerfiles and shell scripts don't have unit tests in the traditional sense. Correctness is verified through integration and manual testing.

### Integration Tests

1. **Image builds successfully**: `./scripts/build-image.sh` exits with code 0
2. **Binary executes in container**: `docker run choo:latest --version` returns version string
3. **Git is available**: `docker run choo:latest git --version` succeeds
4. **GitHub CLI is available**: `docker run choo:latest gh --version` succeeds
5. **Claude CLI is available**: `docker run choo:latest claude --version` succeeds

### Manual Testing

- [ ] Build image on macOS (Apple Silicon)
- [ ] Build image on macOS (Intel)
- [ ] Build image on Linux
- [ ] Run container and verify choo binary works
- [ ] Verify git clone works inside container
- [ ] Verify gh auth works with mounted credentials
- [ ] Verify Claude CLI can authenticate and respond
- [ ] Check image size is under 200 MB target

## Design Decisions

### Why Multi-Stage Build?

Multi-stage builds separate the build environment (with Go toolchain, ~800MB) from the runtime environment (~50MB base). This reduces the final image size by 10x or more and eliminates build tools that could be security risks in production.

**Trade-offs considered:**
- Single stage with Go: Simpler but 800MB+ image size
- External binary build: Requires separate build step, more complex CI
- Multi-stage: Slightly longer initial build, but self-contained and reproducible

### Why Alpine?

Alpine Linux provides a minimal base image (~5MB) with a package manager for installing required tools. The musl libc is compatible with statically-compiled Go binaries.

**Trade-offs considered:**
- Debian/Ubuntu: Larger image, more familiar tools, better glibc compatibility
- Distroless: Smaller, but no shell for debugging or running git/gh
- Alpine: Good balance of size and functionality

### Why Local-Only Images?

Container registries add complexity (authentication, versioning, distribution) that isn't needed for local development workflows. Developers build the image on their machine and use it locally.

**Trade-offs considered:**
- Docker Hub: Public, easy, but requires account and versioning strategy
- Private registry: Full control, but operational overhead
- Local-only: Simple, no infrastructure, but each developer rebuilds

### Why Bake Claude CLI Into Image?

Baking Claude CLI into the image ensures consistent versions across all container runs and simplifies the runtime environment. Mounting the CLI from the host would require path mapping and version synchronization.

## Future Enhancements

1. **Multi-architecture support**: Build for both `linux/amd64` and `linux/arm64` using `docker buildx`
2. **Image signing**: Sign images with cosign for supply chain security
3. **Health check**: Add `HEALTHCHECK` instruction for orchestration systems
4. **Non-root user**: Run as non-root user for improved security posture
5. **Build cache optimization**: Use `--mount=type=cache` for Go module cache
6. **Version pinning**: Pin all package versions for reproducible builds

## References

- [CONTAINER-ISOLATION](./CONTAINER-ISOLATION.md) — Container isolation architecture
- [PRD](./PRD.md) — Product requirements document
- [Docker multi-stage builds](https://docs.docker.com/build/building/multi-stage/)
- [Alpine Linux packages](https://pkgs.alpinelinux.org/packages)
