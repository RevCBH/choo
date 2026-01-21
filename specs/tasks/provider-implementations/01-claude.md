---
task: 1
status: complete
backpressure: "go test ./internal/provider/... -run Claude"
depends_on: []
---

# Claude Provider Implementation

**Parent spec**: `/specs/PROVIDER-IMPLEMENTATIONS.md`
**Task**: #1 of 2 in implementation plan

## Objective

Implement ClaudeProvider struct with Invoke and Name methods that invoke the Claude CLI as a subprocess.

## Dependencies

### Task Dependencies (within this unit)
- None

### External Spec Dependencies
- `provider-interface`: Provider interface and ProviderType constants

### Package Dependencies
- `context` (standard library)
- `fmt` (standard library)
- `io` (standard library)
- `os/exec` (standard library)
- `errors` (standard library)

## Deliverables

### Files to Create/Modify
```
internal/provider/
└── claude.go    # CREATE: ClaudeProvider implementation
```

### Types to Implement

```go
// internal/provider/claude.go
package provider

import (
    "context"
    "errors"
    "fmt"
    "io"
    "os/exec"
)

// ClaudeProvider implements Provider using the Claude CLI
type ClaudeProvider struct {
    // command is the path to the claude executable
    command string
}

// NewClaude creates a Claude provider with the specified command path.
// If command is empty, defaults to "claude".
func NewClaude(command string) *ClaudeProvider {
    if command == "" {
        command = "claude"
    }
    return &ClaudeProvider{command: command}
}

// Invoke executes the Claude CLI with the given prompt.
// Uses --dangerously-skip-permissions and --print for automated execution.
func (p *ClaudeProvider) Invoke(ctx context.Context, prompt string, workdir string, stdout, stderr io.Writer) error {
    // Build command with flags for automated execution
    // --dangerously-skip-permissions: Skip permission prompts (required for automation)
    // --print: Output to stdout instead of interactive mode
    // -p: Prompt to execute
    cmd := exec.CommandContext(ctx, p.command,
        "--dangerously-skip-permissions",
        "--print",
        "-p", prompt,
    )
    cmd.Dir = workdir
    cmd.Stdout = stdout
    cmd.Stderr = stderr

    if err := cmd.Run(); err != nil {
        // Check if context was cancelled
        if ctx.Err() != nil {
            return fmt.Errorf("claude invocation cancelled: %w", ctx.Err())
        }
        // Wrap error with provider name and exit code if available
        var exitErr *exec.ExitError
        if errors.As(err, &exitErr) {
            return fmt.Errorf("claude exited with code %d: %w", exitErr.ExitCode(), err)
        }
        return fmt.Errorf("claude invocation failed: %w", err)
    }

    return nil
}

// Name returns the provider type identifier
func (p *ClaudeProvider) Name() ProviderType {
    return ProviderClaude
}
```

## Backpressure

### Validation Command
```bash
go build ./internal/provider/...
```

### Must Pass
| Test | Assertion |
|------|-----------|
| Build succeeds | No compilation errors |
| ClaudeProvider compiles | Struct and methods defined |
| Implements Provider | Interface satisfaction verified by compiler |

### CI Compatibility
- [ ] `go build ./internal/provider/...` exits 0
- [ ] `go vet ./internal/provider/...` reports no issues

## Implementation Notes

### Claude CLI Flags

The Claude CLI requires specific flags for non-interactive operation:

- `--dangerously-skip-permissions`: Required to bypass permission prompts in automated workflows. Without this flag, Claude will pause and ask for permission before file operations.
- `--print`: Outputs to stdout instead of the interactive TUI mode.
- `-p <prompt>`: Passes the prompt as a command-line argument.

### Error Wrapping

All errors must be wrapped with "claude" prefix for identification in logs:
- Cancellation: `"claude invocation cancelled: ..."`
- Exit error: `"claude exited with code N: ..."`
- Other: `"claude invocation failed: ..."`

### Context Cancellation

`exec.CommandContext` automatically sends SIGKILL when context is cancelled. The error check `ctx.Err() != nil` distinguishes user-initiated cancellation from other failures.

## NOT In Scope
- CodexProvider implementation (Task #2)
- Integration tests requiring Claude CLI installed
- Custom flag configuration per invocation
- Retry logic on transient failures
