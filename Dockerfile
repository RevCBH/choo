# Build stage
FROM golang:1.23-alpine AS builder

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

# Add ca-certificates for HTTPS, git for runtime operations, and dependencies for CLI tools
RUN apk add --no-cache ca-certificates git curl bash nodejs npm

# Install GitHub CLI from Alpine community repository
RUN apk add --no-cache --repository=https://dl-cdn.alpinelinux.org/alpine/edge/community github-cli

# Install Claude CLI via npm (official distribution method)
RUN npm install -g @anthropic-ai/claude-code

# Create non-root user and set up home directory
RUN adduser -D -u 1000 choo

# Use home directory as workdir (user has write permissions here)
WORKDIR /home/choo

# Copy the binary from builder
COPY --from=builder /choo /usr/local/bin/choo

# Switch to non-root user
USER choo

ENTRYPOINT ["choo"]
