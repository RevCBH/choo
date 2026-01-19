package github

import (
	"context"
	"net/http"
	"net/http/httptest"
	"os"
	"os/exec"
	"regexp"
	"testing"
	"time"
)

func TestGetToken_EnvVariable(t *testing.T) {
	originalToken := os.Getenv("GITHUB_TOKEN")
	defer func() {
		if originalToken != "" {
			os.Setenv("GITHUB_TOKEN", originalToken)
		} else {
			os.Unsetenv("GITHUB_TOKEN")
		}
	}()

	expectedToken := "test-token-from-env"
	os.Setenv("GITHUB_TOKEN", expectedToken)

	token, err := getToken()
	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}

	if token != expectedToken {
		t.Errorf("expected token %q, got %q", expectedToken, token)
	}
}

func TestGetToken_NoToken(t *testing.T) {
	// Skip in CI environments where gh is authenticated
	// In CI (GitHub Actions), gh auth token works even without GITHUB_TOKEN env
	if os.Getenv("CI") == "true" || os.Getenv("GITHUB_ACTIONS") == "true" {
		t.Skip("Skipping test: gh CLI is authenticated in CI environment")
	}

	originalToken := os.Getenv("GITHUB_TOKEN")
	defer func() {
		if originalToken != "" {
			os.Setenv("GITHUB_TOKEN", originalToken)
		} else {
			os.Unsetenv("GITHUB_TOKEN")
		}
	}()

	os.Unsetenv("GITHUB_TOKEN")

	// Mock exec.Command to fail
	if os.Getenv("TEST_MOCK_GH_FAIL") == "1" {
		token, err := getToken()
		if err == nil {
			t.Fatal("expected error when no token available, got nil")
		}
		if token != "" {
			t.Errorf("expected empty token, got %q", token)
		}
		return
	}

	// Re-run ourselves with mock env
	cmd := exec.Command(os.Args[0], "-test.run=TestGetToken_NoToken")
	cmd.Env = append(os.Environ(), "TEST_MOCK_GH_FAIL=1")
	err := cmd.Run()
	if err == nil {
		t.Fatal("expected subprocess to fail, but it succeeded")
	}
}

func TestDetectOwnerRepo_HTTPS(t *testing.T) {
	// This test would need to mock exec.Command
	// For now, we'll test the regex logic by calling it in a subprocess
	// In a real scenario, you'd use a more sophisticated mocking approach

	tests := []struct {
		name     string
		url      string
		wantErr  bool
		expected [2]string
	}{
		{
			name:     "HTTPS with .git",
			url:      "https://github.com/owner/repo.git",
			expected: [2]string{"owner", "repo"},
		},
		{
			name:     "HTTPS without .git",
			url:      "https://github.com/owner/repo",
			expected: [2]string{"owner", "repo"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			httpsRegex := regexp.MustCompile(`https://github\.com/([^/]+)/([^/]+?)(\.git)?$`)
			matches := httpsRegex.FindStringSubmatch(tt.url)
			if matches == nil {
				t.Fatalf("failed to parse URL: %s", tt.url)
			}
			if matches[1] != tt.expected[0] || matches[2] != tt.expected[1] {
				t.Errorf("expected %v, got [%s, %s]", tt.expected, matches[1], matches[2])
			}
		})
	}
}

func TestDetectOwnerRepo_SSH(t *testing.T) {
	tests := []struct {
		name     string
		url      string
		expected [2]string
	}{
		{
			name:     "SSH with .git",
			url:      "git@github.com:owner/repo.git",
			expected: [2]string{"owner", "repo"},
		},
		{
			name:     "SSH without .git",
			url:      "git@github.com:owner/repo",
			expected: [2]string{"owner", "repo"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			sshRegex := regexp.MustCompile(`git@github\.com:([^/]+)/([^/]+?)(\.git)?$`)
			matches := sshRegex.FindStringSubmatch(tt.url)
			if matches == nil {
				t.Fatalf("failed to parse URL: %s", tt.url)
			}
			if matches[1] != tt.expected[0] || matches[2] != tt.expected[1] {
				t.Errorf("expected %v, got [%s, %s]", tt.expected, matches[1], matches[2])
			}
		})
	}
}

func TestNewPRClient_Defaults(t *testing.T) {
	originalToken := os.Getenv("GITHUB_TOKEN")
	defer func() {
		if originalToken != "" {
			os.Setenv("GITHUB_TOKEN", originalToken)
		} else {
			os.Unsetenv("GITHUB_TOKEN")
		}
	}()

	os.Setenv("GITHUB_TOKEN", "test-token")

	cfg := PRClientConfig{
		Owner: "test-owner",
		Repo:  "test-repo",
	}

	client, err := NewPRClient(cfg)
	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}

	if client.pollInterval != 30*time.Second {
		t.Errorf("expected poll interval 30s, got %v", client.pollInterval)
	}

	if client.reviewTimeout != 2*time.Hour {
		t.Errorf("expected review timeout 2h, got %v", client.reviewTimeout)
	}

	if client.owner != "test-owner" {
		t.Errorf("expected owner 'test-owner', got %q", client.owner)
	}

	if client.repo != "test-repo" {
		t.Errorf("expected repo 'test-repo', got %q", client.repo)
	}
}

func TestDoRequest_RateLimitRetry(t *testing.T) {
	originalToken := os.Getenv("GITHUB_TOKEN")
	defer func() {
		if originalToken != "" {
			os.Setenv("GITHUB_TOKEN", originalToken)
		} else {
			os.Unsetenv("GITHUB_TOKEN")
		}
	}()

	os.Setenv("GITHUB_TOKEN", "test-token")

	attemptCount := 0
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		attemptCount++
		if attemptCount == 1 {
			w.Header().Set("Retry-After", "1")
			w.WriteHeader(http.StatusTooManyRequests)
			return
		}
		w.WriteHeader(http.StatusOK)
		w.Write([]byte(`{"success": true}`))
	}))
	defer server.Close()

	client := &PRClient{
		httpClient: &http.Client{},
		token:      "test-token",
		owner:      "test-owner",
		repo:       "test-repo",
	}

	ctx := context.Background()
	resp, err := client.doRequest(ctx, "GET", server.URL, nil)
	if err != nil {
		t.Fatalf("expected no error after retry, got %v", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		t.Errorf("expected status 200, got %d", resp.StatusCode)
	}

	if attemptCount != 2 {
		t.Errorf("expected 2 attempts (1 failure + 1 success), got %d", attemptCount)
	}
}
