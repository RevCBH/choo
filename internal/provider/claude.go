package provider

import (
	"context"
	"fmt"
	"io"
	"os/exec"

	"golang.org/x/term"
)

// ClaudeProvider implements Provider using the Claude CLI.
// Uses --dangerously-skip-permissions to run without interactive prompts.
type ClaudeProvider struct {
	// command is the path to the claude executable.
	// Defaults to "claude" (resolved via PATH).
	command string

	// streamJSON enables JSON streaming output for visibility.
	streamJSON bool

	// verbose controls whether to show full text output.
	verbose bool

	// showAssistant controls whether to emit assistant text in streaming mode.
	showAssistant bool

	// streamCtx provides context for streaming output.
	streamCtx StreamContext
}

// NewClaude creates a Claude provider with the specified command path.
// If command is empty, defaults to "claude".
func NewClaude(command string) *ClaudeProvider {
	if command == "" {
		command = "claude"
	}
	return &ClaudeProvider{command: command}
}

// NewClaudeWithStreaming creates a Claude provider with JSON streaming enabled.
func NewClaudeWithStreaming(command string, verbose bool) *ClaudeProvider {
	p := NewClaude(command)
	p.streamJSON = true
	p.verbose = verbose
	return p
}

// SetStreaming enables or disables JSON streaming mode.
func (p *ClaudeProvider) SetStreaming(enabled bool) {
	p.streamJSON = enabled
}

// SetVerbose enables or disables verbose output in streaming mode.
func (p *ClaudeProvider) SetVerbose(enabled bool) {
	p.verbose = enabled
}

// SetShowAssistant enables assistant text output in streaming mode.
func (p *ClaudeProvider) SetShowAssistant(enabled bool) {
	p.showAssistant = enabled
}

// SetStreamContext configures streaming display context.
func (p *ClaudeProvider) SetStreamContext(ctx StreamContext) {
	p.streamCtx = ctx
}

// Invoke executes Claude CLI with the given prompt.
// The command runs in workdir with stdout/stderr connected to the provided writers.
// Returns when the subprocess exits or context is cancelled.
func (p *ClaudeProvider) Invoke(ctx context.Context, prompt string, workdir string, stdout, stderr io.Writer) error {
	if p.streamJSON {
		return p.invokeWithStream(ctx, prompt, workdir, stdout, stderr)
	}
	return p.invokeBasic(ctx, prompt, workdir, stdout, stderr)
}

// invokeBasic runs Claude without JSON streaming.
func (p *ClaudeProvider) invokeBasic(ctx context.Context, prompt string, workdir string, stdout, stderr io.Writer) error {
	args := []string{
		"--dangerously-skip-permissions",
		"-p", prompt,
	}

	cmd := exec.CommandContext(ctx, p.command, args...)
	cmd.Dir = workdir
	cmd.Stdout = stdout
	cmd.Stderr = stderr

	if err := cmd.Run(); err != nil {
		return fmt.Errorf("claude invocation failed: %w", err)
	}
	return nil
}

// invokeWithStream runs Claude with JSON streaming output.
func (p *ClaudeProvider) invokeWithStream(ctx context.Context, prompt string, workdir string, stdout, stderr io.Writer) error {
	// Note: --verbose is required when using --print with --output-format=stream-json
	args := []string{
		"--dangerously-skip-permissions",
		"--output-format", "stream-json",
		"--verbose",
		"-p", prompt,
	}

	cmd := exec.CommandContext(ctx, p.command, args...)
	cmd.Dir = workdir

	// Create pipe for stdout to process JSON stream
	stdoutPipe, err := cmd.StdoutPipe()
	if err != nil {
		return fmt.Errorf("create stdout pipe: %w", err)
	}

	cmd.Stderr = stderr

	if err := cmd.Start(); err != nil {
		return fmt.Errorf("start claude: %w", err)
	}

	// Process the stream
	handler := NewStreamHandler(StreamOptions{
		Output:         stdout,
		Verbose:        p.verbose,
		ShowAssistant:  p.showAssistant,
		UseTUI:         isTerminalWriter(stdout),
		EnableProgress: p.streamCtx.EnableProgress,
		SpecsDir:       p.streamCtx.SpecsDir,
		RepoRoot:       p.streamCtx.RepoRoot,
		PhaseTitle:     p.streamCtx.PhaseTitle,
		CounterLabel:   p.streamCtx.CounterLabel,
		PRDPath:        p.streamCtx.PRDPath,
		InitialItems:   p.streamCtx.InitialItems,
		Total:          p.streamCtx.Total,
	})
	streamErr := handler.ProcessStream(stdoutPipe)

	// Wait for command to complete
	cmdErr := cmd.Wait()

	// Report any stream processing errors
	if streamErr != nil {
		fmt.Fprintf(stderr, "stream processing error: %v\n", streamErr)
	}

	if cmdErr != nil {
		return fmt.Errorf("claude invocation failed: %w", cmdErr)
	}

	// Print summary
	messages, tools := handler.Stats()
	if tools > 0 {
		fmt.Fprintf(stdout, "\n📊 %d tool calls completed\n", tools)
	}
	_ = messages // Could show message count if useful

	return nil
}

func isTerminalWriter(w io.Writer) bool {
	if f, ok := w.(interface{ Fd() uintptr }); ok {
		return term.IsTerminal(int(f.Fd()))
	}
	return false
}

// Name returns ProviderClaude
func (p *ClaudeProvider) Name() ProviderType {
	return ProviderClaude
}
