package config

import (
	"os/exec"
	"strings"
	"testing"
)

func TestParseGitHubURL_HTTPS_WithGit(t *testing.T) {
	owner, repo, err := parseGitHubURL("https://github.com/anthropics/choo.git")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if owner != "anthropics" {
		t.Errorf("expected owner to be 'anthropics', got %q", owner)
	}
	if repo != "choo" {
		t.Errorf("expected repo to be 'choo', got %q", repo)
	}
}

func TestParseGitHubURL_HTTPS_NoGit(t *testing.T) {
	owner, repo, err := parseGitHubURL("https://github.com/anthropics/choo")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if owner != "anthropics" {
		t.Errorf("expected owner to be 'anthropics', got %q", owner)
	}
	if repo != "choo" {
		t.Errorf("expected repo to be 'choo', got %q", repo)
	}
}

func TestParseGitHubURL_SSH_WithGit(t *testing.T) {
	owner, repo, err := parseGitHubURL("git@github.com:anthropics/choo.git")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if owner != "anthropics" {
		t.Errorf("expected owner to be 'anthropics', got %q", owner)
	}
	if repo != "choo" {
		t.Errorf("expected repo to be 'choo', got %q", repo)
	}
}

func TestParseGitHubURL_SSH_NoGit(t *testing.T) {
	owner, repo, err := parseGitHubURL("git@github.com:anthropics/choo")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if owner != "anthropics" {
		t.Errorf("expected owner to be 'anthropics', got %q", owner)
	}
	if repo != "choo" {
		t.Errorf("expected repo to be 'choo', got %q", repo)
	}
}

func TestParseGitHubURL_GitLab(t *testing.T) {
	_, _, err := parseGitHubURL("https://gitlab.com/owner/repo.git")
	if err == nil {
		t.Fatal("expected error for GitLab URL, got nil")
	}
	if !strings.Contains(err.Error(), "unrecognized GitHub URL format") {
		t.Errorf("expected error to contain 'unrecognized GitHub URL format', got %q", err.Error())
	}
}

func TestParseGitHubURL_Invalid(t *testing.T) {
	_, _, err := parseGitHubURL("not-a-url")
	if err == nil {
		t.Fatal("expected error for invalid URL, got nil")
	}
	if !strings.Contains(err.Error(), "unrecognized GitHub URL format") {
		t.Errorf("expected error to contain 'unrecognized GitHub URL format', got %q", err.Error())
	}
}

func TestDetectGitHubRepo_Integration(t *testing.T) {
	// Create temp directory for test repo
	dir := t.TempDir()

	// Test HTTPS URL
	initGitRepo(t, dir, "https://github.com/anthropics/choo.git")
	owner, repo, err := detectGitHubRepo(dir)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if owner != "anthropics" {
		t.Errorf("expected owner to be 'anthropics', got %q", owner)
	}
	if repo != "choo" {
		t.Errorf("expected repo to be 'choo', got %q", repo)
	}

	// Clean up and test SSH URL
	cleanDir := t.TempDir()
	initGitRepo(t, cleanDir, "git@github.com:anthropics/choo.git")
	owner, repo, err = detectGitHubRepo(cleanDir)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if owner != "anthropics" {
		t.Errorf("expected owner to be 'anthropics', got %q", owner)
	}
	if repo != "choo" {
		t.Errorf("expected repo to be 'choo', got %q", repo)
	}
}

// initGitRepo creates a git repo with the given remote URL for testing
func initGitRepo(t *testing.T, dir, remoteURL string) {
	t.Helper()

	// git init
	cmd := exec.Command("git", "init")
	cmd.Dir = dir
	if err := cmd.Run(); err != nil {
		t.Fatalf("failed to init git repo: %v", err)
	}

	// git remote add origin <url>
	cmd = exec.Command("git", "remote", "add", "origin", remoteURL)
	cmd.Dir = dir
	if err := cmd.Run(); err != nil {
		t.Fatalf("failed to add git remote: %v", err)
	}
}
