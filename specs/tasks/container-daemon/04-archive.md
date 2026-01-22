---
task: 4
status: pending
backpressure: "go test ./internal/cli/... -run TestArchive"
depends_on: []
---

# Archive Command

**Parent spec**: `/specs/CONTAINER-DAEMON.md`
**Task**: #4 of 6 in implementation plan

## Objective

Implement the `choo archive` command to move completed specs from `specs/` to `specs/completed/`.

## Dependencies

### External Specs (must be implemented)
- None

### Task Dependencies (within this unit)
- None

### Package Dependencies
- `os` (standard library)
- `path/filepath` (standard library)
- `strings` (standard library)

## Deliverables

### Files to Create/Modify

```
internal/cli/
├── archive.go      # CREATE: Archive command implementation
└── archive_test.go # CREATE: Archive tests
```

### Functions to Implement

```go
// internal/cli/archive.go

package cli

import (
    "fmt"
    "log"
    "os"
    "path/filepath"
    "strings"
)

// ArchiveOptions holds configuration for the archive command.
type ArchiveOptions struct {
    // SpecsDir is the directory containing specs (default: "specs")
    SpecsDir string

    // DryRun if true, prints what would be moved without moving
    DryRun bool

    // Verbose enables verbose output
    Verbose bool
}

// Archive moves completed specs to specs/completed/.
// Returns the list of archived file names.
func Archive(opts ArchiveOptions) ([]string, error) {
    // Determine source and destination
    srcDir := opts.SpecsDir
    if srcDir == "" {
        srcDir = "specs"
    }
    dstDir := filepath.Join(srcDir, "completed")

    // Ensure source directory exists
    if _, err := os.Stat(srcDir); os.IsNotExist(err) {
        return nil, fmt.Errorf("specs directory does not exist: %s", srcDir)
    }

    // Ensure completed directory exists
    if err := os.MkdirAll(dstDir, 0755); err != nil {
        return nil, fmt.Errorf("failed to create completed directory: %w", err)
    }

    // Find spec files to archive
    entries, err := os.ReadDir(srcDir)
    if err != nil {
        return nil, fmt.Errorf("failed to read specs directory: %w", err)
    }

    var archived []string
    for _, entry := range entries {
        // Skip directories and non-markdown files
        if entry.IsDir() || !strings.HasSuffix(entry.Name(), ".md") {
            continue
        }

        // Skip README.md
        if strings.EqualFold(entry.Name(), "readme.md") {
            continue
        }

        srcPath := filepath.Join(srcDir, entry.Name())
        if shouldArchive(srcPath) {
            dstPath := filepath.Join(dstDir, entry.Name())

            if opts.DryRun {
                if opts.Verbose {
                    log.Printf("[dry-run] Would move %s -> %s", srcPath, dstPath)
                }
                archived = append(archived, entry.Name())
                continue
            }

            if err := os.Rename(srcPath, dstPath); err != nil {
                return archived, fmt.Errorf("failed to move %s: %w", entry.Name(), err)
            }

            if opts.Verbose {
                log.Printf("Archived: %s", entry.Name())
            }
            archived = append(archived, entry.Name())
        }
    }

    if len(archived) > 0 && !opts.DryRun {
        log.Printf("Archived %d specs: %v", len(archived), archived)
    } else if len(archived) == 0 {
        log.Printf("No specs to archive")
    }

    return archived, nil
}

// shouldArchive checks if a spec file should be archived.
// A spec is archived if its frontmatter contains "status: complete".
func shouldArchive(path string) bool {
    data, err := os.ReadFile(path)
    if err != nil {
        return false
    }

    content := string(data)

    // Check for YAML frontmatter
    if !strings.HasPrefix(content, "---") {
        return false
    }

    // Find end of frontmatter
    endIdx := strings.Index(content[3:], "---")
    if endIdx == -1 {
        return false
    }

    frontmatter := content[3 : 3+endIdx]

    // Check for status: complete (with various whitespace patterns)
    lines := strings.Split(frontmatter, "\n")
    for _, line := range lines {
        line = strings.TrimSpace(line)
        if strings.HasPrefix(line, "status:") {
            value := strings.TrimSpace(strings.TrimPrefix(line, "status:"))
            return value == "complete"
        }
    }

    return false
}

// ArchiveTasksDir archives completed task directories.
// This moves entire unit directories when all tasks are complete.
func ArchiveTasksDir(specsDir string, dryRun bool) ([]string, error) {
    tasksDir := filepath.Join(specsDir, "tasks")
    completedDir := filepath.Join(specsDir, "completed", "tasks")

    if _, err := os.Stat(tasksDir); os.IsNotExist(err) {
        return nil, nil // No tasks directory, nothing to archive
    }

    // Ensure completed/tasks directory exists
    if err := os.MkdirAll(completedDir, 0755); err != nil {
        return nil, fmt.Errorf("failed to create completed tasks directory: %w", err)
    }

    entries, err := os.ReadDir(tasksDir)
    if err != nil {
        return nil, fmt.Errorf("failed to read tasks directory: %w", err)
    }

    var archived []string
    for _, entry := range entries {
        if !entry.IsDir() {
            continue
        }

        unitDir := filepath.Join(tasksDir, entry.Name())
        if isUnitComplete(unitDir) {
            dstDir := filepath.Join(completedDir, entry.Name())

            if dryRun {
                log.Printf("[dry-run] Would move %s -> %s", unitDir, dstDir)
                archived = append(archived, entry.Name())
                continue
            }

            if err := os.Rename(unitDir, dstDir); err != nil {
                return archived, fmt.Errorf("failed to move %s: %w", entry.Name(), err)
            }

            log.Printf("Archived task unit: %s", entry.Name())
            archived = append(archived, entry.Name())
        }
    }

    return archived, nil
}

// isUnitComplete checks if all tasks in a unit directory are complete.
func isUnitComplete(unitDir string) bool {
    entries, err := os.ReadDir(unitDir)
    if err != nil {
        return false
    }

    taskCount := 0
    completeCount := 0

    for _, entry := range entries {
        if entry.IsDir() || !strings.HasSuffix(entry.Name(), ".md") {
            continue
        }

        // Skip IMPLEMENTATION_PLAN.md for task counting
        if entry.Name() == "IMPLEMENTATION_PLAN.md" {
            continue
        }

        taskCount++
        path := filepath.Join(unitDir, entry.Name())
        if shouldArchive(path) {
            completeCount++
        }
    }

    // Unit is complete if there are tasks and all are complete
    return taskCount > 0 && taskCount == completeCount
}
```

