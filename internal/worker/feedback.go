package worker

import (
	"context"
	"fmt"
	"os/exec"
	"time"

	"github.com/RevCBH/choo/internal/events"
	"github.com/RevCBH/choo/internal/github"
)

// FeedbackConfig holds configuration for feedback handling
type FeedbackConfig struct {
	MaxRetries        int           // Max Claude invocation attempts (default 3)
	InvocationTimeout time.Duration // Timeout per Claude invocation (default 10m)
}

// DefaultFeedbackConfig returns the default feedback configuration
func DefaultFeedbackConfig() FeedbackConfig {
	return FeedbackConfig{
		MaxRetries:        3,
		InvocationTimeout: 10 * time.Minute,
	}
}

// ClaudeInvoker abstracts Claude invocation for testing
type ClaudeInvoker interface {
	Invoke(ctx context.Context, prompt string, workdir string) error
}

// Escalator handles escalation when automated handling fails
type Escalator interface {
	Escalate(ctx context.Context, e Escalation) error
}

// Escalation represents an escalation event
type Escalation struct {
	Severity EscalationSeverity
	Unit     string
	Title    string
	Message  string
	Context  map[string]string
}

// EscalationSeverity indicates urgency level
type EscalationSeverity string

const (
	SeverityInfo     EscalationSeverity = "info"
	SeverityWarning  EscalationSeverity = "warning"
	SeverityBlocking EscalationSeverity = "blocking"
)

// FeedbackDeps holds dependencies for FeedbackHandler
type FeedbackDeps struct {
	GitHub    *github.PRClient
	Events    *events.Bus
	Claude    ClaudeInvoker
	Escalator Escalator
}

// FeedbackHandler manages responding to PR feedback via Claude
type FeedbackHandler struct {
	github    *github.PRClient
	events    *events.Bus
	claude    ClaudeInvoker
	escalator Escalator
	config    FeedbackConfig
}

// NewFeedbackHandler creates a new feedback handler
func NewFeedbackHandler(cfg FeedbackConfig, deps FeedbackDeps) *FeedbackHandler {
	if cfg.MaxRetries == 0 {
		cfg.MaxRetries = 3
	}
	if cfg.InvocationTimeout == 0 {
		cfg.InvocationTimeout = 10 * time.Minute
	}

	return &FeedbackHandler{
		github:    deps.GitHub,
		events:    deps.Events,
		claude:    deps.Claude,
		escalator: deps.Escalator,
		config:    cfg,
	}
}

// HandleFeedback addresses PR feedback by delegating to Claude
// Returns nil on success, error if Claude cannot address feedback
func (h *FeedbackHandler) HandleFeedback(ctx context.Context, prNumber int, prURL string, worktreePath string, branch string) error {
	comments, err := h.github.GetPRComments(ctx, prNumber)
	if err != nil {
		return fmt.Errorf("failed to get PR comments: %w", err)
	}

	prompt := BuildFeedbackPrompt(prURL, comments)

	// Retry loop for Claude invocation
	var lastErr error
	for attempt := 0; attempt < h.config.MaxRetries; attempt++ {
		invokeCtx, cancel := context.WithTimeout(ctx, h.config.InvocationTimeout)
		err := h.claude.Invoke(invokeCtx, prompt, worktreePath)
		cancel()

		if err == nil {
			lastErr = nil
			break
		}
		lastErr = err
	}

	if lastErr != nil {
		if h.escalator != nil {
			h.escalator.Escalate(ctx, Escalation{
				Severity: SeverityBlocking,
				Unit:     "",
				Title:    "Failed to address PR feedback",
				Message:  fmt.Sprintf("Claude could not address feedback after %d attempts", h.config.MaxRetries),
				Context: map[string]string{
					"pr":    prURL,
					"error": lastErr.Error(),
				},
			})
		}
		return lastErr
	}

	// Verify push happened
	cmd := exec.CommandContext(ctx, "git", "ls-remote", "--heads", "origin", branch)
	cmd.Dir = worktreePath
	if err := cmd.Run(); err != nil {
		return fmt.Errorf("branch not updated on remote after feedback: %w", err)
	}

	if h.events != nil {
		h.events.Emit(events.NewEvent(events.PRFeedbackAddressed, "").WithPR(prNumber))
	}
	return nil
}
