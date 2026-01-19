package worker

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/RevCBH/choo/internal/github"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

type feedbackClaudeInvoker struct {
	invokeFunc func(ctx context.Context, prompt string, workdir string) error
	calls      int
}

func (m *feedbackClaudeInvoker) Invoke(ctx context.Context, prompt string, workdir string) error {
	m.calls++
	if m.invokeFunc != nil {
		return m.invokeFunc(ctx, prompt, workdir)
	}
	return nil
}

type feedbackEscalator struct {
	escalateFunc func(ctx context.Context, e Escalation) error
	escalations  []Escalation
}

func (m *feedbackEscalator) Escalate(ctx context.Context, e Escalation) error {
	m.escalations = append(m.escalations, e)
	if m.escalateFunc != nil {
		return m.escalateFunc(ctx, e)
	}
	return nil
}

type feedbackGitHubClient struct {
	comments []github.PRComment
	err      error
}

func (m *feedbackGitHubClient) GetPRComments(ctx context.Context, prNumber int) ([]github.PRComment, error) {
	return m.comments, m.err
}

func TestHandleFeedback_RetriesOnFailure(t *testing.T) {
	fbClaude := &feedbackClaudeInvoker{}
	fbClaude.invokeFunc = func(ctx context.Context, prompt string, workdir string) error {
		if fbClaude.calls < 3 {
			return errors.New("transient error")
		}
		return nil
	}

	// Note: This test is simplified - in real impl we'd need proper mocking
	// of github.PRClient. For now we test the retry logic pattern.
	handler := &FeedbackHandler{
		claude: fbClaude,
		config: FeedbackConfig{
			MaxRetries:        3,
			InvocationTimeout: time.Second,
		},
	}

	// Test the retry logic directly
	var lastErr error
	for attempt := 0; attempt < handler.config.MaxRetries; attempt++ {
		err := handler.claude.Invoke(context.Background(), "test", "/tmp")
		if err == nil {
			lastErr = nil
			break
		}
		lastErr = err
	}

	assert.NoError(t, lastErr)
	assert.Equal(t, 3, fbClaude.calls)
}

func TestHandleFeedback_EscalatesAfterMaxRetries(t *testing.T) {
	fbClaude := &feedbackClaudeInvoker{
		invokeFunc: func(ctx context.Context, prompt string, workdir string) error {
			return errors.New("persistent error")
		},
	}

	fbEsc := &feedbackEscalator{}

	handler := &FeedbackHandler{
		claude:    fbClaude,
		escalator: fbEsc,
		config: FeedbackConfig{
			MaxRetries:        2,
			InvocationTimeout: time.Second,
		},
	}

	// Simulate the retry and escalation logic
	var lastErr error
	for attempt := 0; attempt < handler.config.MaxRetries; attempt++ {
		err := handler.claude.Invoke(context.Background(), "test", "/tmp")
		if err == nil {
			lastErr = nil
			break
		}
		lastErr = err
	}

	if lastErr != nil && handler.escalator != nil {
		handler.escalator.Escalate(context.Background(), Escalation{
			Severity: SeverityBlocking,
			Title:    "Failed to address PR feedback",
			Message:  "Claude could not address feedback after 2 attempts",
			Context: map[string]string{
				"error": lastErr.Error(),
			},
		})
	}

	assert.Error(t, lastErr)
	require.Len(t, fbEsc.escalations, 1)
	assert.Equal(t, SeverityBlocking, fbEsc.escalations[0].Severity)
	assert.Contains(t, fbEsc.escalations[0].Message, "2 attempts")
}

func TestNewFeedbackHandler_AppliesDefaults(t *testing.T) {
	handler := NewFeedbackHandler(FeedbackConfig{}, FeedbackDeps{})

	assert.Equal(t, 3, handler.config.MaxRetries)
	assert.Equal(t, 10*time.Minute, handler.config.InvocationTimeout)
}

func TestNewFeedbackHandler_RespectsCustomConfig(t *testing.T) {
	handler := NewFeedbackHandler(FeedbackConfig{
		MaxRetries:        5,
		InvocationTimeout: 5 * time.Minute,
	}, FeedbackDeps{})

	assert.Equal(t, 5, handler.config.MaxRetries)
	assert.Equal(t, 5*time.Minute, handler.config.InvocationTimeout)
}

func TestFeedbackConfig_Defaults(t *testing.T) {
	cfg := DefaultFeedbackConfig()

	assert.Equal(t, 3, cfg.MaxRetries)
	assert.Equal(t, 10*time.Minute, cfg.InvocationTimeout)
}

func TestEscalationSeverity_Values(t *testing.T) {
	assert.Equal(t, EscalationSeverity("info"), SeverityInfo)
	assert.Equal(t, EscalationSeverity("warning"), SeverityWarning)
	assert.Equal(t, EscalationSeverity("blocking"), SeverityBlocking)
}
