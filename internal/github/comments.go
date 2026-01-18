package github

import (
	"context"
	"encoding/json"
	"fmt"
	"time"
)

// PRComment represents a review comment on a PR
type PRComment struct {
	ID        int64
	Path      string
	Line      int
	Body      string
	Author    string
	CreatedAt time.Time
}

// ghReviewComment is the GitHub API response for a PR review comment
type ghReviewComment struct {
	ID        int64     `json:"id"`
	Path      string    `json:"path"`
	Line      *int      `json:"line"`
	Body      string    `json:"body"`
	User      ghUser    `json:"user"`
	CreatedAt time.Time `json:"created_at"`
}

type ghUser struct {
	Login string `json:"login"`
}

// GetPRComments fetches all review comments on a PR
func (c *PRClient) GetPRComments(ctx context.Context, prNumber int) ([]PRComment, error) {
	url := fmt.Sprintf("https://api.github.com/repos/%s/%s/pulls/%d/comments", c.owner, c.repo, prNumber)
	resp, err := c.doRequest(ctx, "GET", url, nil)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	var ghComments []ghReviewComment
	if err := json.NewDecoder(resp.Body).Decode(&ghComments); err != nil {
		return nil, fmt.Errorf("failed to decode comments: %w", err)
	}

	comments := make([]PRComment, 0, len(ghComments))
	for _, ghComment := range ghComments {
		line := 0
		if ghComment.Line != nil {
			line = *ghComment.Line
		}

		comments = append(comments, PRComment{
			ID:        ghComment.ID,
			Path:      ghComment.Path,
			Line:      line,
			Body:      ghComment.Body,
			Author:    ghComment.User.Login,
			CreatedAt: ghComment.CreatedAt,
		})
	}

	return comments, nil
}

// GetUnaddressedComments returns comments since the given timestamp
// Used to find comments that need to be addressed after a push
func (c *PRClient) GetUnaddressedComments(ctx context.Context, prNumber int, since time.Time) ([]PRComment, error) {
	all, err := c.GetPRComments(ctx, prNumber)
	if err != nil {
		return nil, err
	}

	var unaddressed []PRComment
	for _, comment := range all {
		if comment.CreatedAt.After(since) {
			unaddressed = append(unaddressed, comment)
		}
	}
	return unaddressed, nil
}
