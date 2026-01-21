---
task: 2
status: pending
backpressure: "go build ./internal/worker/..."
depends_on: [1]
---

# Rename invokeClaudeForTask to invokeProvider

**Parent spec**: `/specs/PROVIDER-INTEGRATION.md`
**Task**: #2 of 4 in implementation plan

## Objective

Rename `invokeClaudeForTask` to `invokeProvider` and change the implementation to call `w.provider.Invoke()` instead of hardcoded Claude CLI subprocess execution. Event names remain unchanged for backward compatibility.

## Dependencies

### External Specs (must be implemented)
- PROVIDER - provides `Provider.Invoke()` method

### Task Dependencies (within this unit)
- Task #1 (01-worker-types.md) - Worker has provider field

### Package Dependencies
- `github.com/RevCBH/choo/internal/provider`

## Deliverables

### Files to Modify

```
internal/worker/
└── loop.go    # MODIFY: Rename and update invokeClaudeForTask
```

### Functions to Update

```go
// internal/worker/loop.go

package worker

import (
	"context"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"time"

	"github.com/RevCBH/choo/internal/discovery"
	"github.com/RevCBH/choo/internal/events"
)

// invokeProvider runs the configured provider with the task prompt
// CRITICAL: Uses subprocess via Provider.Invoke(), NEVER direct API calls
// NOTE: We do NOT pass --max-turns to providers. Each invocation is ONE Ralph turn.
// The provider should have unlimited turns to complete the task within a single invocation.
// MaxClaudeRetries controls how many times Ralph will invoke the provider, not internal turns.
func (w *Worker) invokeProvider(ctx context.Context, prompt TaskPrompt) error {
	// Create log file for provider output
	logDir := filepath.Join(w.config.WorktreeBase, "logs")
	if err := os.MkdirAll(logDir, 0755); err != nil {
		fmt.Fprintf(os.Stderr, "Warning: failed to create log directory: %v\n", err)
	}

	// Get provider name for logging
	providerName := "unknown"
	if w.provider != nil {
		providerName = string(w.provider.Name())
	}

	logFile, err := os.Create(filepath.Join(logDir,
		fmt.Sprintf("%s-%s-%d.log", providerName, w.unit.ID, time.Now().Unix())))
	if err != nil {
		fmt.Fprintf(os.Stderr, "Warning: failed to create log file: %v\n", err)
		// Fall back to stdout/stderr (unless suppressed)
		if w.provider == nil {
			return fmt.Errorf("no provider configured for worker")
		}
		if !w.config.SuppressOutput {
			return w.provider.Invoke(ctx, prompt.Content, w.worktreePath, os.Stdout, os.Stderr)
		}
		return w.provider.Invoke(ctx, prompt.Content, w.worktreePath, io.Discard, io.Discard)
	}
	defer logFile.Close()

	// Write prompt to log file
	fmt.Fprintf(logFile, "=== PROMPT ===\n%s\n=== END PROMPT ===\n\n", prompt.Content)
	fmt.Fprintf(logFile, "=== PROVIDER: %s ===\n", providerName)

	// Configure output writers
	var stdout, stderr io.Writer
	if w.config.SuppressOutput {
		stdout = logFile
		stderr = logFile
	} else {
		stdout = io.MultiWriter(os.Stdout, logFile)
		stderr = io.MultiWriter(os.Stderr, logFile)
		fmt.Fprintf(os.Stderr, "Provider output logging to: %s\n", logFile.Name())
	}

	// Emit TaskClaudeInvoke event (name unchanged for backward compatibility)
	if w.events != nil {
		evt := events.NewEvent(events.TaskClaudeInvoke, w.unit.ID)
		if w.currentTask != nil {
			evt = evt.WithTask(w.currentTask.Number).WithPayload(map[string]any{
				"title":    w.currentTask.Title,
				"provider": providerName,
			})
		}
		w.events.Emit(evt)
	}

	// Check if provider is configured
	if w.provider == nil {
		return fmt.Errorf("no provider configured for worker")
	}

	// Invoke provider
	runErr := w.provider.Invoke(ctx, prompt.Content, w.worktreePath, stdout, stderr)

	// Write completion status to log
	fmt.Fprintf(logFile, "\n=== END PROVIDER OUTPUT ===\n")
	if runErr != nil {
		fmt.Fprintf(logFile, "Exit error: %v\n", runErr)
	} else {
		fmt.Fprintf(logFile, "Exit: success\n")
	}

	// Emit TaskClaudeDone event (name unchanged for backward compatibility)
	if w.events != nil {
		evt := events.NewEvent(events.TaskClaudeDone, w.unit.ID)
		if w.currentTask != nil {
			evt = evt.WithTask(w.currentTask.Number)
		}
		if runErr != nil {
			evt = evt.WithError(runErr)
		}
		w.events.Emit(evt)
	}

	return runErr
}
```

### Update Call Sites

Update all references to `invokeClaudeForTask` to use `invokeProvider`:

```go
// internal/worker/loop.go - in executeTaskWithRetry function
// Change:
//   claudeErr := w.invokeClaudeForTask(ctx, prompt)
// To:
//   claudeErr := w.invokeProvider(ctx, prompt)

// internal/worker/worker.go - in runBaselinePhase function
// Change:
//   if err := w.invokeClaudeForTask(ctx, prompt); err != nil {
// To:
//   if err := w.invokeProvider(ctx, prompt); err != nil {
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
| No undefined references | invokeClaudeForTask fully replaced |
| Provider invoked | w.provider.Invoke() called in invokeProvider |
| Events emitted | TaskClaudeInvoke and TaskClaudeDone still emitted |

### CI Compatibility

- [x] No external API keys required
- [x] No network access required
- [x] Runs in <60 seconds

## Implementation Notes

- Remove the old `invokeClaudeForTask` function entirely
- Create new `invokeProvider` function that delegates to `w.provider.Invoke()`
- Keep event names `TaskClaudeInvoke` and `TaskClaudeDone` unchanged for backward compatibility
- Add `provider` field to event payload for consumers that need to distinguish
- Handle nil provider gracefully with clear error message
- Log file naming changes from `claude-<unit>-<timestamp>.log` to `<provider>-<unit>-<timestamp>.log`
- Provider.Invoke() handles the subprocess execution internally

## NOT In Scope

- Creating new event types (keep existing for compatibility)
- Pool modifications (task #3)
- Orchestrator changes (task #4)
- Provider resolution logic (task #4)
