package github

import "time"

// PRComment represents a review comment on a PR
type PRComment struct {
	ID        int64
	Path      string
	Line      int
	Body      string
	Author    string
	CreatedAt time.Time
}
