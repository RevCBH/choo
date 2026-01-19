---
task: 2
status: complete
backpressure: "go test ./internal/worker/... -run Prompt"
depends_on: []
---

# Task Prompt Construction

**Parent spec**: `/specs/WORKER.md`
**Task**: #2 of 8 in implementation plan

## Objective

Implement functions to construct Claude prompts for task execution and baseline fix scenarios.

## Dependencies

### External Specs (must be implemented)
- DISCOVERY - provides `Task` type

### Task Dependencies (within this unit)
- None (can be implemented in parallel with task #1)

### Package Dependencies
- `strings` - for string building
- `fmt` - for formatting

## Deliverables

### Files to Create

```
internal/worker/
└── prompt.go       # Prompt construction functions
```

### Types to Implement

```go
// internal/worker/prompt.go

// TaskPrompt contains the constructed prompt for Claude
type TaskPrompt struct {
    Content    string
    ReadyTasks []*discovery.Task
}
```

### Functions to Implement

```go
// BuildTaskPrompt constructs the Claude prompt for ready tasks
// The prompt presents all ready tasks and instructs Claude to choose one
func BuildTaskPrompt(readyTasks []*discovery.Task) TaskPrompt {
    // Build task list with each task's number, title, file path, and backpressure
    // Include instructions for:
    // 1. Choose ONE task from the ready list
    // 2. Read the task spec file completely
    // 3. Implement ONLY what is specified
    // 4. Run backpressure validation
    // 5. Update frontmatter status to complete when done
    // 6. Do NOT move on to other tasks
}

// BuildBaselineFixPrompt constructs the prompt for fixing baseline failures
func BuildBaselineFixPrompt(checkOutput string, baselineCommands string) string {
    // Include:
    // 1. The baseline check failure output
    // 2. The commands that need to pass
    // 3. Instructions to fix issues only (no refactoring)
    // 4. Note that orchestrator will commit
}
```

## Backpressure

### Validation Command

```bash
go test ./internal/worker/... -run Prompt
```

### Must Pass

| Test | Assertion |
|------|-----------|
| `TestBuildTaskPrompt_SingleTask` | Prompt contains task title, file path, backpressure command |
| `TestBuildTaskPrompt_MultipleTasks` | Prompt contains all tasks, instructs to "Choose ONE" |
| `TestBuildTaskPrompt_EmptyTasks` | Returns empty TaskPrompt with no panic |
| `TestBuildBaselineFixPrompt` | Output contains check output and baseline commands |

### Test Implementation

```go
func TestBuildTaskPrompt_SingleTask(t *testing.T) {
    tasks := []*discovery.Task{
        {Number: 1, Title: "Nav Types", FilePath: "01-nav-types.md", Backpressure: "pnpm typecheck"},
    }

    prompt := BuildTaskPrompt(tasks)

    if !strings.Contains(prompt.Content, "Task #1: Nav Types") {
        t.Error("prompt should contain task title")
    }
    if !strings.Contains(prompt.Content, "pnpm typecheck") {
        t.Error("prompt should contain backpressure command")
    }
    if len(prompt.ReadyTasks) != 1 {
        t.Errorf("expected 1 ready task, got %d", len(prompt.ReadyTasks))
    }
}

func TestBuildTaskPrompt_MultipleTasks(t *testing.T) {
    tasks := []*discovery.Task{
        {Number: 1, Title: "Task A", FilePath: "01-a.md", Backpressure: "cmd-a"},
        {Number: 2, Title: "Task B", FilePath: "02-b.md", Backpressure: "cmd-b"},
        {Number: 3, Title: "Task C", FilePath: "03-c.md", Backpressure: "cmd-c"},
    }

    prompt := BuildTaskPrompt(tasks)

    for _, task := range tasks {
        if !strings.Contains(prompt.Content, task.Title) {
            t.Errorf("prompt should contain task %q", task.Title)
        }
    }
    if !strings.Contains(prompt.Content, "Choose ONE") {
        t.Error("prompt should instruct to choose one task")
    }
}

func TestBuildTaskPrompt_EmptyTasks(t *testing.T) {
    prompt := BuildTaskPrompt([]*discovery.Task{})

    if prompt.Content == "" {
        t.Error("should still have instruction content")
    }
    if len(prompt.ReadyTasks) != 0 {
        t.Error("should have empty ready tasks")
    }
}

func TestBuildBaselineFixPrompt(t *testing.T) {
    output := "fmt: main.go has incorrect formatting"
    commands := "go fmt ./..."

    prompt := BuildBaselineFixPrompt(output, commands)

    if !strings.Contains(prompt, output) {
        t.Error("prompt should contain check output")
    }
    if !strings.Contains(prompt, commands) {
        t.Error("prompt should contain baseline commands")
    }
    if !strings.Contains(prompt, "Do NOT commit") {
        t.Error("prompt should instruct not to commit")
    }
}
```

### CI Compatibility

- [x] No external API keys required
- [x] No network access required
- [x] Runs in <60 seconds

## Implementation Notes

- The prompt format must match what's specified in WORKER.md exactly
- Use backticks for code in the prompt (status values, commands)
- Include critical instruction about NEVER running tests in watch mode
- ReadyTasks slice in TaskPrompt should be the same reference as input

## NOT In Scope

- Claude CLI invocation (task #5)
- Parsing Claude responses
- Streaming output handling