## Backpressure

### Validation Command

```bash
go test ./internal/cli/... -run TestArchive
```

### Must Pass

| Test | Assertion |
|------|-----------|
| `TestArchive_MovesCompletedSpecs` | Completed spec moved to completed/ |
| `TestArchive_SkipsIncompleteSpecs` | In-progress spec stays in place |
| `TestArchive_SkipsNoFrontmatter` | File without frontmatter not moved |
| `TestArchive_SkipsReadme` | README.md is never moved |
| `TestArchive_CreatesCompletedDir` | Creates completed/ if missing |
| `TestArchive_DryRunNoMove` | Dry run doesn't move files |
| `TestArchive_ReturnsArchivedList` | Returns list of archived files |
| `TestShouldArchive_CompleteStatus` | status: complete returns true |
| `TestShouldArchive_PendingStatus` | status: pending returns false |
| `TestShouldArchive_NoFrontmatter` | No frontmatter returns false |
| `TestIsUnitComplete_AllComplete` | Unit with all complete tasks returns true |
| `TestIsUnitComplete_SomePending` | Unit with pending tasks returns false |

### Test Implementations

```go
// internal/cli/archive_test.go

package cli

import (
    "os"
    "path/filepath"
    "testing"
)

func TestArchive_MovesCompletedSpecs(t *testing.T) {
    tmpDir := t.TempDir()
    specsDir := filepath.Join(tmpDir, "specs")
    os.MkdirAll(specsDir, 0755)

    completeSpec := `---
status: complete
---
# Complete Spec`
    os.WriteFile(filepath.Join(specsDir, "COMPLETE.md"), []byte(completeSpec), 0644)

    archived, err := Archive(ArchiveOptions{SpecsDir: specsDir})
    if err != nil {
        t.Fatalf("Archive() error = %v", err)
    }

    // Verify complete spec moved
    completedDir := filepath.Join(specsDir, "completed")
    if _, err := os.Stat(filepath.Join(completedDir, "COMPLETE.md")); os.IsNotExist(err) {
        t.Error("COMPLETE.md should be in completed directory")
    }
    if _, err := os.Stat(filepath.Join(specsDir, "COMPLETE.md")); !os.IsNotExist(err) {
        t.Error("COMPLETE.md should not be in specs directory")
    }

    if len(archived) != 1 || archived[0] != "COMPLETE.md" {
        t.Errorf("archived = %v, want [COMPLETE.md]", archived)
    }
}

func TestArchive_SkipsIncompleteSpecs(t *testing.T) {
    tmpDir := t.TempDir()
    specsDir := filepath.Join(tmpDir, "specs")
    os.MkdirAll(specsDir, 0755)

    incompleteSpec := `---
status: in_progress
---
# Incomplete Spec`
    os.WriteFile(filepath.Join(specsDir, "INCOMPLETE.md"), []byte(incompleteSpec), 0644)

    archived, err := Archive(ArchiveOptions{SpecsDir: specsDir})
    if err != nil {
        t.Fatalf("Archive() error = %v", err)
    }

    // Verify incomplete spec not moved
    if _, err := os.Stat(filepath.Join(specsDir, "INCOMPLETE.md")); os.IsNotExist(err) {
        t.Error("INCOMPLETE.md should remain in specs directory")
    }

    if len(archived) != 0 {
        t.Errorf("archived = %v, want []", archived)
    }
}

