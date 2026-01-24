package specs

import (
	"context"
	"fmt"
	"path/filepath"
	"strings"

	"github.com/RevCBH/choo/internal/config"
	"github.com/RevCBH/choo/internal/discovery"
	"github.com/RevCBH/choo/internal/events"
	"github.com/RevCBH/choo/internal/git"
)

// PreflightOptions configures spec validation/normalization/repair.
type PreflightOptions struct {
	RepoRoot     string
	TasksDir     string
	Normalize    bool
	Repair       bool
	DryRun       bool
	RepairConfig config.SpecRepairConfig
	Bus          *events.Bus
}

// ValidationError is returned when specs are non-conforming.
type ValidationError struct {
	Report *Report
}

func (e *ValidationError) Error() string {
	if e.Report == nil || len(e.Report.Errors) == 0 {
		return "spec validation failed"
	}
	var lines []string
	for _, issue := range e.Report.Errors {
		lines = append(lines, fmt.Sprintf("%s: %s", issue.Path, issue.Message))
	}
	return "spec validation failed:\n" + strings.Join(lines, "\n")
}

// Preflight validates and optionally normalizes/repairs specs before execution.
func Preflight(ctx context.Context, opts PreflightOptions) (*Report, error) {
	emit(opts.Bus, events.SpecValidationStarted, map[string]any{
		"tasks_dir": opts.TasksDir,
	}, nil)

	report, err := Normalize(NormalizeOptions{
		TasksDir: opts.TasksDir,
		RepoRoot: opts.RepoRoot,
		Apply:    false,
	})
	if err != nil {
		emit(opts.Bus, events.SpecValidationFailed, map[string]any{"error": err.Error()}, err)
		return nil, err
	}

	repairApplied := false
	if report.HasErrors() {
		if opts.Repair && !opts.DryRun {
			if err := ensureCleanRepo(ctx, opts.RepoRoot); err != nil {
				emit(opts.Bus, events.SpecValidationFailed, reportPayload(report), err)
				return nil, err
			}
			repaired, err := repairFiles(ctx, opts, report.Errors)
			if err != nil {
				emit(opts.Bus, events.SpecRepairFailed, map[string]any{"error": err.Error()}, err)
				emit(opts.Bus, events.SpecValidationFailed, reportPayload(report), err)
				return nil, err
			}
			if len(repaired) > 0 {
				repairApplied = true
				emit(opts.Bus, events.SpecRepairApplied, map[string]any{
					"count": len(repaired),
					"files": repaired,
				}, nil)
			}
			report, err = Normalize(NormalizeOptions{
				TasksDir: opts.TasksDir,
				RepoRoot: opts.RepoRoot,
				Apply:    false,
			})
			if err != nil {
				emit(opts.Bus, events.SpecValidationFailed, map[string]any{"error": err.Error()}, err)
				return nil, err
			}
		} else {
			emit(opts.Bus, events.SpecValidationFailed, reportPayload(report), nil)
			return report, &ValidationError{Report: report}
		}
	}

	if report.HasErrors() {
		emit(opts.Bus, events.SpecValidationFailed, reportPayload(report), nil)
		return report, &ValidationError{Report: report}
	}

	normalizedApplied := false
	if opts.Normalize && !opts.DryRun && len(report.Warnings) > 0 {
		if err := ensureCleanRepo(ctx, opts.RepoRoot); err != nil {
			emit(opts.Bus, events.SpecValidationFailed, reportPayload(report), err)
			return nil, err
		}
		report, err = Normalize(NormalizeOptions{
			TasksDir: opts.TasksDir,
			RepoRoot: opts.RepoRoot,
			Apply:    true,
		})
		if err != nil {
			emit(opts.Bus, events.SpecValidationFailed, map[string]any{"error": err.Error()}, err)
			return nil, err
		}
		if report.HasChanges() {
			normalizedApplied = true
			emit(opts.Bus, events.SpecNormalizationApplied, reportPayload(report), nil)
		}
	}

	emit(opts.Bus, events.SpecValidationCompleted, reportPayload(report), nil)

	if normalizedApplied || repairApplied {
		if err := commitSpecChanges(ctx, opts.RepoRoot, opts.TasksDir); err != nil {
			return report, err
		}
	}

	return report, nil
}

