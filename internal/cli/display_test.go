package cli

import (
	"strings"
	"testing"

	"github.com/anthropics/choo/internal/discovery"
)

func TestRenderProgressBar_Empty(t *testing.T) {
	result := RenderProgressBar(0.0, 10)

	// Must contain 0%
	if !strings.Contains(result, "  0%") {
		t.Errorf("Expected result to contain '  0%%', got %q", result)
	}

	// Must contain no filled blocks (only empty blocks)
	if strings.Contains(result, "█") {
		t.Errorf("Expected no filled blocks for 0%% progress, got %q", result)
	}

	// Check that we have the right number of empty blocks
	if !strings.Contains(result, "░░░░░░░░░░") {
		t.Errorf("Expected 10 empty blocks, got %q", result)
	}
}

func TestRenderProgressBar_Half(t *testing.T) {
	result := RenderProgressBar(0.5, 10)

	// Must contain 50%
	if !strings.Contains(result, " 50%") {
		t.Errorf("Expected result to contain ' 50%%', got %q", result)
	}

	// Must contain 5 filled blocks
	filledCount := strings.Count(result, "█")
	if filledCount != 5 {
		t.Errorf("Expected 5 filled blocks for 50%% progress, got %d in %q", filledCount, result)
	}
}

func TestRenderProgressBar_Full(t *testing.T) {
	result := RenderProgressBar(1.0, 10)

	// Must contain 100%
	if !strings.Contains(result, "100%") {
		t.Errorf("Expected result to contain '100%%', got %q", result)
	}

	// Must contain all filled blocks (no empty blocks)
	if strings.Contains(result, "░") {
		t.Errorf("Expected no empty blocks for 100%% progress, got %q", result)
	}

	// Check that we have the right number of filled blocks
	if !strings.Contains(result, "██████████") {
		t.Errorf("Expected 10 filled blocks, got %q", result)
	}
}

func TestGetStatusSymbol_Complete(t *testing.T) {
	result := GetStatusSymbol(discovery.TaskStatusComplete)
	if result != SymbolComplete {
		t.Errorf("Expected %q for complete status, got %q", SymbolComplete, result)
	}
}

func TestGetStatusSymbol_InProgress(t *testing.T) {
	result := GetStatusSymbol(discovery.TaskStatusInProgress)
	if result != SymbolInProgress {
		t.Errorf("Expected %q for in_progress status, got %q", SymbolInProgress, result)
	}
}

func TestGetStatusSymbol_Pending(t *testing.T) {
	result := GetStatusSymbol(discovery.TaskStatusPending)
	if result != SymbolPending {
		t.Errorf("Expected %q for pending status, got %q", SymbolPending, result)
	}
}

func TestGetStatusSymbol_Failed(t *testing.T) {
	result := GetStatusSymbol(discovery.TaskStatusFailed)
	if result != SymbolFailed {
		t.Errorf("Expected %q for failed status, got %q", SymbolFailed, result)
	}
}
