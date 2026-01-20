package config

import (
	"strings"
	"testing"
)

func TestParseGitHubURL_HTTPS_WithGit(t *testing.T) {
	owner, repo, err := parseGitHubURL("https://github.com/RevCBH/choo.git")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if owner != "RevCBH" {
		t.Errorf("expected owner to be 'RevCBH', got %q", owner)
	}
	if repo != "choo" {
		t.Errorf("expected repo to be 'choo', got %q", repo)
	}
}

func TestParseGitHubURL_HTTPS_NoGit(t *testing.T) {
	owner, repo, err := parseGitHubURL("https://github.com/RevCBH/choo")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if owner != "RevCBH" {
		t.Errorf("expected owner to be 'RevCBH', got %q", owner)
	}
	if repo != "choo" {
		t.Errorf("expected repo to be 'choo', got %q", repo)
	}
}

func TestParseGitHubURL_SSH_WithGit(t *testing.T) {
	owner, repo, err := parseGitHubURL("git@github.com:RevCBH/choo.git")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if owner != "RevCBH" {
		t.Errorf("expected owner to be 'RevCBH', got %q", owner)
	}
	if repo != "choo" {
		t.Errorf("expected repo to be 'choo', got %q", repo)
	}
}

func TestParseGitHubURL_SSH_NoGit(t *testing.T) {
	owner, repo, err := parseGitHubURL("git@github.com:RevCBH/choo")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if owner != "RevCBH" {
		t.Errorf("expected owner to be 'RevCBH', got %q", owner)
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

func TestDetectGitHubRepo_Stubbed(t *testing.T) {
	// Test HTTPS URL
	stubGitRemote(t, "https://github.com/RevCBH/choo.git", nil)
	owner, repo, err := detectGitHubRepo("ignored")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if owner != "RevCBH" {
		t.Errorf("expected owner to be 'RevCBH', got %q", owner)
	}
	if repo != "choo" {
		t.Errorf("expected repo to be 'choo', got %q", repo)
	}

	// Test SSH URL
	stubGitRemote(t, "git@github.com:RevCBH/choo.git", nil)
	owner, repo, err = detectGitHubRepo("ignored")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if owner != "RevCBH" {
		t.Errorf("expected owner to be 'RevCBH', got %q", owner)
	}
	if repo != "choo" {
		t.Errorf("expected repo to be 'choo', got %q", repo)
	}
}
