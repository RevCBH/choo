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
