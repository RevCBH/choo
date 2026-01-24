package specs

import (
	"bytes"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"regexp"
	"strings"

	"github.com/RevCBH/choo/internal/discovery"
)

// FileKind identifies the spec file type.
type FileKind string

const (
	FileKindTask FileKind = "task"
	FileKindUnit FileKind = "unit_plan"
)

// FileIssue represents a warning or error tied to a specific file.
type FileIssue struct {
	Path    string
	Kind    FileKind
	Source  discovery.MetadataSource
	Message string
}

// Report summarizes validation/normalization results.
type Report struct {
	Tasks      int
	Units      int
	Normalized []FileIssue
	Warnings   []FileIssue
	Errors     []FileIssue
}

// HasErrors returns true if the report contains errors.
func (r *Report) HasErrors() bool {
	return len(r.Errors) > 0
}

// HasChanges returns true if any files were normalized.
func (r *Report) HasChanges() bool {
	return len(r.Normalized) > 0
}

// NormalizeOptions configures spec normalization.
type NormalizeOptions struct {
	TasksDir string
	RepoRoot string
	Apply    bool
}

// Normalize scans task/unit specs for metadata conformance.
// When Apply is true, non-canonical metadata blocks are rewritten to frontmatter.
func Normalize(opts NormalizeOptions) (*Report, error) {
	if opts.TasksDir == "" {
		return nil, fmt.Errorf("tasks directory is required")
	}
	if _, err := os.Stat(opts.TasksDir); err != nil {
		return nil, fmt.Errorf("tasks directory error: %w", err)
	}

	report := &Report{}

	taskFiles, unitFiles, err := collectSpecFiles(opts.TasksDir)
	if err != nil {
		return nil, err
	}

	for _, path := range unitFiles {
		report.Units++
		processUnitFile(path, opts, report)
	}
	for _, path := range taskFiles {
		report.Tasks++
		processTaskFile(path, opts, report)
	}

	return report, nil
}

func collectSpecFiles(tasksDir string) ([]string, []string, error) {
	taskPattern := regexp.MustCompile(`^[0-9][0-9]-.*\.md$`)
	var taskFiles []string
	var unitFiles []string

	err := filepath.WalkDir(tasksDir, func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if d.IsDir() {
			return nil
		}
		name := d.Name()
		if name == "IMPLEMENTATION_PLAN.md" {
			unitFiles = append(unitFiles, path)
			return nil
		}
		if taskPattern.MatchString(name) {
			taskFiles = append(taskFiles, path)
		}
		return nil
	})
	if err != nil {
		return nil, nil, fmt.Errorf("scan tasks directory: %w", err)
	}

	return taskFiles, unitFiles, nil
}

func processTaskFile(path string, opts NormalizeOptions, report *Report) {
	content, err := os.ReadFile(path)
	if err != nil {
		report.Errors = append(report.Errors, FileIssue{
			Path:    normalizePath(path, opts.RepoRoot),
			Kind:    FileKindTask,
			Source:  discovery.MetadataSourceNone,
			Message: fmt.Sprintf("read file: %v", err),
		})
		return
	}

	if bytes.HasPrefix(content, []byte("---\n")) {
		handleTaskFrontmatter(path, content, opts, report)
		return
	}

	handleTaskMetadataBlock(path, content, opts, report)
}

