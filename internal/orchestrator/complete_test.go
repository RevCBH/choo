package orchestrator

import (
	"context"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
)

func TestComplete_RunsArchive(t *testing.T) {
	repoDir, specsDir, tasksDir := setupSpecsDirs(t)

	completeSpec := "---\nstatus: complete\n---\n# Complete"
	if err := os.WriteFile(filepath.Join(specsDir, "COMPLETE.md"), []byte(completeSpec), 0644); err != nil {
		t.Fatalf("write spec: %v", err)
	}

	setupFakeBins(t)
	t.Setenv("FAKE_GIT_STATUS", "clean")

	o := &Orchestrator{
		cfg: Config{
			TasksDir:      tasksDir,
			RepoRoot:      repoDir,
			FeatureBranch: "feature/test",
			TargetBranch:  "main",
			NoPR:          true,
		},
	}

	if err := o.Complete(context.Background()); err != nil {
		t.Fatalf("Complete() error = %v", err)
	}

	completedPath := filepath.Join(specsDir, "completed", "COMPLETE.md")
	if _, err := os.Stat(completedPath); err != nil {
		t.Fatalf("expected archived spec at %s: %v", completedPath, err)
	}
}

func TestComplete_CommitsChanges(t *testing.T) {
	repoDir, _, tasksDir := setupSpecsDirs(t)

	gitLog, _ := setupFakeBins(t)
	t.Setenv("FAKE_GIT_STATUS", "dirty")

	o := &Orchestrator{
		cfg: Config{
			TasksDir:      tasksDir,
			RepoRoot:      repoDir,
			FeatureBranch: "feature/test",
			TargetBranch:  "main",
			NoPR:          true,
		},
	}

	if err := o.Complete(context.Background()); err != nil {
		t.Fatalf("Complete() error = %v", err)
	}

	if !logHas(gitLog, "commit") {
		t.Fatalf("expected git commit to run")
	}
}

func TestComplete_PushesBranch(t *testing.T) {
	repoDir, _, tasksDir := setupSpecsDirs(t)

	gitLog, _ := setupFakeBins(t)
	t.Setenv("FAKE_GIT_STATUS", "clean")

	o := &Orchestrator{
		cfg: Config{
			TasksDir:      tasksDir,
			RepoRoot:      repoDir,
			FeatureBranch: "feature/test",
			TargetBranch:  "main",
			NoPR:          true,
		},
	}

	if err := o.Complete(context.Background()); err != nil {
		t.Fatalf("Complete() error = %v", err)
	}

	if !logHas(gitLog, "push") {
		t.Fatalf("expected git push to run")
	}
}

func TestComplete_CreatesPR(t *testing.T) {
	repoDir, _, tasksDir := setupSpecsDirs(t)

	_, ghLog := setupFakeBins(t)
	t.Setenv("FAKE_GIT_STATUS", "clean")

	o := &Orchestrator{
		cfg: Config{
			TasksDir:           tasksDir,
			RepoRoot:           repoDir,
			FeatureBranch:      "feature/test",
			TargetBranch:       "main",
			FeatureTitle:       "Test Feature",
			FeatureDescription: "Test feature description",
			FeatureMode:        true,
		},
		completedUnits: []string{"unit-a", "unit-b"},
	}

	if err := o.Complete(context.Background()); err != nil {
		t.Fatalf("Complete() error = %v", err)
	}

	if !logHas(ghLog, "pr create") {
		t.Fatalf("expected gh pr create to run")
	}
}

func TestComplete_NoPRSkipsCreation(t *testing.T) {
	repoDir, _, tasksDir := setupSpecsDirs(t)

	_, ghLog := setupFakeBins(t)
	t.Setenv("FAKE_GIT_STATUS", "clean")

	o := &Orchestrator{
		cfg: Config{
			TasksDir:      tasksDir,
			RepoRoot:      repoDir,
			FeatureBranch: "feature/test",
			TargetBranch:  "main",
			NoPR:          true,
			FeatureMode:   true,
		},
	}

	if err := o.Complete(context.Background()); err != nil {
		t.Fatalf("Complete() error = %v", err)
	}

	if _, err := os.Stat(ghLog); err == nil {
		if logHas(ghLog, "pr create") {
			t.Fatalf("expected PR creation to be skipped")
		}
	}
}

func TestComplete_NoChangesSkipsCommit(t *testing.T) {
	repoDir, _, tasksDir := setupSpecsDirs(t)

	gitLog, _ := setupFakeBins(t)
	t.Setenv("FAKE_GIT_STATUS", "clean")

	o := &Orchestrator{
		cfg: Config{
			TasksDir:      tasksDir,
			RepoRoot:      repoDir,
			FeatureBranch: "feature/test",
			TargetBranch:  "main",
			NoPR:          true,
		},
	}

	if err := o.Complete(context.Background()); err != nil {
		t.Fatalf("Complete() error = %v", err)
	}

	if logHas(gitLog, "commit") {
		t.Fatalf("expected no git commit when there are no changes")
	}
}

