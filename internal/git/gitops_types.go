package git

import "time"

// StatusResult contains the parsed output of git status.
type StatusResult struct {
	Clean      bool     // True if the working tree has no changes
	Staged     []string // Files with staged changes
	Modified   []string // Files with unstaged modifications
	Untracked  []string // Untracked files
	Conflicted []string // Files with merge conflicts
}

// CommitRecord represents a parsed git commit.
type CommitRecord struct {
	Hash    string
	Author  string
	Date    time.Time
	Subject string
	Body    string
}

// CommitOpts configures commit behavior.
type CommitOpts struct {
	NoVerify   bool   // Skip pre-commit and commit-msg hooks
	Author     string // Override commit author (format: "Name <email>")
	AllowEmpty bool   // Permit creating commits with no changes
}

// CleanOpts configures git clean behavior.
type CleanOpts struct {
	Force       bool // -f flag (required for git clean to do anything)
	Directories bool // -d flag to remove untracked directories
	IgnoredOnly bool // -X flag to only remove ignored files
	IgnoredToo  bool // -x flag to remove ignored and untracked files
}

// PushOpts configures git push behavior.
type PushOpts struct {
	Force          bool // --force push (use with caution)
	SetUpstream    bool // -u flag to set upstream tracking
	ForceWithLease bool // --force-with-lease (safer than Force)
}

// MergeOpts configures git merge behavior.
type MergeOpts struct {
	FFOnly   bool   // Only allows fast-forward merges
	NoFF     bool   // Creates merge commit even for fast-forward merges
	Message  string // Merge commit message
	NoCommit bool   // Performs merge but stops before creating commit
}

// LogOpts configures git log output.
type LogOpts struct {
	MaxCount int       // Limits the number of commits returned
	Since    time.Time // Filters commits after this time
	Until    time.Time // Filters commits before this time
	Path     string    // Filters commits affecting this path
}
