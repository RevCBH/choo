# Multi-Provider Support - Product Requirements Document

## Document Info

| Field   | Value      |
| ------- | ---------- |
| Status  | Draft      |
| Author  | Claude     |
| Created | 2026-01-20 |
| Target  | v0.4       |

---

## 1. Overview

### 1.1 Goal

Add OpenAI Codex CLI as an alternative provider to Claude CLI for **task execution in the ralph inner loops only**, with multi-level configuration (global, per-run, per-unit).

### 1.2 Scope

**In Scope:**
- Provider selection for task execution within `invokeClaudeForTask()` inner loops
- CLI flags, environment variables, and per-unit frontmatter for controlling provider selection

**Out of Scope (Future Work):**
- Provider selection for merge conflict resolution (`git_delegate.go`)
- Provider selection for branch naming (`git/branch.go`)
- Provider selection for PR creation and other orchestration-level LLM calls
- Custom invocation configuration per provider (model parameters, MaxTurns, etc.)
- UX/event naming updates (TUI and events currently reference "Claude" specifically)

These items remain explicitly tied to Claude for now and will be addressed in future PRDs.

### 1.3 Architecture

Currently, Claude is invoked in `internal/worker/loop.go:invokeClaudeForTask()` via subprocess:
```go
cmd := exec.CommandContext(ctx, "claude", args...)  // args: --dangerously-skip-permissions -p <prompt>
```

We'll create a **Provider interface** following the existing factory pattern in `internal/escalate/factory.go`.

```
┌─────────────────────────────────────────────────────────────────────────────┐
│  Configuration Resolution (for task provider selection)                      │
│                                                                              │
│  Precedence (highest to lowest):                                            │
│  1. --force-task-provider    ──► Overrides everything (for inner loops)     │
│  2. Per-unit frontmatter     ──► Unit author specifies provider             │
│  3. --provider CLI arg       ──► Default for units without override         │
│  4. RALPH_PROVIDER env var   ──► Environment-level default                  │
│  5. .choo.yaml global        ──► Fallback default                           │
└─────────────────────────────────────────────────────────────────────────────┘

┌─────────────────────────────────────────────────────────────────────────────┐
│  Provider Interface                                                          │
│                                                                              │
│  internal/provider/                                                          │
│  ├── provider.go     ──► Interface definition                               │
│  ├── claude.go       ──► claude --dangerously-skip-permissions -p <prompt>  │
│  ├── codex.go        ──► codex exec --yolo <prompt>                         │
│  └── factory.go      ──► FromConfig(cfg) (Provider, error)                  │
└─────────────────────────────────────────────────────────────────────────────┘
```

**Note:** Provider invocation configuration (model parameters, MaxTurns, etc.) could be customized per-provider in the context of task selection, but this PRD assumes sensible defaults for each provider to keep the initial implementation simple. Exposing these configuration knobs is deferred to future work.

### 1.4 Benefits

- **Cost optimization** - Use cheaper providers for simpler tasks
- **Speed** - Some providers may have lower latency
- **Flexibility** - Swap providers easily for testing/comparison
- **Per-unit control** - Unit authors can specify their preferred provider

### 1.5 Key Decisions

- **Interface-based with factory pattern** - Clean abstraction for different CLI invocation patterns
- **Subprocess execution** - Both Claude and Codex are invoked as CLI subprocesses (no direct API)
- **Five-level precedence** - `--force-task-provider` > per-unit frontmatter > `--provider` CLI arg > env var > `.choo.yaml`
- **Task-scoped only** - This PRD only covers task execution in inner loops; other LLM uses remain Claude-only
- **Default invocation settings** - Provider invocation configuration uses sensible defaults; customization is future work
- **Backward compatible** - Claude remains default, existing configs continue to work

---

## 2. Provider Interface

### 2.1 Interface Definition

