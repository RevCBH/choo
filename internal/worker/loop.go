package worker

import "github.com/anthropics/choo/internal/discovery"

// LoopState tracks the Ralph loop execution state
type LoopState struct {
	Iteration      int
	Phase          LoopPhase
	ReadyTasks     []*discovery.Task
	CompletedTasks []*discovery.Task
	FailedTasks    []*discovery.Task
	CurrentTask    *discovery.Task
}

// LoopPhase indicates the current loop phase
type LoopPhase string

const (
	PhaseTaskSelection  LoopPhase = "task_selection"
	PhaseClaudeInvoke   LoopPhase = "claude_invoke"
	PhaseBackpressure   LoopPhase = "backpressure"
	PhaseCommit         LoopPhase = "commit"
	PhaseBaselineChecks LoopPhase = "baseline_checks"
	PhaseBaselineFix    LoopPhase = "baseline_fix"
	PhasePRCreation     LoopPhase = "pr_creation"
)
