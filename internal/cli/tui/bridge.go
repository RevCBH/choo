package tui

import (
	"github.com/RevCBH/choo/internal/events"
	tea "github.com/charmbracelet/bubbletea"
)

// Bridge connects the event bus to the bubbletea program
type Bridge struct {
	program *tea.Program
}

// NewBridge creates a new bridge for the given program
func NewBridge(program *tea.Program) *Bridge {
	return &Bridge{
		program: program,
	}
}

// Handler returns an event handler function for the event bus
func (b *Bridge) Handler() events.Handler {
	return func(evt events.Event) {
		msg := b.eventToMsg(evt)
		if msg != nil {
			b.program.Send(msg)
		}
	}
}

// eventToMsg converts an events.Event to a tea.Msg
func (b *Bridge) eventToMsg(evt events.Event) tea.Msg {
	switch evt.Type {
	case events.OrchStarted:
		totalUnits := 0
		if payload, ok := evt.Payload.(map[string]any); ok {
			if t, ok := payload["unit_count"].(int); ok {
				totalUnits = t
			}
		}
		return OrchStartedMsg{
			TotalUnits: totalUnits,
		}

	case events.UnitStarted:
		totalTasks := 0
		completedTasks := 0
		if payload, ok := evt.Payload.(map[string]any); ok {
			if t, ok := payload["total_tasks"].(int); ok {
				totalTasks = t
			}
			if c, ok := payload["completed_tasks"].(int); ok {
				completedTasks = c
			}
		}
		return UnitStartedMsg{
			UnitID:         evt.Unit,
			TotalTasks:     totalTasks,
			CompletedTasks: completedTasks,
		}

	case events.UnitCompleted:
		return UnitCompletedMsg{
			UnitID: evt.Unit,
		}

	case events.UnitFailed:
		return UnitFailedMsg{
			UnitID: evt.Unit,
			Error:  evt.Error,
		}

	case events.TaskClaudeInvoke:
		taskNum := 0
		taskTitle := ""
		if evt.Task != nil {
			taskNum = *evt.Task
		}
		if payload, ok := evt.Payload.(map[string]any); ok {
			if t, ok := payload["title"].(string); ok {
				taskTitle = t
			}
		}
		return TaskPhaseMsg{
			UnitID:    evt.Unit,
			TaskNum:   taskNum,
			TaskTitle: taskTitle,
			Phase:     "invoking Claude",
			PhaseIcon: IconClaude,
		}

	case events.TaskBackpressure:
		taskNum := 0
		taskTitle := ""
		if evt.Task != nil {
			taskNum = *evt.Task
		}
		if payload, ok := evt.Payload.(map[string]any); ok {
			if t, ok := payload["title"].(string); ok {
				taskTitle = t
			}
		}
		return TaskPhaseMsg{
			UnitID:    evt.Unit,
			TaskNum:   taskNum,
			TaskTitle: taskTitle,
			Phase:     "running validation",
			PhaseIcon: IconValidate,
		}

	case events.TaskCommitted:
		taskNum := 0
		if evt.Task != nil {
			taskNum = *evt.Task
		}
		return TaskCompletedMsg{
			UnitID:  evt.Unit,
			TaskNum: taskNum,
		}

	default:
		return nil
	}
}

// SendDone sends a DoneMsg to the program
func (b *Bridge) SendDone() {
	b.program.Send(DoneMsg{})
}

// SendQuit sends a QuitMsg to the program
func (b *Bridge) SendQuit() {
	b.program.Send(QuitMsg{})
}
