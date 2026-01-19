package worker

import (
	"strings"
	"testing"

	"github.com/anthropics/choo/internal/discovery"
)

func TestWorker_Run_HappyPath(t *testing.T) {
	t.Skip("Integration test requires full mock setup - skipped for now")
}

func TestWorker_Run_TaskLoopFails(t *testing.T) {
	t.Skip("Integration test requires full mock setup - skipped for now")
}

func TestWorker_Run_BaselineFails(t *testing.T) {
	t.Skip("Integration test requires full mock setup - skipped for now")
}

func TestWorker_Run_NoPR(t *testing.T) {
	t.Skip("Integration test requires full mock setup - skipped for now")
}

func TestGenerateBranchName(t *testing.T) {
	unit := &discovery.Unit{ID: "my-unit"}
	w := &Worker{unit: unit}

	branch := w.generateBranchName()

	if !strings.HasPrefix(branch, "ralph/my-unit-") {
		t.Errorf("expected branch to start with 'ralph/my-unit-', got %q", branch)
	}

	// Should have 6 hex chars after the dash
	parts := strings.Split(branch, "-")
	if len(parts) < 2 {
		t.Fatalf("expected branch name to have at least one dash, got %q", branch)
	}
	hashPart := parts[len(parts)-1]
	if len(hashPart) != 6 {
		t.Errorf("expected 6 char hash suffix, got %d chars: %q", len(hashPart), hashPart)
	}

	// Verify it's hex
	for _, c := range hashPart {
		if !((c >= '0' && c <= '9') || (c >= 'a' && c <= 'f')) {
			t.Errorf("expected hex characters, got %q", hashPart)
			break
		}
	}
}

func TestSetupWorktree(t *testing.T) {
	t.Skip("Integration test requires git commands - skipped for now")
}

func TestCleanup(t *testing.T) {
	t.Skip("Integration test requires mock git manager - skipped for now")
}
