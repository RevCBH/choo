package git

import (
	"context"
	"fmt"
	"math/rand"
	"regexp"
	"strings"
)

// WorkingDirStatus represents the result of checking for uncommitted changes.
type WorkingDirStatus struct {
	// HasChanges is true if there are any uncommitted changes
	HasChanges bool

	// ChangedFiles is a list of files with uncommitted changes
	ChangedFiles []string

	// PathChanges maps specific paths to whether they have changes
	PathChanges map[string]bool
}

// GetWorkingDirStatus checks if the working directory has uncommitted changes.
// If watchPaths are provided, it also reports which of those specific paths have changes.
// Uses "git status --porcelain" which outputs nothing for a clean working directory.
func GetWorkingDirStatus(ctx context.Context, repoDir string, watchPaths ...string) (*WorkingDirStatus, error) {
	output, err := gitExec(ctx, repoDir, "status", "--porcelain")
	if err != nil {
		return nil, fmt.Errorf("failed to check git status: %w", err)
	}

	result := &WorkingDirStatus{
		ChangedFiles: []string{},
		PathChanges:  make(map[string]bool),
	}

	// Initialize watched paths as having no changes
	for _, path := range watchPaths {
		result.PathChanges[path] = false
	}

	// Parse porcelain output - each line is a changed file
	// Format: XY filename (where XY is the 2-char status code, then a space, then the filename)
	// Note: Don't use TrimSpace on the output as the first char of each line is part of the status
	lines := strings.Split(output, "\n")
	for _, line := range lines {
		if strings.TrimSpace(line) == "" {
			continue
		}
		result.HasChanges = true

		// Extract filename (starts at position 3 after "XY ")
		if len(line) >= 3 {
			filename := line[3:]
			// Handle renamed files: "R  old -> new"
			if idx := strings.Index(filename, " -> "); idx != -1 {
				filename = filename[idx+4:]
			}
			result.ChangedFiles = append(result.ChangedFiles, filename)

			// Check if this file is under any watched path
			for _, path := range watchPaths {
				if strings.HasPrefix(filename, path) {
					result.PathChanges[path] = true
				}
			}
		}
	}

	return result, nil
}

// GetCurrentBranch returns the name of the currently checked out branch.
// Returns an error if in detached HEAD state or if the command fails.
func GetCurrentBranch(ctx context.Context, repoDir string) (string, error) {
	output, err := gitExec(ctx, repoDir, "rev-parse", "--abbrev-ref", "HEAD")
	if err != nil {
		return "", fmt.Errorf("failed to get current branch: %w", err)
	}

	branch := strings.TrimSpace(output)
	if branch == "HEAD" {
		return "", fmt.Errorf("repository is in detached HEAD state")
	}

	return branch, nil
}

// BranchExistsOnRemote checks if a branch exists on the remote (origin).
func BranchExistsOnRemote(ctx context.Context, repoDir, branch string) (bool, error) {
	output, err := gitExec(ctx, repoDir, "ls-remote", "--heads", "origin", branch)
	if err != nil {
		return false, fmt.Errorf("failed to check remote branch: %w", err)
	}

	// If output is non-empty, the branch exists
	return strings.TrimSpace(output) != "", nil
}

// PushBranch pushes a local branch to the remote with upstream tracking.
func PushBranch(ctx context.Context, repoDir, branch string) error {
	_, err := gitExec(ctx, repoDir, "push", "-u", "origin", branch)
	if err != nil {
		return fmt.Errorf("failed to push branch %s: %w", branch, err)
	}
	return nil
}

// EnsureBranchOnRemote checks if the target branch exists on the remote,
// and pushes it if not. Returns true if the branch was pushed.
func EnsureBranchOnRemote(ctx context.Context, repoDir, branch string) (pushed bool, err error) {
	exists, err := BranchExistsOnRemote(ctx, repoDir, branch)
	if err != nil {
		return false, err
	}

	if exists {
		return false, nil
	}

	// Branch doesn't exist on remote, push it
	if err := PushBranch(ctx, repoDir, branch); err != nil {
		return false, err
	}

	return true, nil
}

// Branch represents a git branch with its metadata
type Branch struct {
	// Name is the full branch name (e.g., "ralph/deck-list-sunset-harbor")
	Name string

	// UnitID is the unit this branch is for
	UnitID string

	// TargetBranch is the branch this will merge into
	TargetBranch string

	// Worktree is the absolute path to the worktree for this branch
	Worktree string
}

// ClaudeClient interface for generating branch names
type ClaudeClient interface {
	Invoke(ctx context.Context, opts InvokeOptions) (string, error)
}

