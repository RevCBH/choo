package config

// Config holds configuration for the orchestrator
type Config struct {
	TasksDir     string `yaml:"tasks_dir"`
	Parallelism  int    `yaml:"parallelism"`
	TargetBranch string `yaml:"target_branch"`
	WorktreeDir  string `yaml:"worktree_dir"`
}

// Load loads configuration from file or returns defaults
func Load(path string) (*Config, error) {
	// TODO: Implement config file loading
	return &Config{
		TasksDir:     "specs/tasks",
		Parallelism:  4,
		TargetBranch: "main",
		WorktreeDir:  ".worktrees",
	}, nil
}
