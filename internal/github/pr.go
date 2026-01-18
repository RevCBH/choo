package github

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"time"
)

// PRInfo holds information about a created PR
type PRInfo struct {
	Number       int
	URL          string
	Branch       string
	TargetBranch string
	Title        string
	CreatedAt    time.Time
}

// MergeResult holds the result of a merge operation
type MergeResult struct {
	Merged  bool
	SHA     string
	Message string
}

// GitHub API response structures

type ghPullRequest struct {
	Number    int       `json:"number"`
	HTMLURL   string    `json:"html_url"`
	Head      ghRef     `json:"head"`
	Base      ghRef     `json:"base"`
	Title     string    `json:"title"`
	CreatedAt time.Time `json:"created_at"`
}

type ghRef struct {
	Ref string `json:"ref"`
}

type ghMergeResponse struct {
	SHA     string `json:"sha"`
	Merged  bool   `json:"merged"`
	Message string `json:"message"`
}

// CreatePR is a placeholder - PR creation is delegated to Claude via gh CLI.
// This returns an error indicating the operation should be performed externally.
// The orchestrator will instruct Claude to run: gh pr create --title "..." --body "..."
func (c *PRClient) CreatePR(ctx context.Context, title, body, branch string) (*PRInfo, error) {
	return nil, fmt.Errorf("PR creation is delegated to Claude via 'gh pr create'")
}

// GetPR fetches current PR information by number
func (c *PRClient) GetPR(ctx context.Context, prNumber int) (*PRInfo, error) {
	url := fmt.Sprintf("https://api.github.com/repos/%s/%s/pulls/%d", c.owner, c.repo, prNumber)
	resp, err := c.doRequest(ctx, "GET", url, nil)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	bodyBytes, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("failed to read response body: %w", err)
	}

	var ghPR ghPullRequest
	if err := json.Unmarshal(bodyBytes, &ghPR); err != nil {
		return nil, fmt.Errorf("failed to parse PR response: %w", err)
	}

	return &PRInfo{
		Number:       ghPR.Number,
		URL:          ghPR.HTMLURL,
		Branch:       ghPR.Head.Ref,
		TargetBranch: ghPR.Base.Ref,
		Title:        ghPR.Title,
		CreatedAt:    ghPR.CreatedAt,
	}, nil
}

// UpdatePR updates an existing PR's title or body
func (c *PRClient) UpdatePR(ctx context.Context, prNumber int, title, body string) error {
	url := fmt.Sprintf("https://api.github.com/repos/%s/%s/pulls/%d", c.owner, c.repo, prNumber)
	reqBody := map[string]string{
		"title": title,
		"body":  body,
	}

	resp, err := c.doRequest(ctx, "PATCH", url, reqBody)
	if err != nil {
		return err
	}
	resp.Body.Close()

	return nil
}

// Merge executes a squash merge on the PR
func (c *PRClient) Merge(ctx context.Context, prNumber int) (*MergeResult, error) {
	url := fmt.Sprintf("https://api.github.com/repos/%s/%s/pulls/%d/merge", c.owner, c.repo, prNumber)
	reqBody := map[string]string{
		"merge_method": "squash",
	}

	resp, err := c.doRequest(ctx, "PUT", url, reqBody)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	bodyBytes, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("failed to read response body: %w", err)
	}

	var ghMerge ghMergeResponse
	if err := json.Unmarshal(bodyBytes, &ghMerge); err != nil {
		return nil, fmt.Errorf("failed to parse merge response: %w", err)
	}

	return &MergeResult{
		Merged:  ghMerge.Merged,
		SHA:     ghMerge.SHA,
		Message: ghMerge.Message,
	}, nil
}

// ClosePR closes a PR without merging
func (c *PRClient) ClosePR(ctx context.Context, prNumber int) error {
	url := fmt.Sprintf("https://api.github.com/repos/%s/%s/pulls/%d", c.owner, c.repo, prNumber)
	reqBody := map[string]string{
		"state": "closed",
	}

	resp, err := c.doRequest(ctx, "PATCH", url, reqBody)
	if err != nil {
		return err
	}
	resp.Body.Close()

	return nil
}