func handleTaskFrontmatter(path string, content []byte, opts NormalizeOptions, report *Report) {
	fm, _, err := discovery.ParseFrontmatter(content)
	if err != nil {
		report.Errors = append(report.Errors, FileIssue{
			Path:    normalizePath(path, opts.RepoRoot),
			Kind:    FileKindTask,
			Source:  discovery.MetadataSourceFrontmatter,
			Message: fmt.Sprintf("frontmatter parse: %v", err),
		})
		return
	}

	taskMeta, err := discovery.ParseTaskFrontmatter(fm)
	if err != nil {
		report.Errors = append(report.Errors, FileIssue{
			Path:    normalizePath(path, opts.RepoRoot),
			Kind:    FileKindTask,
			Source:  discovery.MetadataSourceFrontmatter,
			Message: fmt.Sprintf("frontmatter decode: %v", err),
		})
		return
	}
	if err := validateTaskFrontmatter(taskMeta); err != nil {
		report.Errors = append(report.Errors, FileIssue{
			Path:    normalizePath(path, opts.RepoRoot),
			Kind:    FileKindTask,
			Source:  discovery.MetadataSourceFrontmatter,
			Message: fmt.Sprintf("frontmatter invalid: %v", err),
		})
		return
	}

	block, blockErr := discovery.FindMetadataBlock(content)
	if blockErr != nil {
		report.Warnings = append(report.Warnings, FileIssue{
			Path:    normalizePath(path, opts.RepoRoot),
			Kind:    FileKindTask,
			Source:  discovery.MetadataSourceFrontmatter,
			Message: fmt.Sprintf("metadata block ignored (frontmatter wins): %v", blockErr),
		})
		return
	}
	if block == nil {
		return
	}

	report.Warnings = append(report.Warnings, FileIssue{
		Path:    normalizePath(path, opts.RepoRoot),
		Kind:    FileKindTask,
		Source:  discovery.MetadataSourceFrontmatter,
		Message: "metadata block ignored (frontmatter wins)",
	})

	if opts.Apply {
		normalized := discovery.RemoveMetadataBlock(content, block)
		if !bytes.Equal(content, normalized) {
			if writeErr := os.WriteFile(path, normalized, 0644); writeErr != nil {
				report.Errors = append(report.Errors, FileIssue{
					Path:    normalizePath(path, opts.RepoRoot),
					Kind:    FileKindTask,
					Source:  discovery.MetadataSourceFrontmatter,
					Message: fmt.Sprintf("write normalized file: %v", writeErr),
				})
				return
			}
			report.Normalized = append(report.Normalized, FileIssue{
				Path:    normalizePath(path, opts.RepoRoot),
				Kind:    FileKindTask,
				Source:  discovery.MetadataSourceFrontmatter,
				Message: "removed metadata block",
			})
		}
	}
}

func handleTaskMetadataBlock(path string, content []byte, opts NormalizeOptions, report *Report) {
	block, err := discovery.FindMetadataBlock(content)
	if err != nil {
		report.Errors = append(report.Errors, FileIssue{
			Path:    normalizePath(path, opts.RepoRoot),
			Kind:    FileKindTask,
			Source:  discovery.MetadataSourceMetadataBlock,
			Message: fmt.Sprintf("metadata block parse: %v", err),
		})
		return
	}
	if block == nil {
		report.Errors = append(report.Errors, FileIssue{
			Path:    normalizePath(path, opts.RepoRoot),
			Kind:    FileKindTask,
			Source:  discovery.MetadataSourceNone,
			Message: "missing metadata",
		})
		return
	}

	taskMeta, err := discovery.ParseTaskFrontmatter(block.YAML)
	if err != nil {
		report.Errors = append(report.Errors, FileIssue{
			Path:    normalizePath(path, opts.RepoRoot),
			Kind:    FileKindTask,
			Source:  discovery.MetadataSourceMetadataBlock,
			Message: fmt.Sprintf("metadata block decode: %v", err),
		})
		return
	}
	if err := validateTaskFrontmatter(taskMeta); err != nil {
		report.Errors = append(report.Errors, FileIssue{
			Path:    normalizePath(path, opts.RepoRoot),
			Kind:    FileKindTask,
			Source:  discovery.MetadataSourceMetadataBlock,
			Message: fmt.Sprintf("metadata block invalid: %v", err),
		})
		return
	}

	report.Warnings = append(report.Warnings, FileIssue{
		Path:    normalizePath(path, opts.RepoRoot),
		Kind:    FileKindTask,
		Source:  discovery.MetadataSourceMetadataBlock,
		Message: "metadata block is non-canonical",
	})

	if opts.Apply {
		body := discovery.RemoveMetadataBlock(content, block)
		frontmatter := buildFrontmatter(block.YAML)
		normalized := append(frontmatter, body...)
		if writeErr := os.WriteFile(path, normalized, 0644); writeErr != nil {
			report.Errors = append(report.Errors, FileIssue{
				Path:    normalizePath(path, opts.RepoRoot),
				Kind:    FileKindTask,
				Source:  discovery.MetadataSourceMetadataBlock,
				Message: fmt.Sprintf("write normalized file: %v", writeErr),
			})
			return
		}
		report.Normalized = append(report.Normalized, FileIssue{
			Path:    normalizePath(path, opts.RepoRoot),
			Kind:    FileKindTask,
			Source:  discovery.MetadataSourceMetadataBlock,
			Message: "normalized metadata block to frontmatter",
		})
	}
}

