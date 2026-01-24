package tui

import "github.com/charmbracelet/lipgloss"

// Styles contains all lipgloss styles for the TUI
type Styles struct {
	// Header styling
	Title       lipgloss.Style
	Timer       lipgloss.Style
	Parallelism lipgloss.Style

	// Unit styling
	UnitActive   lipgloss.Style
	UnitComplete lipgloss.Style
	UnitFailed   lipgloss.Style
	UnitName     lipgloss.Style

	// Progress bar colors
	ProgressFilled lipgloss.Style
	ProgressEmpty  lipgloss.Style

	// Phase icons and text
	PhaseIcon lipgloss.Style
	PhaseText lipgloss.Style

	// Footer styling
	Footer    lipgloss.Style
	FooterKey lipgloss.Style

	// Status counts
	StatusComplete lipgloss.Style
	StatusFailed   lipgloss.Style
	StatusActive   lipgloss.Style

	// Log area styling
	LogTitle lipgloss.Style
	LogLine  lipgloss.Style
}

// DefaultStyles returns the default TUI styles
func DefaultStyles() Styles {
	return Styles{
		Title:       lipgloss.NewStyle().Bold(true).Foreground(lipgloss.Color("39")),
		Timer:       lipgloss.NewStyle().Foreground(lipgloss.Color("245")),
		Parallelism: lipgloss.NewStyle().Foreground(lipgloss.Color("245")),

		UnitActive:   lipgloss.NewStyle().Foreground(lipgloss.Color("214")),
		UnitComplete: lipgloss.NewStyle().Foreground(lipgloss.Color("42")),
		UnitFailed:   lipgloss.NewStyle().Foreground(lipgloss.Color("196")),
		UnitName:     lipgloss.NewStyle().Bold(true),

		ProgressFilled: lipgloss.NewStyle().Foreground(lipgloss.Color("214")),
		ProgressEmpty:  lipgloss.NewStyle().Foreground(lipgloss.Color("240")),

		PhaseIcon: lipgloss.NewStyle().Foreground(lipgloss.Color("245")),
		PhaseText: lipgloss.NewStyle().Foreground(lipgloss.Color("250")).Italic(true),

		Footer:    lipgloss.NewStyle().Foreground(lipgloss.Color("245")).MarginTop(1),
		FooterKey: lipgloss.NewStyle().Foreground(lipgloss.Color("214")).Bold(true),

		StatusComplete: lipgloss.NewStyle().Foreground(lipgloss.Color("42")),
		StatusFailed:   lipgloss.NewStyle().Foreground(lipgloss.Color("196")),
		StatusActive:   lipgloss.NewStyle().Foreground(lipgloss.Color("214")),

		LogTitle: lipgloss.NewStyle().Foreground(lipgloss.Color("240")).Bold(true),
		LogLine:  lipgloss.NewStyle().Foreground(lipgloss.Color("245")),
	}
}

// Icons used in the TUI
const (
	IconActive   = "●"
	IconComplete = "✓"
	IconFailed   = "✗"
	IconClaude   = "🤖"
	IconValidate = "🧪"
	IconCommit   = "📝"
	IconWaiting  = "⏳"
)
