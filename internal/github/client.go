package github

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"os/exec"
	"regexp"
	"strconv"
	"strings"
	"time"
)

// PRClient manages GitHub PR operations
type PRClient struct {
	httpClient    *http.Client
	owner         string
	repo          string
	pollInterval  time.Duration
	reviewTimeout time.Duration
	token         string
	// events *events.Bus - added later when events package exists
}

// PRClientConfig holds configuration for the PR client
type PRClientConfig struct {
	Owner         string
	Repo          string
	PollInterval  time.Duration
	ReviewTimeout time.Duration
}

// NewPRClient creates a new GitHub PR client
func NewPRClient(cfg PRClientConfig) (*PRClient, error) {
	token, err := getToken()
	if err != nil {
		return nil, err
	}

	owner := cfg.Owner
	repo := cfg.Repo
	if owner == "" || repo == "" {
		detectedOwner, detectedRepo, err := detectOwnerRepo()
		if err != nil {
			return nil, err
		}
		if owner == "" {
			owner = detectedOwner
		}
		if repo == "" {
			repo = detectedRepo
		}
	}

	pollInterval := cfg.PollInterval
	if pollInterval == 0 {
		pollInterval = 30 * time.Second
	}

	reviewTimeout := cfg.ReviewTimeout
	if reviewTimeout == 0 {
		reviewTimeout = 2 * time.Hour
	}

	return &PRClient{
		httpClient:    &http.Client{},
		owner:         owner,
		repo:          repo,
		pollInterval:  pollInterval,
		reviewTimeout: reviewTimeout,
		token:         token,
	}, nil
}

// getToken retrieves the GitHub token from env or gh CLI
func getToken() (string, error) {
	if token := os.Getenv("GITHUB_TOKEN"); token != "" {
		return token, nil
	}

	cmd := exec.Command("gh", "auth", "token")
	out, err := cmd.Output()
	if err == nil {
		return strings.TrimSpace(string(out)), nil
	}

	return "", fmt.Errorf("no GitHub token found: set GITHUB_TOKEN or run 'gh auth login'")
}

// detectOwnerRepo parses owner/repo from git remote
func detectOwnerRepo() (owner, repo string, err error) {
	cmd := exec.Command("git", "remote", "get-url", "origin")
	out, err := cmd.Output()
	if err != nil {
		return "", "", fmt.Errorf("failed to get git remote: %w", err)
	}

	remoteURL := strings.TrimSpace(string(out))

	// Handle HTTPS format: https://github.com/owner/repo.git
	httpsRegex := regexp.MustCompile(`https://github\.com/([^/]+)/([^/]+?)(\.git)?$`)
	if matches := httpsRegex.FindStringSubmatch(remoteURL); matches != nil {
		return matches[1], matches[2], nil
	}

	// Handle SSH format: git@github.com:owner/repo.git
	sshRegex := regexp.MustCompile(`git@github\.com:([^/]+)/([^/]+?)(\.git)?$`)
	if matches := sshRegex.FindStringSubmatch(remoteURL); matches != nil {
		return matches[1], matches[2], nil
	}

	return "", "", fmt.Errorf("failed to parse owner/repo from remote URL: %s", remoteURL)
}

// doRequest executes an HTTP request with rate limit handling
func (c *PRClient) doRequest(ctx context.Context, method, url string, body any) (*http.Response, error) {
	var reqBody io.Reader
	if body != nil {
		jsonData, err := json.Marshal(body)
		if err != nil {
			return nil, fmt.Errorf("failed to marshal request body: %w", err)
		}
		reqBody = bytes.NewReader(jsonData)
	}

	maxRetries := 5
	initialBackoff := 1 * time.Second
	backoff := initialBackoff

	for attempt := 0; attempt <= maxRetries; attempt++ {
		req, err := http.NewRequestWithContext(ctx, method, url, reqBody)
		if err != nil {
			return nil, fmt.Errorf("failed to create request: %w", err)
		}

		req.Header.Set("Authorization", "Bearer "+c.token)
		req.Header.Set("Accept", "application/vnd.github+json")
		if body != nil {
			req.Header.Set("Content-Type", "application/json")
		}

		resp, err := c.httpClient.Do(req)
		if err != nil {
			return nil, fmt.Errorf("failed to execute request: %w", err)
		}

		// Success
		if resp.StatusCode >= 200 && resp.StatusCode < 300 {
			return resp, nil
		}

		// Handle rate limits (403 or 429)
		if resp.StatusCode == 403 || resp.StatusCode == 429 {
			resp.Body.Close()

			if attempt == maxRetries {
				return nil, fmt.Errorf("rate limit exceeded after %d retries", maxRetries)
			}

			// Check Retry-After header
			waitDuration := backoff
			if retryAfter := resp.Header.Get("Retry-After"); retryAfter != "" {
				if seconds, err := strconv.Atoi(retryAfter); err == nil {
					waitDuration = time.Duration(seconds) * time.Second
				}
			}

			select {
			case <-time.After(waitDuration):
				backoff *= 2
				continue
			case <-ctx.Done():
				return nil, ctx.Err()
			}
		}

		// Retry 5xx errors
		if resp.StatusCode >= 500 {
			resp.Body.Close()

			if attempt == maxRetries {
				return nil, fmt.Errorf("server error after %d retries: status %d", maxRetries, resp.StatusCode)
			}

			select {
			case <-time.After(backoff):
				backoff *= 2
				continue
			case <-ctx.Done():
				return nil, ctx.Err()
			}
		}

		// Fail on other 4xx errors
		defer resp.Body.Close()
		bodyBytes, _ := io.ReadAll(resp.Body)
		return nil, fmt.Errorf("request failed with status %d: %s", resp.StatusCode, string(bodyBytes))
	}

	return nil, fmt.Errorf("request failed after %d retries", maxRetries)
}
