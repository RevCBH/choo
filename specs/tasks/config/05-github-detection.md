---
task: 5
status: complete
backpressure: "go test ./internal/config/... -run TestGitHub"
depends_on: [1]
---

# GitHub Auto-Detection

**Parent spec**: `/specs/CONFIG.md`
**Task**: #5 of 6 in implementation plan

## Objective

Implement GitHub owner/repo auto-detection from git remote origin URL.

## Dependencies

### External Specs (must be implemented)
- None

### Task Dependencies (within this unit)
- Task #1 must be complete (provides: GitHubConfig type)

### Package Dependencies
- `os/exec` (standard library)
- `regexp` (standard library)
- `strings` (standard library)
- `fmt` (standard library)

## Deliverables

### Files to Create/Modify

```
internal/
└── config/
    └── github.go    # CREATE: GitHub detection logic
```

### Functions to Implement

```go
// detectGitHubRepo extracts owner and repo from the git remote origin URL.
// Supports both HTTPS and SSH URL formats.
func detectGitHubRepo(repoRoot string) (owner, repo string, err error)

// parseGitHubURL extracts owner/repo from a GitHub URL.
// Supports:
//   - https://github.com/owner/repo.git
//   - git@github.com:owner/repo.git
//   - https://github.com/owner/repo
//   - git@github.com:owner/repo
func parseGitHubURL(url string) (owner, repo string, err error)
```

### Implementation

```go
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
```

## Backpressure

### Validation Command

```bash
go test ./internal/config/... -run TestGitHub
```

### Must Pass

| Test | Assertion |
|------|-----------|
| `TestParseGitHubURL_HTTPS_WithGit` | `parseGitHubURL("https://github.com/anthropics/choo.git")` returns `"anthropics", "choo"` |
| `TestParseGitHubURL_HTTPS_NoGit` | `parseGitHubURL("https://github.com/anthropics/choo")` returns `"anthropics", "choo"` |
| `TestParseGitHubURL_SSH_WithGit` | `parseGitHubURL("git@github.com:anthropics/choo.git")` returns `"anthropics", "choo"` |
| `TestParseGitHubURL_SSH_NoGit` | `parseGitHubURL("git@github.com:anthropics/choo")` returns `"anthropics", "choo"` |
| `TestParseGitHubURL_GitLab` | `parseGitHubURL("https://gitlab.com/owner/repo.git")` returns error |
| `TestParseGitHubURL_Invalid` | `parseGitHubURL("not-a-url")` returns error |
| `TestDetectGitHubRepo_Integration` | Creates temp git repo with remote, verifies detection works |

### Test File to Create

```
internal/
└── config/
    └── github_test.go    # CREATE: Tests for GitHub detection
```

### Test Helper for Integration Test

```go
// initGitRepo creates a git repo with the given remote URL for testing
func initGitRepo(t *testing.T, dir, remoteURL string) {
    t.Helper()

    // git init
    cmd := exec.Command("git", "init")
    cmd.Dir = dir
    require.NoError(t, cmd.Run())

    // git remote add origin <url>
    cmd = exec.Command("git", "remote", "add", "origin", remoteURL)
    cmd.Dir = dir
    require.NoError(t, cmd.Run())
}
```

### Test Fixtures

None required (uses temp directories with git init).

### CI Compatibility

- [x] No external API keys required
- [x] No network access required (git commands are local)
- [x] Runs in <60 seconds

## Implementation Notes

- `detectGitHubRepo` runs `git remote get-url origin` in the repo directory
- `parseGitHubURL` is a pure function, separate for easy testing
- Regex handles optional `.git` suffix
- Non-GitHub URLs (GitLab, Bitbucket) return errors
- The `repoRoot` path must be the actual git repository root

## NOT In Scope

- LoadConfig function (Task #6)
- Default values (Task #2)
- Environment variable handling (Task #3)
- Validation logic (Task #4)
- GitLab/Bitbucket support (explicitly out of scope per design)