// InvokeOptions configures a Claude invocation
type InvokeOptions struct {
	Prompt   string
	Model    string
	MaxTurns int
}

// BranchNamer generates creative branch names using Claude
type BranchNamer struct {
	// Claude client for name generation
	Claude ClaudeClient

	// Prefix for all branch names (default: "ralph/")
	Prefix string
}

// NewBranchNamer creates a branch namer with the given Claude client
func NewBranchNamer(claude ClaudeClient) *BranchNamer {
	return &BranchNamer{
		Claude: claude,
		Prefix: "ralph/",
	}
}

// GenerateName creates a creative branch name for a unit
// Uses Claude CLI with haiku model for short, memorable suffixes
// Falls back to random suffix if Claude fails
func (n *BranchNamer) GenerateName(ctx context.Context, unitID string) (string, error) {
	sanitizedUnitID := SanitizeBranchName(unitID)

	prompt := fmt.Sprintf(`Generate a short, memorable 2-3 word suffix for a git branch.
The branch is for a unit called "%s".
Return ONLY the suffix, lowercase, words separated by hyphens.
Examples: sunset-harbor, quick-fox, blue-mountain
No explanation, just the suffix.`, unitID)

	var suffix string

	// Try to use Claude if available
	if n.Claude != nil {
		result, err := n.Claude.Invoke(ctx, InvokeOptions{
			Prompt:   prompt,
			Model:    "claude-3-haiku-20240307",
			MaxTurns: 1,
		})
		if err == nil {
			// Successfully got response from Claude
			suffix = strings.TrimSpace(result)
			suffix = SanitizeBranchName(suffix)
		} else {
			// Claude failed, use random suffix
			suffix = randomSuffix()
		}
	} else {
		// No Claude client, use random suffix
		suffix = randomSuffix()
	}

	branchName := fmt.Sprintf("%s%s-%s", n.Prefix, sanitizedUnitID, suffix)

	if err := ValidateBranchName(branchName); err != nil {
		return "", fmt.Errorf("generated invalid branch name: %w", err)
	}

	return branchName, nil
}

// ValidateBranchName checks if a branch name is valid for git
func ValidateBranchName(name string) error {
	if name == "" {
		return fmt.Errorf("branch name cannot be empty")
	}

	if strings.HasPrefix(name, "refs/") {
		return fmt.Errorf("branch name cannot start with 'refs/'")
	}

	if strings.Contains(name, "..") {
		return fmt.Errorf("branch name cannot contain '..'")
	}

	if strings.Contains(name, " ") {
		return fmt.Errorf("branch name cannot contain spaces")
	}

	// Additional git branch name restrictions
	if strings.HasPrefix(name, "-") {
		return fmt.Errorf("branch name cannot start with '-'")
	}

	if strings.HasSuffix(name, ".") {
		return fmt.Errorf("branch name cannot end with '.'")
	}

	if strings.HasSuffix(name, ".lock") {
		return fmt.Errorf("branch name cannot end with '.lock'")
	}

	return nil
}

// SanitizeBranchName converts a string to a valid branch name component
func SanitizeBranchName(s string) string {
	// Convert to lowercase
	s = strings.ToLower(s)

	// Trim spaces
	s = strings.TrimSpace(s)

	// Replace spaces with hyphens
	s = strings.ReplaceAll(s, " ", "-")

	// Replace slashes with hyphens
	s = strings.ReplaceAll(s, "/", "-")

	// Replace consecutive dots with single hyphen
	dotsRegex := regexp.MustCompile(`\.\.+`)
	s = dotsRegex.ReplaceAllString(s, "-")

	// Replace single dots with hyphens
	s = strings.ReplaceAll(s, ".", "-")

	// Remove special characters, keep only alphanumeric and hyphens
	validCharsRegex := regexp.MustCompile(`[^a-z0-9-]+`)
	s = validCharsRegex.ReplaceAllString(s, "-")

	// Replace multiple consecutive hyphens with single hyphen
	hyphensRegex := regexp.MustCompile(`-+`)
	s = hyphensRegex.ReplaceAllString(s, "-")

	// Trim hyphens from start and end
	s = strings.Trim(s, "-")

	return s
}

// randomSuffix generates a random 6-character alphanumeric suffix
func randomSuffix() string {
	const chars = "abcdefghijklmnopqrstuvwxyz0123456789"
	b := make([]byte, 6)
	for i := range b {
		b[i] = chars[rand.Intn(len(chars))]
	}
	return string(b)
}
