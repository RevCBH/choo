package orchestrator

import (
	"context"
	"fmt"
	"log"
	"os"
	"os/exec"
	"strings"

	"github.com/RevCBH/choo/internal/archive"
)

// Complete runs the completion flow after all units have merged.
// This includes: archive, commit, push, and PR creation.
func (o *Orchestrator) Complete(ctx context.Context) error {
	log.Printf("All units complete, running completion flow")

	if err := o.runArchive(ctx); err != nil {
		return fmt.Errorf("archive step failed: %w", err)
	}

	if err := o.commitArchive(ctx); err != nil {
		return fmt.Errorf("commit step failed: %w", err)
	}

	if o.cfg.NoPR {
		if err := o.pushFeatureBranch(ctx); err != nil {
			return fmt.Errorf("push step failed: %w", err)
		}
		return nil
	}

	if _, err := o.createFeaturePR(ctx); err != nil {
		return fmt.Errorf("PR creation failed: %w", err)
	}

	return nil
}

// runArchive moves completed specs to specs/completed/.
func (o *Orchestrator) runArchive(ctx context.Context) error {
	specsDir := o.cfg.TasksDir
	if specsDir == "" {
		specsDir = "specs/tasks"
	}

	specsDir = strings.TrimSuffix(specsDir, "/tasks")

	opts := archive.ArchiveOptions{
		SpecsDir: specsDir,
		Verbose:  true,
	}

	archived, err := archive.Archive(opts)
	if err != nil {
		return err
	}

	taskArchived, err := archive.ArchiveTasksDir(specsDir, false)
	if err != nil {
		return err
	}

	if len(archived) == 0 && len(taskArchived) == 0 {
		log.Printf("No specs to archive")
	}

	return nil
}

// commitArchive commits the archived specs with a standard message.
func (o *Orchestrator) commitArchive(ctx context.Context) error {
	statusCmd := exec.CommandContext(ctx, "git", "status", "--porcelain")
	statusCmd.Dir = o.cfg.RepoRoot
	output, err := statusCmd.Output()
	if err != nil {
		return fmt.Errorf("git status failed: %w", err)
	}

	if len(strings.TrimSpace(string(output))) == 0 {
		log.Printf("No archive changes to commit")
		return nil
	}

	addCmd := exec.CommandContext(ctx, "git", "add", "-A")
	addCmd.Dir = o.cfg.RepoRoot
	addCmd.Stdout = os.Stdout
	addCmd.Stderr = os.Stderr
	if err := addCmd.Run(); err != nil {
		return fmt.Errorf("git add failed: %w", err)
	}

	commitCmd := exec.CommandContext(ctx, "git", "commit", "-m", "chore: archive completed specs")
	commitCmd.Dir = o.cfg.RepoRoot
	commitCmd.Stdout = os.Stdout
	commitCmd.Stderr = os.Stderr
	if err := commitCmd.Run(); err != nil {
		if isNoChangesError(err) {
			log.Printf("No changes to commit after staging")
			return nil
		}
		return fmt.Errorf("git commit failed: %w", err)
	}

	log.Printf("Committed archive changes")
	return nil
}

// buildPRBody constructs the PR description.
func (o *Orchestrator) buildPRBody() string {
	var sb strings.Builder

	sb.WriteString("## Summary\n\n")
	sb.WriteString(o.cfg.FeatureDescription)
	sb.WriteString("\n\n")

	sb.WriteString("## Units Implemented\n\n")
	units := o.completedUnits
	if len(units) == 0 {
		for _, unit := range o.units {
			units = append(units, unit.ID)
		}
	}
	for _, unit := range units {
		sb.WriteString(fmt.Sprintf("- [x] %s\n", unit))
	}
	sb.WriteString("\n")

	sb.WriteString("## Automated Implementation\n\n")
	sb.WriteString("This PR was created by the choo orchestrator.\n")
	sb.WriteString("All unit branches have been merged and specs archived.\n")

	return sb.String()
}

// isNoChangesError checks if an error is due to no changes to commit.
func isNoChangesError(err error) bool {
	if exitErr, ok := err.(*exec.ExitError); ok {
		return exitErr.ExitCode() == 1
	}
	return false
}