func repairFiles(ctx context.Context, opts PreflightOptions, issues []FileIssue) ([]map[string]any, error) {
	results, err := RepairIssues(ctx, RepairBatchOptions{
		RepoRoot: opts.RepoRoot,
		Config:   opts.RepairConfig,
	}, issues)
	if err != nil {
		return nil, err
	}

	issueByPath := make(map[string]FileIssue, len(issues))
	for _, issue := range issues {
		key := issue.Path
		if !filepath.IsAbs(key) && opts.RepoRoot != "" {
			key = normalizePath(filepath.Join(opts.RepoRoot, key), opts.RepoRoot)
		} else {
			key = normalizePath(key, opts.RepoRoot)
		}
		if _, ok := issueByPath[key]; ok {
			continue
		}
		issueByPath[key] = issue
	}

	var repaired []map[string]any
	for _, result := range results {
		issue, ok := issueByPath[result.Path]
		if !ok {
			issue = FileIssue{
				Path:   result.Path,
				Kind:   result.Kind,
				Source: discovery.MetadataSourceNone,
			}
		}
		repaired = append(repaired, map[string]any{
			"path":   result.Path,
			"kind":   issue.Kind,
			"source": issue.Source,
		})
	}

	return repaired, nil
}

func ensureCleanRepo(ctx context.Context, repoRoot string) error {
	status, err := git.GetWorkingDirStatus(ctx, repoRoot)
	if err != nil {
		return fmt.Errorf("failed to check git status: %w", err)
	}
	if status.HasChanges {
		var lines []string
		for _, file := range status.ChangedFiles {
			lines = append(lines, "  "+file)
		}
		return fmt.Errorf("working directory has uncommitted changes:\n%s", strings.Join(lines, "\n"))
	}
	return nil
}

func commitSpecChanges(ctx context.Context, repoRoot string, tasksDir string) error {
	stagePath := tasksDir
	if filepath.IsAbs(stagePath) {
		if rel, err := filepath.Rel(repoRoot, stagePath); err == nil {
			stagePath = rel
		}
	}

	if err := git.StagePath(ctx, repoRoot, stagePath); err != nil {
		return fmt.Errorf("failed to stage spec changes: %w", err)
	}
	staged, err := git.GetStagedFiles(ctx, repoRoot)
	if err != nil {
		return fmt.Errorf("failed to list staged files: %w", err)
	}
	if len(staged) == 0 {
		return nil
	}
	if err := git.Commit(ctx, repoRoot, git.CommitOptions{
		Message:  "chore: normalize spec metadata",
		NoVerify: true,
	}); err != nil {
		return fmt.Errorf("failed to commit spec normalization: %w", err)
	}
	return nil
}

func reportPayload(report *Report) map[string]any {
	if report == nil {
		return map[string]any{}
	}

	return map[string]any{
		"counts": map[string]any{
			"tasks":      report.Tasks,
			"units":      report.Units,
			"errors":     len(report.Errors),
			"warnings":   len(report.Warnings),
			"normalized": len(report.Normalized),
		},
		"errors":     issuesPayload(report.Errors),
		"warnings":   issuesPayload(report.Warnings),
		"normalized": issuesPayload(report.Normalized),
	}
}

func issuesPayload(issues []FileIssue) []map[string]any {
	if len(issues) == 0 {
		return nil
	}
	payload := make([]map[string]any, 0, len(issues))
	for _, issue := range issues {
		payload = append(payload, map[string]any{
			"path":   issue.Path,
			"kind":   issue.Kind,
			"source": issue.Source,
			"detail": issue.Message,
		})
	}
	return payload
}

func resolvePath(opts PreflightOptions, path string) string {
	if filepath.IsAbs(path) || opts.RepoRoot == "" {
		return path
	}
	return filepath.Join(opts.RepoRoot, path)
}

func emit(bus *events.Bus, eventType events.EventType, payload any, err error) {
	if bus == nil {
		return
	}
	bus.Emit(events.NewEvent(eventType, "").WithPayload(payload).WithError(err))
}
