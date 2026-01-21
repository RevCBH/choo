# Known Issues

This document tracks known issues, workarounds, and planned fixes for Choo.

## PATH Resolution When Forking Multiple Units

**Issue:** When forking multiple units in parallel, the PATH environment variable may not be correctly set, causing commands like `claude` to fail with "command not found" errors.

### Symptoms
- Units fail to start with errors like: `exec: "claude": executable file not found in $PATH`
- Issue occurs specifically when running multiple units in parallel (`choo run` with multiple units)
- Works fine when running a single unit (`choo run --unit <unit-id>`)
- More common when using version managers like `nvm` that modify PATH in shell initialization scripts

### Root Cause
When units are executed in parallel goroutines, they inherit the environment from the parent process. If the PATH was set by shell initialization scripts (e.g., `~/.zshrc`, `~/.bashrc`) that aren't executed in the Go process context, commands that rely on PATH resolution (like `claude`) won't be found.

### Workaround
Fully qualify the path to the `claude` binary in `.choo.yaml`:

```yaml
claude:
  command: /Users/bennett/.nvm/versions/node/v22.16.0/bin/claude
```

Or use the `RALPH_CLAUDE_CMD` environment variable:

```bash
export RALPH_CLAUDE_CMD=/Users/bennett/.nvm/versions/node/v22.16.0/bin/claude
choo run specs/tasks/
```

### Planned Fix
- Resolve the full path to the binary at startup (using `exec.LookPath` or similar)
- Cache the resolved path and use it for all unit executions
- Ensure PATH is properly inherited or reconstructed for forked processes
- Consider validating that the command exists and is executable during config loading

### Related Code
- `internal/provider/claude.go` - ClaudeProvider command resolution
- `internal/config/env.go` - Environment variable handling
- `internal/worker/pool.go` - Worker pool that forks units in parallel
