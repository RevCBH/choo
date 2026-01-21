---
task: 1
status: complete
backpressure: "go build ./internal/worker/..."
depends_on: []
---

# Worker Types Update for Provider

**Parent spec**: `/specs/PROVIDER-INTEGRATION.md`
**Task**: #1 of 4 in implementation plan

## Objective

Update the Worker struct and WorkerDeps to include a Provider field, replacing the existing ClaudeClient dependency. This enables workers to use any provider implementation rather than being hardcoded to Claude.

## Dependencies

### External Specs (must be implemented)
- PROVIDER - provides `Provider` interface, `ProviderType`

### Task Dependencies (within this unit)
- None (first task)

### Package Dependencies
- `github.com/RevCBH/choo/internal/provider`

## Deliverables

### Files to Modify

```
internal/worker/
└── worker.go    # MODIFY: Add provider field to Worker and WorkerDeps
```

### Types to Update

```go
// internal/worker/worker.go

package worker

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"time"

	"github.com/RevCBH/choo/internal/discovery"
	"github.com/RevCBH/choo/internal/escalate"
	"github.com/RevCBH/choo/internal/events"
	"github.com/RevCBH/choo/internal/git"
	"github.com/RevCBH/choo/internal/github"
	"github.com/RevCBH/choo/internal/provider"
	"gopkg.in/yaml.v3"
)

// Worker executes a single unit in an isolated worktree
type Worker struct {
	unit         *discovery.Unit
	config       WorkerConfig
	events       *events.Bus
	git          *git.WorktreeManager
	gitRunner    git.Runner
	github       *github.PRClient
	provider     provider.Provider  // NEW: replaces claude field
	escalator    escalate.Escalator
	worktreePath string
	branch       string
	currentTask  *discovery.Task

	// prNumber is the PR number after creation
	//nolint:unused // WIP: used by forcePushAndMerge when conflict resolution is fully integrated
	prNumber int

	// invokeClaudeWithOutput is the function that invokes Claude and captures output
	// Can be overridden for testing
	//nolint:unused // WIP: used in integration tests for PR creation
	invokeClaudeWithOutput func(ctx context.Context, prompt string) (string, error)
}

// WorkerConfig holds worker configuration
type WorkerConfig struct {
	RepoRoot            string
	TargetBranch        string
	WorktreeBase        string
	BaselineChecks      []BaselineCheck
	MaxClaudeRetries    int
	MaxBaselineRetries  int
	BackpressureTimeout time.Duration
	BaselineTimeout     time.Duration
	NoPR                bool
	SuppressOutput      bool // When true, don't tee Claude output to stdout (TUI mode)
}

// BaselineCheck represents a single baseline validation command
type BaselineCheck struct {
	Name    string
	Command string
	Pattern string
}

// WorkerDeps bundles worker dependencies for injection
type WorkerDeps struct {
	Events    *events.Bus
	Git       *git.WorktreeManager
	GitRunner git.Runner
	GitHub    *github.PRClient
	Provider  provider.Provider  // NEW: replaces Claude field
	Escalator escalate.Escalator
}

// ClaudeClient is deprecated - use Provider instead
// Kept for backward compatibility during migration
// Deprecated: Use Provider field in WorkerDeps instead
type ClaudeClient any
```

### Functions to Update

```go
// internal/worker/worker.go

// NewWorker creates a worker for executing a unit
func NewWorker(unit *discovery.Unit, cfg WorkerConfig, deps WorkerDeps) *Worker {
	gitRunner := deps.GitRunner
	if gitRunner == nil {
		gitRunner = git.DefaultRunner()
	}
	return &Worker{
		unit:      unit,
		config:    cfg,
		events:    deps.Events,
		git:       deps.Git,
		gitRunner: gitRunner,
		github:    deps.GitHub,
		provider:  deps.Provider,  // NEW: store provider instead of claude
		escalator: deps.Escalator,
	}
}
```

## Backpressure

### Validation Command

```bash
go build ./internal/worker/...
```

### Must Pass

| Test | Assertion |
|------|-----------|
| Build succeeds | No compilation errors |
| Provider field exists | Worker.provider field accessible |
| WorkerDeps updated | WorkerDeps.Provider field accessible |
| NewWorker compiles | Function accepts WorkerDeps with Provider |

### CI Compatibility

- [x] No external API keys required
- [x] No network access required
- [x] Runs in <60 seconds

## Implementation Notes

- Import the `github.com/RevCBH/choo/internal/provider` package
- Remove the `claude *ClaudeClient` field from Worker struct
- Remove the `Claude *ClaudeClient` field from WorkerDeps struct
- Add `provider provider.Provider` field to Worker struct
- Add `Provider provider.Provider` field to WorkerDeps struct
- Update NewWorker to store `deps.Provider` in the worker
- Keep ClaudeClient type definition for backward compatibility but mark as deprecated
- The provider field may be nil initially (when called from existing code) - handle gracefully in task #2

## NOT In Scope

- Changing invokeClaudeForTask implementation (task #2)
- Pool modifications (task #3)
- Orchestrator changes (task #4)
- Removing ClaudeClient references from pool.go (handled in task #3)