func processUnitFile(path string, opts NormalizeOptions, report *Report) {
	content, err := os.ReadFile(path)
	if err != nil {
		report.Errors = append(report.Errors, FileIssue{
			Path:    normalizePath(path, opts.RepoRoot),
			Kind:    FileKindUnit,
			Source:  discovery.MetadataSourceNone,
			Message: fmt.Sprintf("read file: %v", err),
		})
		return
	}

	if bytes.HasPrefix(content, []byte("---\n")) {
		handleUnitFrontmatter(path, content, opts, report)
		return
	}

	handleUnitMetadataBlock(path, content, opts, report)
}

func handleUnitFrontmatter(path string, content []byte, opts NormalizeOptions, report *Report) {
	fm, _, err := discovery.ParseFrontmatter(content)
	if err != nil {
		report.Errors = append(report.Errors, FileIssue{
			Path:    normalizePath(path, opts.RepoRoot),
			Kind:    FileKindUnit,
			Source:  discovery.MetadataSourceFrontmatter,
			Message: fmt.Sprintf("frontmatter parse: %v", err),
		})
		return
	}

	unitMeta, err := discovery.ParseUnitFrontmatter(fm)
	if err != nil {
		report.Errors = append(report.Errors, FileIssue{
			Path:    normalizePath(path, opts.RepoRoot),
			Kind:    FileKindUnit,
			Source:  discovery.MetadataSourceFrontmatter,
			Message: fmt.Sprintf("frontmatter decode: %v", err),
		})
		return
	}
	if err := validateUnitFrontmatter(unitMeta); err != nil {
		report.Errors = append(report.Errors, FileIssue{
			Path:    normalizePath(path, opts.RepoRoot),
			Kind:    FileKindUnit,
			Source:  discovery.MetadataSourceFrontmatter,
			Message: fmt.Sprintf("frontmatter invalid: %v", err),
		})
		return
	}

	block, blockErr := discovery.FindMetadataBlock(content)
	if blockErr != nil {
		report.Warnings = append(report.Warnings, FileIssue{
			Path:    normalizePath(path, opts.RepoRoot),
			Kind:    FileKindUnit,
			Source:  discovery.MetadataSourceFrontmatter,
			Message: fmt.Sprintf("metadata block ignored (frontmatter wins): %v", blockErr),
		})
		return
	}
	if block == nil {
		return
	}

	report.Warnings = append(report.Warnings, FileIssue{
		Path:    normalizePath(path, opts.RepoRoot),
		Kind:    FileKindUnit,
		Source:  discovery.MetadataSourceFrontmatter,
		Message: "metadata block ignored (frontmatter wins)",
	})

	if opts.Apply {
		normalized := discovery.RemoveMetadataBlock(content, block)
		if !bytes.Equal(content, normalized) {
			if writeErr := os.WriteFile(path, normalized, 0644); writeErr != nil {
				report.Errors = append(report.Errors, FileIssue{
					Path:    normalizePath(path, opts.RepoRoot),
					Kind:    FileKindUnit,
					Source:  discovery.MetadataSourceFrontmatter,
					Message: fmt.Sprintf("write normalized file: %v", writeErr),
				})
				return
			}
			report.Normalized = append(report.Normalized, FileIssue{
				Path:    normalizePath(path, opts.RepoRoot),
				Kind:    FileKindUnit,
				Source:  discovery.MetadataSourceFrontmatter,
				Message: "removed metadata block",
			})
		}
	}
}