func TestArchive_SkipsReadme(t *testing.T) {
    tmpDir := t.TempDir()
    specsDir := filepath.Join(tmpDir, "specs")
    os.MkdirAll(specsDir, 0755)

    readme := `---
status: complete
---
# README`
    os.WriteFile(filepath.Join(specsDir, "README.md"), []byte(readme), 0644)

    archived, err := Archive(ArchiveOptions{SpecsDir: specsDir})
    if err != nil {
        t.Fatalf("Archive() error = %v", err)
    }

    // Verify README not moved
    if _, err := os.Stat(filepath.Join(specsDir, "README.md")); os.IsNotExist(err) {
        t.Error("README.md should remain in specs directory")
    }

    if len(archived) != 0 {
        t.Errorf("archived = %v, want []", archived)
    }
}

func TestArchive_DryRunNoMove(t *testing.T) {
    tmpDir := t.TempDir()
    specsDir := filepath.Join(tmpDir, "specs")
    os.MkdirAll(specsDir, 0755)

    completeSpec := `---
status: complete
---
# Complete Spec`
    os.WriteFile(filepath.Join(specsDir, "COMPLETE.md"), []byte(completeSpec), 0644)

    archived, err := Archive(ArchiveOptions{SpecsDir: specsDir, DryRun: true})
    if err != nil {
        t.Fatalf("Archive() error = %v", err)
    }

    // Verify file not actually moved
    if _, err := os.Stat(filepath.Join(specsDir, "COMPLETE.md")); os.IsNotExist(err) {
        t.Error("COMPLETE.md should remain in specs directory during dry run")
    }

    // But it should be reported as archived
    if len(archived) != 1 {
        t.Errorf("archived = %v, want [COMPLETE.md]", archived)
    }
}

func TestShouldArchive_CompleteStatus(t *testing.T) {
    tmpDir := t.TempDir()
    path := filepath.Join(tmpDir, "test.md")

    content := `---
status: complete
---
# Test`
    os.WriteFile(path, []byte(content), 0644)

    if !shouldArchive(path) {
        t.Error("shouldArchive() = false, want true for status: complete")
    }
}

func TestShouldArchive_PendingStatus(t *testing.T) {
    tmpDir := t.TempDir()
    path := filepath.Join(tmpDir, "test.md")

    content := `---
status: pending
---
# Test`
    os.WriteFile(path, []byte(content), 0644)

    if shouldArchive(path) {
        t.Error("shouldArchive() = true, want false for status: pending")
    }
}

func TestShouldArchive_NoFrontmatter(t *testing.T) {
    tmpDir := t.TempDir()
    path := filepath.Join(tmpDir, "test.md")

    content := `# Test without frontmatter`
    os.WriteFile(path, []byte(content), 0644)

    if shouldArchive(path) {
        t.Error("shouldArchive() = true, want false for no frontmatter")
    }
}

func TestIsUnitComplete_AllComplete(t *testing.T) {
    tmpDir := t.TempDir()
    unitDir := filepath.Join(tmpDir, "test-unit")
    os.MkdirAll(unitDir, 0755)

    plan := `---
unit: test-unit
---
# Plan`
    task := `---
task: 1
status: complete
---
# Task`

    os.WriteFile(filepath.Join(unitDir, "IMPLEMENTATION_PLAN.md"), []byte(plan), 0644)
    os.WriteFile(filepath.Join(unitDir, "01-task.md"), []byte(task), 0644)

    if !isUnitComplete(unitDir) {
        t.Error("isUnitComplete() = false, want true when all tasks complete")
    }
}

func TestIsUnitComplete_SomePending(t *testing.T) {
    tmpDir := t.TempDir()
    unitDir := filepath.Join(tmpDir, "test-unit")
    os.MkdirAll(unitDir, 0755)

    completeTask := `---
task: 1
status: complete
---
# Task 1`
    pendingTask := `---
task: 2
status: pending
---
# Task 2`

    os.WriteFile(filepath.Join(unitDir, "01-task.md"), []byte(completeTask), 0644)
    os.WriteFile(filepath.Join(unitDir, "02-task.md"), []byte(pendingTask), 0644)

    if isUnitComplete(unitDir) {
        t.Error("isUnitComplete() = true, want false when some tasks pending")
    }
}
```

### CI Compatibility

- [x] No external API keys required
- [x] No network access required
- [x] Runs in <60 seconds

## Implementation Notes

- Uses simple string-based frontmatter parsing (no YAML library needed)
- README.md is always skipped regardless of status
- Creates completed/ directory if it doesn't exist
- Dry run mode for testing without modifications
- Returns list of archived files for reporting

## NOT In Scope

- Git commit of archive changes (Task #6)
- Git push of archive changes (Task #6)
- CLI command registration (separate CLI wiring)
- Archive of task directories (helper function provided but not wired to CLI)
