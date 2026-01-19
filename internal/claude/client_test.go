package claude

import (
	"context"
	"testing"
	"time"
)

func TestNewCLIClient(t *testing.T) {
	client := NewCLIClient()
	if client == nil {
		t.Fatal("NewCLIClient returned nil")
	}
	if client.claudeBinary != "claude" {
		t.Errorf("expected binary 'claude', got %q", client.claudeBinary)
	}
}

func TestNewCLIClientWithBinary(t *testing.T) {
	client := NewCLIClientWithBinary("/custom/path/claude")
	if client == nil {
		t.Fatal("NewCLIClientWithBinary returned nil")
	}
	if client.claudeBinary != "/custom/path/claude" {
		t.Errorf("expected binary '/custom/path/claude', got %q", client.claudeBinary)
	}
}

func TestExecute_ValidationErrors(t *testing.T) {
	client := NewCLIClient()
	ctx := context.Background()

	tests := []struct {
		name    string
		opts    ExecuteOptions
		wantErr error
	}{
		{
			name: "empty prompt",
			opts: ExecuteOptions{
				Prompt:  "",
				WorkDir: "/tmp",
			},
			wantErr: ErrEmptyPrompt,
		},
		{
			name: "empty workdir",
			opts: ExecuteOptions{
				Prompt:  "test prompt",
				WorkDir: "",
			},
			wantErr: ErrEmptyWorkDir,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			_, err := client.Execute(ctx, tt.opts)
			if err != tt.wantErr {
				t.Errorf("expected error %v, got %v", tt.wantErr, err)
			}
		})
	}
}

func TestBuildArgs(t *testing.T) {
	client := NewCLIClient()

	tests := []struct {
		name     string
		opts     ExecuteOptions
		expected []string
	}{
		{
			name: "basic prompt",
			opts: ExecuteOptions{
				Prompt:                     "test prompt",
				DangerouslySkipPermissions: true,
			},
			expected: []string{"--dangerously-skip-permissions", "-p", "test prompt"},
		},
		{
			name: "with max turns",
			opts: ExecuteOptions{
				Prompt:                     "test prompt",
				DangerouslySkipPermissions: true,
				MaxTurns:                   5,
			},
			expected: []string{"--dangerously-skip-permissions", "-p", "test prompt", "--max-turns", "5"},
		},
		{
			name: "without skip permissions",
			opts: ExecuteOptions{
				Prompt:                     "test prompt",
				DangerouslySkipPermissions: false,
			},
			expected: []string{"-p", "test prompt"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			args := client.buildArgs(tt.opts)
			if len(args) != len(tt.expected) {
				t.Errorf("expected %d args, got %d: %v", len(tt.expected), len(args), args)
				return
			}
			for i, arg := range args {
				if arg != tt.expected[i] {
					t.Errorf("arg[%d]: expected %q, got %q", i, tt.expected[i], arg)
				}
			}
		})
	}
}

func TestDefaultExecuteOptions(t *testing.T) {
	opts := DefaultExecuteOptions()

	if !opts.DangerouslySkipPermissions {
		t.Error("expected DangerouslySkipPermissions to be true")
	}
	if opts.MaxTurns != 10 {
		t.Errorf("expected MaxTurns to be 10, got %d", opts.MaxTurns)
	}
	if opts.Timeout != 10*time.Minute {
		t.Errorf("expected Timeout to be 10 minutes, got %v", opts.Timeout)
	}
}

func TestMockClient(t *testing.T) {
	callCount := 0
	mock := &MockClient{
		ExecuteFunc: func(ctx context.Context, opts ExecuteOptions) (*ExecuteResult, error) {
			callCount++
			return &ExecuteResult{Success: true}, nil
		},
	}

	ctx := context.Background()
	opts := ExecuteOptions{
		Prompt:  "test",
		WorkDir: "/tmp",
	}

	result, err := mock.Execute(ctx, opts)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !result.Success {
		t.Error("expected success")
	}
	if callCount != 1 {
		t.Errorf("expected 1 call, got %d", callCount)
	}
}

func TestMockClient_DefaultBehavior(t *testing.T) {
	// Test that MockClient with no ExecuteFunc returns success
	mock := &MockClient{}

	ctx := context.Background()
	opts := ExecuteOptions{
		Prompt:  "test",
		WorkDir: "/tmp",
	}

	result, err := mock.Execute(ctx, opts)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !result.Success {
		t.Error("expected default success")
	}
}