func handleUnitMetadataBlock(path string, content []byte, opts NormalizeOptions, report *Report) {
	block, err := discovery.FindMetadataBlock(content)
	if err != nil {
		report.Errors = append(report.Errors, FileIssue{
			Path:    normalizePath(path, opts.RepoRoot),
			Kind:    FileKindUnit,
			Source:  discovery.MetadataSourceMetadataBlock,
			Message: fmt.Sprintf("metadata block parse: %v", err),
		})
		return
	}
	if block == nil {
		report.Errors = append(report.Errors, FileIssue{
			Path:    normalizePath(path, opts.RepoRoot),
			Kind:    FileKindUnit,
			Source:  discovery.MetadataSourceNone,
			Message: "missing metadata",
		})
		return
	}

	unitMeta, err := discovery.ParseUnitFrontmatter(block.YAML)
	if err != nil {
		report.Errors = append(report.Errors, FileIssue{
			Path:    normalizePath(path, opts.RepoRoot),
			Kind:    FileKindUnit,
			Source:  discovery.MetadataSourceMetadataBlock,
			Message: fmt.Sprintf("metadata block decode: %v", err),
		})
		return
	}
	if err := validateUnitFrontmatter(unitMeta); err != nil {
		report.Errors = append(report.Errors, FileIssue{
			Path:    normalizePath(path, opts.RepoRoot),
			Kind:    FileKindUnit,
			Source:  discovery.MetadataSourceMetadataBlock,
			Message: fmt.Sprintf("metadata block invalid: %v", err),
		})
		return
	}

	report.Warnings = append(report.Warnings, FileIssue{
		Path:    normalizePath(path, opts.RepoRoot),
		Kind:    FileKindUnit,
		Source:  discovery.MetadataSourceMetadataBlock,
		Message: "metadata block is non-canonical",
	})

	if opts.Apply {
		body := discovery.RemoveMetadataBlock(content, block)
		frontmatter := buildFrontmatter(block.YAML)
		normalized := append(frontmatter, body...)
		if writeErr := os.WriteFile(path, normalized, 0644); writeErr != nil {
			report.Errors = append(report.Errors, FileIssue{
				Path:    normalizePath(path, opts.RepoRoot),
				Kind:    FileKindUnit,
				Source:  discovery.MetadataSourceMetadataBlock,
				Message: fmt.Sprintf("write normalized file: %v", writeErr),
			})
			return
		}
		report.Normalized = append(report.Normalized, FileIssue{
			Path:    normalizePath(path, opts.RepoRoot),
			Kind:    FileKindUnit,
			Source:  discovery.MetadataSourceMetadataBlock,
			Message: "normalized metadata block to frontmatter",
		})
	}
}

func buildFrontmatter(yamlContent []byte) []byte {
	text := strings.TrimRight(string(yamlContent), "\n")
	return []byte("---\n" + text + "\n---\n")
}

func normalizePath(path string, repoRoot string) string {
	if repoRoot == "" {
		return filepath.ToSlash(path)
	}
	if rel, err := filepath.Rel(repoRoot, path); err == nil {
		return filepath.ToSlash(rel)
	}
	return filepath.ToSlash(path)
}

func validateTaskFrontmatter(tf *discovery.TaskFrontmatter) error {
	if tf.Task < 1 {
		return fmt.Errorf("task must be >= 1")
	}
	if strings.TrimSpace(tf.Backpressure) == "" {
		return fmt.Errorf("backpressure must be set")
	}
	if tf.Status != "" {
		if err := validateTaskStatus(tf.Status); err != nil {
			return err
		}
	}
	return nil
}

func validateUnitFrontmatter(uf *discovery.UnitFrontmatter) error {
	if strings.TrimSpace(uf.Unit) == "" {
		return fmt.Errorf("unit must be set")
	}
	if uf.OrchStatus != "" {
		if err := validateUnitStatus(uf.OrchStatus); err != nil {
			return err
		}
	}
	return nil
}

func validateTaskStatus(status string) error {
	switch status {
	case "pending", "in_progress", "complete", "failed":
		return nil
	default:
		return fmt.Errorf("invalid task status: %q", status)
	}
}

func validateUnitStatus(status string) error {
	switch status {
	case "pending", "in_progress", "complete", "failed", "blocked", "pr_open", "in_review", "merging":
		return nil
	default:
		return fmt.Errorf("invalid unit status: %q", status)
	}
}
