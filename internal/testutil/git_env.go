package testutil

import "os"

var gitEnvVars = []string{
	"GIT_DIR",
	"GIT_WORK_TREE",
	"GIT_INDEX_FILE",
	"GIT_COMMON_DIR",
	"GIT_PREFIX",
	"GIT_OBJECT_DIRECTORY",
	"GIT_ALTERNATE_OBJECT_DIRECTORIES",
	"GIT_CEILING_DIRECTORIES",
}

// UnsetGitEnv clears git environment variables that can redirect repo operations.
func UnsetGitEnv() {
	for _, key := range gitEnvVars {
		_ = os.Unsetenv(key)
	}
}