func TestBuildPRBody_IncludesUnits(t *testing.T) {
	o := &Orchestrator{
		cfg: Config{
			FeatureDescription: "Test feature description",
		},
		completedUnits: []string{"unit-a", "unit-b", "unit-c"},
	}

	body := o.buildPRBody()

	if !strings.Contains(body, "unit-a") {
		t.Error("PR body should contain unit-a")
	}
	if !strings.Contains(body, "unit-b") {
		t.Error("PR body should contain unit-b")
	}
	if !strings.Contains(body, "Test feature description") {
		t.Error("PR body should contain feature description")
	}
}

func TestIsNoChangesError_ExitCode1(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("requires shell")
	}

	cmd := exec.Command("sh", "-c", "exit 1")
	err := cmd.Run()

	if !isNoChangesError(err) {
		t.Error("isNoChangesError should return true for exit code 1")
	}
}

func TestIsNoChangesError_ExitCode0(t *testing.T) {
	if isNoChangesError(nil) {
		t.Error("isNoChangesError should return false for nil error")
	}
}

func TestIsNoChangesError_ExitCode2(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("requires shell")
	}

	cmd := exec.Command("sh", "-c", "exit 2")
	err := cmd.Run()

	if isNoChangesError(err) {
		t.Error("isNoChangesError should return false for exit code 2")
	}
}

func TestCommitArchive_HandlesNoChanges(t *testing.T) {
	repoDir, _, _ := setupSpecsDirs(t)

	gitLog, _ := setupFakeBins(t)
	t.Setenv("FAKE_GIT_STATUS", "dirty")
	t.Setenv("FAKE_GIT_COMMIT_EXIT", "1")

	o := &Orchestrator{
		cfg: Config{
			RepoRoot: repoDir,
		},
	}

	if err := o.commitArchive(context.Background()); err != nil {
		t.Fatalf("commitArchive should succeed with no changes, got: %v", err)
	}

	if !logHas(gitLog, "commit") {
		t.Fatalf("expected git commit attempt")
	}
}

func setupSpecsDirs(t *testing.T) (repoDir, specsDir, tasksDir string) {
	t.Helper()

	repoDir = t.TempDir()
	specsDir = filepath.Join(repoDir, "specs")
	tasksDir = filepath.Join(specsDir, "tasks")

	if err := os.MkdirAll(tasksDir, 0755); err != nil {
		t.Fatalf("mkdir tasks: %v", err)
	}

	return repoDir, specsDir, tasksDir
}

func setupFakeBins(t *testing.T) (gitLog, ghLog string) {
	t.Helper()
	if runtime.GOOS == "windows" {
		t.Skip("fake binaries require a POSIX shell")
	}

	binDir := filepath.Join(t.TempDir(), "bin")
	if err := os.MkdirAll(binDir, 0755); err != nil {
		t.Fatalf("mkdir bin: %v", err)
	}

	gitLog = filepath.Join(binDir, "git.log")
	ghLog = filepath.Join(binDir, "gh.log")

	gitScript := "#!/bin/sh\n" +
		"echo \"$@\" >> \"$FAKE_GIT_LOG\"\n" +
		"if [ \"$1\" = \"status\" ]; then\n" +
		"  if [ \"$FAKE_GIT_STATUS\" = \"dirty\" ]; then\n" +
		"    echo \" M specs/COMPLETE.md\"\n" +
		"  fi\n" +
		"  exit 0\n" +
		"fi\n" +
		"if [ \"$1\" = \"commit\" ]; then\n" +
		"  if [ \"$FAKE_GIT_COMMIT_EXIT\" = \"1\" ]; then\n" +
		"    exit 1\n" +
		"  fi\n" +
		"  exit 0\n" +
		"fi\n" +
		"exit 0\n"

	ghScript := "#!/bin/sh\n" +
		"echo \"$@\" >> \"$FAKE_GH_LOG\"\n" +
		"if [ -n \"$FAKE_GH_OUTPUT\" ]; then\n" +
		"  echo \"$FAKE_GH_OUTPUT\"\n" +
		"else\n" +
		"  echo \"https://github.com/example/repo/pull/1\"\n" +
		"fi\n" +
		"exit 0\n"

	gitPath := filepath.Join(binDir, "git")
	if err := os.WriteFile(gitPath, []byte(gitScript), 0755); err != nil {
		t.Fatalf("write git script: %v", err)
	}

	ghPath := filepath.Join(binDir, "gh")
	if err := os.WriteFile(ghPath, []byte(ghScript), 0755); err != nil {
		t.Fatalf("write gh script: %v", err)
	}

	if err := os.Chmod(gitPath, 0755); err != nil {
		t.Fatalf("chmod git: %v", err)
	}
	if err := os.Chmod(ghPath, 0755); err != nil {
		t.Fatalf("chmod gh: %v", err)
	}

	t.Setenv("PATH", binDir+string(os.PathListSeparator)+os.Getenv("PATH"))
	t.Setenv("FAKE_GIT_LOG", gitLog)
	t.Setenv("FAKE_GH_LOG", ghLog)

	return gitLog, ghLog
}

func logHas(path, needle string) bool {
	data, err := os.ReadFile(path)
	if err != nil {
		return false
	}
	return strings.Contains(string(data), needle)
}