```go
// internal/provider/provider.go
package provider

type ProviderType string

const (
    ProviderClaude ProviderType = "claude"
    ProviderCodex  ProviderType = "codex"
)

type Provider interface {
    Invoke(ctx context.Context, prompt string, workdir string, stdout, stderr io.Writer) error
    Name() ProviderType
}

type Config struct {
    Type    ProviderType
    Command string
}
```

### 2.2 Claude Implementation

```go
// internal/provider/claude.go
func (p *ClaudeProvider) Invoke(ctx context.Context, prompt string, workdir string, stdout, stderr io.Writer) error {
    args := []string{
        "--dangerously-skip-permissions",
        "-p", prompt,
    }
    cmd := exec.CommandContext(ctx, p.command, args...)
    cmd.Dir = workdir
    cmd.Stdout = stdout
    cmd.Stderr = stderr
    return cmd.Run()
}
```

### 2.3 Codex Implementation

```go
// internal/provider/codex.go
func (p *CodexProvider) Invoke(ctx context.Context, prompt string, workdir string, stdout, stderr io.Writer) error {
    args := []string{
        "exec",
        "--yolo",
        prompt,
    }
    cmd := exec.CommandContext(ctx, p.command, args...)
    cmd.Dir = workdir
    cmd.Stdout = stdout
    cmd.Stderr = stderr
    return cmd.Run()
}
```

### 2.4 Factory Function

```go
// internal/provider/factory.go
func FromConfig(cfg Config) (Provider, error) {
    switch cfg.Type {
    case ProviderClaude, "":
        return NewClaude(cfg.Command), nil
    case ProviderCodex:
        return NewCodex(cfg.Command), nil
    default:
        return nil, fmt.Errorf("unknown provider type: %s", cfg.Type)
    }
}
```

---

## 3. Configuration

### 3.1 Config Schema

**Add to `internal/config/config.go`:**

```go
type ProviderConfig struct {
    Type      string                       `yaml:"type"`      // "claude" (default) or "codex"
    Command   string                       `yaml:"command"`   // Override CLI binary path
    Providers map[string]ProviderSettings  `yaml:"providers"` // Per-provider settings
}

type ProviderSettings struct {
    Command string `yaml:"command"`
}
```

### 3.2 Global Config (`.choo.yaml`)

```yaml
provider:
  type: claude
  providers:
    claude:
      command: claude
    codex:
      command: codex
```

### 3.3 CLI Flags

| Flag | Description |
|------|-------------|
| `--provider` | Default provider for task execution (units without frontmatter override) |
| `--force-task-provider` | Force all task inner loops to use this provider (overrides per-unit frontmatter) |

**Note:** `--force-task-provider` only applies to task execution in the inner ralph loops. It does not affect other LLM operations like merge conflict resolution, branch naming, or PR creation (those remain Claude-only for this PRD).

```bash
# Use Codex as default for task execution
choo run --provider=codex

# Force all task inner loops to use Codex (ignores per-unit settings)
choo run --force-task-provider=codex
```

### 3.4 Per-Unit Override (Frontmatter)

```yaml
---
unit: my-feature
provider: codex
depends_on:
  - base-types
---
```

### 3.5 Environment Variables

| Variable | Maps To | Notes |
|----------|---------|-------|
| `RALPH_PROVIDER` | `Provider.Type` | Lower precedence than CLI args, higher than `.choo.yaml` |
| `RALPH_CODEX_CMD` | `Provider.Providers["codex"].Command` | Override codex binary path |

**Precedence for task provider selection:**
1. `--force-task-provider` CLI flag (overrides everything)
2. Per-unit frontmatter (`provider: codex`)
3. `--provider` CLI flag
4. `RALPH_PROVIDER` environment variable
5. `.choo.yaml` global config

---

## 4. Implementation Plan

### 4.1 New Files

