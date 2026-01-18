package config

import (
	"fmt"
	"os/exec"
	"regexp"
	"strings"
)

// detectGitHubRepo extracts owner and repo from the git remote origin URL.
// Supports both HTTPS and SSH URL formats.
func detectGitHubRepo(repoRoot string) (owner, repo string, err error) {
	cmd := exec.Command("git", "remote", "get-url", "origin")
	cmd.Dir = repoRoot
	out, err := cmd.Output()
	if err != nil {
		return "", "", fmt.Errorf("get git remote: %w", err)
	}

	url := strings.TrimSpace(string(out))
	return parseGitHubURL(url)
}

// parseGitHubURL extracts owner/repo from a GitHub URL.
// Supports:
//   - https://github.com/owner/repo.git
//   - git@github.com:owner/repo.git
//   - https://github.com/owner/repo
//   - git@github.com:owner/repo
func parseGitHubURL(url string) (owner, repo string, err error) {
	// HTTPS format
	httpsRe := regexp.MustCompile(`https://github\.com/([^/]+)/([^/]+?)(?:\.git)?$`)
	if m := httpsRe.FindStringSubmatch(url); m != nil {
		return m[1], m[2], nil
	}

	// SSH format
	sshRe := regexp.MustCompile(`git@github\.com:([^/]+)/([^/]+?)(?:\.git)?$`)
	if m := sshRe.FindStringSubmatch(url); m != nil {
		return m[1], m[2], nil
	}

	return "", "", fmt.Errorf("unrecognized GitHub URL format: %s", url)
}
