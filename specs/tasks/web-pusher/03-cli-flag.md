---
task: 3
status: complete
backpressure: "go build ./cmd/choo/... && go test ./internal/cli/... -run TestRunOptions"
depends_on: [2]
---

# CLI --web Flag

**Parent spec**: `/specs/WEB-PUSHER.md`
**Task**: #3 of 3

## Objective

Add `--web` flag to the `choo run` command that enables event forwarding to the web UI via SocketPusher.

## Dependencies

### Task Dependencies
- Task #2: SocketPusher implementation

### Package Dependencies
- `choo/internal/web`
- `choo/internal/cli`
- `github.com/spf13/cobra`

## Deliverables

### Files to Modify

```
internal/cli/
└── run.go    # MODIFY: Add --web flag and SocketPusher integration
```

### Changes to Implement

#### 1. Update RunOptions struct

```go
// RunOptions holds flags for the run command
type RunOptions struct {
	Parallelism  int
	TargetBranch string
	DryRun       bool
	NoPR         bool
	Unit         string
	SkipReview   bool
	TasksDir     string
	Web          bool   // NEW: Enable web UI event forwarding
	WebSocket    string // NEW: Custom socket path (optional)
}
```

#### 2. Add flag to NewRunCmd

```go
cmd.Flags().BoolVar(&opts.Web, "web", false, "Enable web UI event forwarding")
cmd.Flags().StringVar(&opts.WebSocket, "web-socket", "", "Custom Unix socket path (default: /tmp/choo.sock)")
```

#### 3. Update RunOrchestrator to create SocketPusher

```go
func (a *App) RunOrchestrator(ctx context.Context, opts RunOptions) error {
	// ... existing setup ...

	// Create event bus
	eventBus := events.NewBus(1000)
	defer eventBus.Close()

	// NEW: Create SocketPusher if --web flag is set
	var pusher *web.SocketPusher
	if opts.Web {
		cfg := web.DefaultPusherConfig()
		if opts.WebSocket != "" {
			cfg.SocketPath = opts.WebSocket
		}
		pusher = web.NewSocketPusher(eventBus, cfg)
		defer pusher.Close()
	}

	// ... create orchestrator ...

	// NEW: Start pusher after graph is built (post-discovery)
	// SetGraph is called with the scheduler's graph
	// pusher.Start(ctx) is called before orch.Run()

	// ... rest of implementation ...
}
```

#### 4. Wire SocketPusher into orchestration flow

The pusher needs the dependency graph after discovery but before execution:

```go
// After discovery and graph construction
if pusher != nil {
	pusher.SetGraph(graph, opts.Parallelism)
	if err := pusher.Start(ctx); err != nil {
		// Log warning but don't fail - web UI is optional
		log.Printf("WARN: failed to start web pusher: %v", err)
	}
}
```

### Test Cases

Add to existing CLI tests:

```go
func TestRunOptions_WebFlag(t *testing.T) {
	// Test --web flag is parsed correctly
}

func TestRunOptions_WebSocketFlag(t *testing.T) {
	// Test --web-socket flag is parsed correctly
}

func TestRunOptions_WebValidation(t *testing.T) {
	// Test that --web-socket without --web is ignored
}
```

### Implementation Notes

1. `--web` is opt-in; orchestration works without it
2. SocketPusher failure should warn but not fail the run
3. `--web-socket` allows custom socket path for development
4. Pusher is created early but started after graph is built
5. Pusher is closed in defer to ensure cleanup on any exit path

## Backpressure

### Validation Command

```bash
go build ./cmd/choo/... && go test ./internal/cli/... -run TestRunOptions
```

### Must Pass
| Check | Assertion |
|-------|-----------|
| Build succeeds | `go build ./cmd/choo/...` completes |
| Flag parsed | `--web` sets opts.Web = true |
| Socket flag parsed | `--web-socket` sets custom path |
| Help shows flags | `choo run --help` lists --web and --web-socket |

### CI Compatibility
- [x] No external API keys
- [x] No network access
- [x] Runs in <60 seconds

## NOT In Scope
- SocketPusher implementation (task #2)
- Web UI implementation
- WebSocket or HTTP transport