| File | Purpose |
|------|---------|
| `internal/provider/provider.go` | Provider interface definition |
| `internal/provider/claude.go` | Claude CLI implementation |
| `internal/provider/codex.go` | Codex CLI implementation |
| `internal/provider/factory.go` | Factory function |

### 4.2 Modified Files

| File | Change |
|------|--------|
| `internal/config/config.go` | Add `ProviderConfig` struct |
| `internal/config/defaults.go` | Add `DefaultProviderType`, `DefaultCodexCommand` |
| `internal/config/env.go` | Add `RALPH_PROVIDER`, `RALPH_CODEX_CMD` |
| `internal/cli/run.go` | Add `--provider` and `--force-task-provider` flags |
| `internal/discovery/frontmatter.go` | Add `Provider` field to `UnitFrontmatter` |
| `internal/worker/worker.go` | Replace `ClaudeClient` with `Provider` |
| `internal/worker/loop.go` | Rename `invokeClaudeForTask` → `invokeProvider` |
| `internal/orchestrator/orchestrator.go` | Add provider config, per-unit resolution |
| `internal/worker/pool.go` | Pass provider configuration to workers |

---

## 5. Backward Compatibility

- Default provider is `claude` - existing behavior unchanged
- Existing `ClaudeConfig` remains for legacy support
- Empty `provider.type` falls back to Claude
- Existing `.choo.yaml` files work without modification

---

## 6. Verification

### 6.1 Unit Tests

- Test provider factory with each provider type
- Test config loading with provider section
- Test precedence chain resolution

### 6.2 Integration Tests

```bash
# Test Claude (existing behavior)
choo run --dry-run

# Test Codex via --provider flag
choo run --provider=codex --dry-run

# Test per-unit override (add provider: codex to a unit's frontmatter)
choo run --dry-run

# Test --force-task-provider overrides per-unit settings
choo run --force-task-provider=claude --dry-run
```

### 6.3 Manual Verification

1. Create a simple task spec
2. Run with `--provider=codex` and verify Codex CLI is invoked for task execution
3. Run with default and verify Claude CLI is invoked
4. Add `provider: codex` to a unit's frontmatter and verify it uses Codex
5. Run with `--force-task-provider=claude` and verify it overrides the per-unit setting
6. Verify that non-task LLM operations (merge conflicts, branch naming) still use Claude regardless of provider settings

---

## 7. Known Limitations

The following limitations are acknowledged and intentionally deferred:

### 7.1 Provider Interface Constraints

The proposed `Provider` interface lacks mechanisms for:
- Capturing LLM output for post-processing (e.g., PR URL extraction)
- Controlling model parameters (MaxTurns, specific models)
- Streaming vs batch output control

These capabilities are currently relied upon by existing functionalities like PR URL extraction and branch naming. For this PRD, those operations remain explicitly tied to Claude and bypass the provider abstraction.

### 7.2 UX/Event Naming

UI elements and event names in the TUI and `choo prompt` command currently reference "Claude" specifically. When using other providers, this will be misleading. Addressing this is out of scope for the initial implementation.

### 7.3 Provider-Specific Features

Different providers may have different capabilities (tool use, output formats, context limits). This PRD assumes providers are interchangeable for basic prompt-in/execution-out tasks. Feature parity checking is not implemented.

---

## 8. Review Feedback Addressed

| Feedback | Resolution |
|----------|------------|
| Provider scope ambiguity | Added explicit "Scope" section (1.2); clarified that only task inner loops are affected |
| `--force-provider` naming | Renamed to `--force-task-provider` to clarify it only applies to task execution |
| Configuration precedence | Documented five-level precedence: task spec > CLI args > env vars > .choo.yaml |
| Provider invocation configuration | Noted as future work; using sensible defaults for initial implementation |
| Provider interface limitations | Acknowledged in "Known Limitations" section; non-task LLM ops remain Claude-only |
| UX/event naming | Acknowledged in "Known Limitations" section; out of scope for this PRD |
