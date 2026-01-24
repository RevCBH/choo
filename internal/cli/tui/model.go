package tui

import (
	"time"

	tea "github.com/charmbracelet/bubbletea"
)

// UnitState tracks the state of a single unit in the TUI
type UnitState struct {
	ID             string
	TotalTasks     int
	CompletedTasks int
	CurrentTask    int
	TaskTitle      string
	Phase          string
	PhaseIcon      string
}

// Model is the bubbletea model for the TUI
type Model struct {
	// Configuration
	TotalUnits  int
	Parallelism int
	Styles      Styles

	// State
	ActiveUnits    map[string]*UnitState
	CompletedUnits int
	FailedUnits    int
	StartTime      time.Time
	LogLines       []string
	LogLimit       int
	ShowLogs       bool
	Width          int
	Height         int

	// Control
	Quitting bool
	Done     bool
}

// NewModel creates a new TUI model
func NewModel(totalUnits, parallelism int) *Model {
	return &Model{
		TotalUnits:  totalUnits,
		Parallelism: parallelism,
		Styles:      DefaultStyles(),
		ActiveUnits: make(map[string]*UnitState),
		StartTime:   time.Now(),
		LogLimit:    500,
	}
}

// Init implements tea.Model
func (m *Model) Init() tea.Cmd {
	return tea.Batch(
		tickCmd(),
	)
}

// TickMsg is sent every second to update the timer
type TickMsg time.Time

// tickCmd returns a command that sends TickMsg every second
func tickCmd() tea.Cmd {
	return tea.Tick(time.Second, func(t time.Time) tea.Msg {
		return TickMsg(t)
	})
}

// DoneMsg signals the TUI should exit
type DoneMsg struct{}

// QuitMsg signals the user requested quit (q or Ctrl+C)
type QuitMsg struct{}

// UnitStartedMsg indicates a unit has started
type UnitStartedMsg struct {
	UnitID         string
	TotalTasks     int
	CompletedTasks int // Already-completed tasks (for resume scenarios)
}

// UnitCompletedMsg indicates a unit has completed
type UnitCompletedMsg struct {
	UnitID string
}

// UnitFailedMsg indicates a unit has failed
type UnitFailedMsg struct {
	UnitID string
	Error  string
}

// TaskPhaseMsg indicates a task phase change
type TaskPhaseMsg struct {
	UnitID    string
	TaskNum   int
	TaskTitle string
	Phase     string
	PhaseIcon string
}

// TaskCompletedMsg indicates a task within a unit completed
type TaskCompletedMsg struct {
	UnitID  string
	TaskNum int
}

// OrchStartedMsg indicates orchestration has started with unit count
type OrchStartedMsg struct {
	TotalUnits int
}
