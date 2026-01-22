package spec

import (
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"os"
	"path/filepath"
	"time"

	"github.com/RevCBH/choo/internal/config"
	"gopkg.in/yaml.v3"
)

// Phase represents a workflow phase.
type Phase string

const (
	// PhaseNotStarted indicates the workflow hasn't started.
	PhaseNotStarted Phase = "not_started"
	// PhaseSpecGeneration indicates spec generation is in progress.
	PhaseSpecGeneration Phase = "spec_generation"
	// PhaseValidation indicates spec validation is in progress.
	PhaseValidation Phase = "validation"
	// PhaseRalphPrep indicates ralph-prep (task generation) is in progress.
	PhaseRalphPrep Phase = "ralph_prep"
	// PhaseComplete indicates the workflow completed successfully.
	PhaseComplete Phase = "complete"
	// PhaseFailed indicates the workflow failed.
	PhaseFailed Phase = "failed"
)

// SpecState tracks the state of a spec generation workflow.
type SpecState struct {
	// PRDID is an identifier for the PRD (derived from filename).
	PRDID string `yaml:"prd_id"`

	// PRDPath is the path to the PRD file (relative to repo root).
	PRDPath string `yaml:"prd_path"`

	// SpecsDir is the output directory for generated specs.
	SpecsDir string `yaml:"specs_dir"`

	// RepoRoot is the absolute path to the target repository.
	RepoRoot string `yaml:"repo_root"`

	// Phase is the current workflow phase.
	Phase Phase `yaml:"phase"`

	// StartedAt is when the workflow started.
	StartedAt time.Time `yaml:"started_at"`

	// LastPhaseAt is when the last phase transition occurred.
	LastPhaseAt time.Time `yaml:"last_phase_at"`

	// SpecCount is the number of specs generated.
	SpecCount int `yaml:"spec_count"`

	// TaskCount is the number of tasks generated during ralph-prep.
	TaskCount int `yaml:"task_count"`

	// ValidationOK indicates whether validation passed.
	ValidationOK bool `yaml:"validation_ok"`

	// Error contains any error message if the workflow failed.
	Error string `yaml:"error,omitempty"`
}

// NewSpecState creates a new SpecState for a workflow.
func NewSpecState(prdPath, specsDir, repoRoot string) *SpecState {
	// Extract PRD ID from filename
	prdID := filepath.Base(prdPath)
	if ext := filepath.Ext(prdID); ext != "" {
		prdID = prdID[:len(prdID)-len(ext)]
	}

	now := time.Now()
	return &SpecState{
		PRDID:       prdID,
		PRDPath:     prdPath,
		SpecsDir:    specsDir,
		RepoRoot:    repoRoot,
		Phase:       PhaseNotStarted,
		StartedAt:   now,
		LastPhaseAt: now,
	}
}

// SetPhase updates the current phase and last phase timestamp.
func (s *SpecState) SetPhase(phase Phase) {
	s.Phase = phase
	s.LastPhaseAt = time.Now()
}

// SetError marks the workflow as failed with an error message.
func (s *SpecState) SetError(err error) {
	s.Phase = PhaseFailed
	s.LastPhaseAt = time.Now()
	if err != nil {
		s.Error = err.Error()
	}
}

// IsResumable returns true if the workflow can be resumed from the current state.
func (s *SpecState) IsResumable() bool {
	switch s.Phase {
	case PhaseNotStarted, PhaseSpecGeneration, PhaseValidation, PhaseRalphPrep:
		return true
	default:
		return false
	}
}

// NextPhase returns the next phase in the workflow.
func (s *SpecState) NextPhase() Phase {
	switch s.Phase {
	case PhaseNotStarted:
		return PhaseSpecGeneration
	case PhaseSpecGeneration:
		return PhaseValidation
	case PhaseValidation:
		return PhaseRalphPrep
	case PhaseRalphPrep:
		return PhaseComplete
	default:
		return s.Phase
	}
}

// Order returns the numeric order of a phase for comparison.
func (p Phase) Order() int {
	switch p {
	case PhaseNotStarted:
		return 0
	case PhaseSpecGeneration:
		return 1
	case PhaseValidation:
		return 2
	case PhaseRalphPrep:
		return 3
	case PhaseComplete:
		return 4
	case PhaseFailed:
		return -1
	default:
		return -1
	}
}

// Save persists the state to the global state directory.
func (s *SpecState) Save() error {
	stateDir, err := config.EnsureGlobalSpecStateDir()
	if err != nil {
		return fmt.Errorf("ensure state dir: %w", err)
	}

	statePath := filepath.Join(stateDir, repoHash(s.RepoRoot)+".yaml")

	data, err := yaml.Marshal(s)
	if err != nil {
		return fmt.Errorf("marshal state: %w", err)
	}

	if err := os.WriteFile(statePath, data, 0644); err != nil {
		return fmt.Errorf("write state: %w", err)
	}

	return nil
}

// LoadState loads the state for a repository from the global state directory.
func LoadState(repoRoot string) (*SpecState, error) {
	stateDir, err := config.EnsureGlobalSpecStateDir()
	if err != nil {
		return nil, fmt.Errorf("ensure state dir: %w", err)
	}

	statePath := filepath.Join(stateDir, repoHash(repoRoot)+".yaml")

	data, err := os.ReadFile(statePath)
	if os.IsNotExist(err) {
		return nil, nil // No state exists
	}
	if err != nil {
		return nil, fmt.Errorf("read state: %w", err)
	}

	var state SpecState
	if err := yaml.Unmarshal(data, &state); err != nil {
		return nil, fmt.Errorf("unmarshal state: %w", err)
	}

	return &state, nil
}

// ClearState removes the state file for a repository.
func ClearState(repoRoot string) error {
	stateDir, err := config.EnsureGlobalSpecStateDir()
	if err != nil {
		return fmt.Errorf("ensure state dir: %w", err)
	}

	statePath := filepath.Join(stateDir, repoHash(repoRoot)+".yaml")

	if err := os.Remove(statePath); err != nil && !os.IsNotExist(err) {
		return fmt.Errorf("remove state: %w", err)
	}

	return nil
}

// repoHash returns a deterministic hash of a repository path for state file naming.
func repoHash(repoRoot string) string {
	// Normalize the path
	absPath, err := filepath.Abs(repoRoot)
	if err != nil {
		absPath = repoRoot
	}

	// Hash it
	hash := sha256.Sum256([]byte(absPath))
	return hex.EncodeToString(hash[:8]) // First 8 bytes (16 hex chars)
}

// StateInfo returns a summary of the current state for display.
func (s *SpecState) StateInfo() string {
	return fmt.Sprintf("PRD: %s, Phase: %s, Started: %s",
		s.PRDID, s.Phase, s.StartedAt.Format(time.RFC3339))
}
