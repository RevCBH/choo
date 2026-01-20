package tui

import tea "github.com/charmbracelet/bubbletea"

// Update implements tea.Model
func (m *Model) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.KeyMsg:
		switch msg.String() {
		case "q", "ctrl+c":
			m.Quitting = true
			return m, tea.Quit
		}

	case TickMsg:
		// Continue ticking for timer updates
		return m, tickCmd()

	case DoneMsg:
		m.Done = true
		return m, tea.Quit

	case QuitMsg:
		m.Quitting = true
		return m, tea.Quit

	case UnitStartedMsg:
		m.ActiveUnits[msg.UnitID] = &UnitState{
			ID:             msg.UnitID,
			TotalTasks:     msg.TotalTasks,
			CompletedTasks: msg.CompletedTasks, // Use already-completed count for resume
			CurrentTask:    msg.CompletedTasks + 1,
			Phase:          "starting",
			PhaseIcon:      IconWaiting,
		}

	case UnitCompletedMsg:
		delete(m.ActiveUnits, msg.UnitID)
		m.CompletedUnits++

	case UnitFailedMsg:
		delete(m.ActiveUnits, msg.UnitID)
		m.FailedUnits++

	case TaskPhaseMsg:
		if unit, ok := m.ActiveUnits[msg.UnitID]; ok {
			unit.CurrentTask = msg.TaskNum
			unit.TaskTitle = msg.TaskTitle
			unit.Phase = msg.Phase
			unit.PhaseIcon = msg.PhaseIcon
		}

	case TaskCompletedMsg:
		if unit, ok := m.ActiveUnits[msg.UnitID]; ok {
			unit.CompletedTasks++
		}

	case OrchStartedMsg:
		m.TotalUnits = msg.TotalUnits
	}

	return m, nil
}
