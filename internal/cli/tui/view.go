package tui

import (
	"fmt"
	"sort"
	"strings"
	"time"
)

// View implements tea.Model
func (m *Model) View() string {
	if m.Done || m.Quitting {
		return ""
	}

	var b strings.Builder

	// Header
	b.WriteString(m.renderHeader())
	b.WriteString("\n\n")

	// Active units
	b.WriteString(m.renderActiveUnits())

	// Status line
	b.WriteString(m.renderStatusLine())
	b.WriteString("\n")

	// Footer
	b.WriteString(m.renderFooter())

	return b.String()
}

// renderHeader renders the title line with timer and parallelism
func (m *Model) renderHeader() string {
	elapsed := time.Since(m.StartTime).Round(time.Second)
	timer := fmt.Sprintf("[%s]", formatDuration(elapsed))
	parallelism := fmt.Sprintf("Parallelism: %d", m.Parallelism)

	return fmt.Sprintf("%s  %s  %s",
		m.Styles.Title.Render("Choo Orchestrator"),
		m.Styles.Timer.Render(timer),
		m.Styles.Parallelism.Render(parallelism),
	)
}

// renderActiveUnits renders the list of in-progress units
func (m *Model) renderActiveUnits() string {
	if len(m.ActiveUnits) == 0 {
		return "  No active units\n\n"
	}

	var b strings.Builder

	// Sort units by ID for stable display
	unitIDs := make([]string, 0, len(m.ActiveUnits))
	for id := range m.ActiveUnits {
		unitIDs = append(unitIDs, id)
	}
	sort.Strings(unitIDs)

	for _, id := range unitIDs {
		unit := m.ActiveUnits[id]
		b.WriteString(m.renderUnit(unit))
		b.WriteString("\n")
	}

	return b.String()
}

// renderUnit renders a single active unit
func (m *Model) renderUnit(unit *UnitState) string {
	var b strings.Builder

	// Unit header: ● unit-name [████░░░░░░░░] 2/5 tasks
	icon := m.Styles.UnitActive.Render(IconActive)
	name := m.Styles.UnitName.Render(unit.ID)
	progress := m.renderProgressBar(unit.CompletedTasks, unit.TotalTasks, 20)
	taskCount := fmt.Sprintf("%d/%d tasks", unit.CompletedTasks, unit.TotalTasks)

	fmt.Fprintf(&b, "  %s %s %s %s\n", icon, name, progress, taskCount)

	// Phase line: 🤖 Task #3 (Title): invoking Claude
	phaseIcon := m.Styles.PhaseIcon.Render(unit.PhaseIcon)
	var taskDesc string
	if unit.TaskTitle != "" {
		taskDesc = fmt.Sprintf("#%d %s", unit.CurrentTask, unit.TaskTitle)
	} else {
		taskDesc = fmt.Sprintf("#%d", unit.CurrentTask)
	}
	phaseText := m.Styles.PhaseText.Render(fmt.Sprintf("%s: %s", taskDesc, unit.Phase))
	fmt.Fprintf(&b, "      %s %s\n", phaseIcon, phaseText)

	return b.String()
}

// renderProgressBar creates a progress bar of the given width
func (m *Model) renderProgressBar(completed, total, width int) string {
	if total == 0 {
		total = 1 // Avoid division by zero
	}

	filled := min((completed*width)/total, width)

	filledStr := strings.Repeat("█", filled)
	emptyStr := strings.Repeat("░", width-filled)

	return "[" +
		m.Styles.ProgressFilled.Render(filledStr) +
		m.Styles.ProgressEmpty.Render(emptyStr) +
		"]"
}

// renderStatusLine renders the summary status line
func (m *Model) renderStatusLine() string {
	activeCount := len(m.ActiveUnits)

	complete := m.Styles.StatusComplete.Render(fmt.Sprintf("%d complete", m.CompletedUnits))
	failed := m.Styles.StatusFailed.Render(fmt.Sprintf("%d failed", m.FailedUnits))
	active := m.Styles.StatusActive.Render(fmt.Sprintf("%d active", activeCount))

	return fmt.Sprintf("  Units: %d/%d %s | %s | %s",
		m.CompletedUnits+m.FailedUnits,
		m.TotalUnits,
		complete,
		failed,
		active,
	)
}

// renderFooter renders the help text
func (m *Model) renderFooter() string {
	key := m.Styles.FooterKey.Render("q")
	return m.Styles.Footer.Render(fmt.Sprintf("  Press %s to quit", key))
}

// formatDuration formats a duration as HH:MM:SS
func formatDuration(d time.Duration) string {
	h := d / time.Hour
	d -= h * time.Hour
	m := d / time.Minute
	d -= m * time.Minute
	s := d / time.Second

	return fmt.Sprintf("%02d:%02d:%02d", h, m, s)
}
