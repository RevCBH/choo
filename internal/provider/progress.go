package provider

import (
	"fmt"
	"strings"
	"sync"
	"time"
	"unicode/utf8"
)

// ItemStatus represents a spec progress status.
type ItemStatus int

const (
	StatusPending ItemStatus = iota
	StatusWorking
	StatusDone
	StatusWarning
	StatusError
)

// ProgressItem tracks a single spec file's progress.
type ProgressItem struct {
	Path        string
	Status      ItemStatus
	Message     string
	CurrentTask string // For ralph-prep: the task currently being generated
	Started     time.Time
	Updated     time.Time
}

// ProgressOptions configures a SpecProgress instance.
type ProgressOptions struct {
	PhaseTitle   string
	PRDPath      string
	SpecsDir     string
	CounterLabel string
	Total        int
	InitialItems []string
	BarWidth     int
}

// SpecProgress tracks live progress for spec generation/validation.
type SpecProgress struct {
	mu           sync.Mutex
	phaseTitle   string
	prdPath      string
	specsDir     string
	counterLabel string
	total        int
	items        []*ProgressItem
	itemIndex    map[string]int
	started      time.Time
	barWidth     int

	spinnerFrames []string
	spinnerIndex  int
	spinnerStop   chan struct{}
	spinnerActive bool
}

// StreamContext provides configuration for streaming progress output.
type StreamContext struct {
	PhaseTitle     string
	CounterLabel   string
	PRDPath        string
	SpecsDir       string
	RepoRoot       string
	InitialItems   []string
	Total          int
	EnableProgress bool
}

// StreamContextSetter allows callers to provide context for stream rendering.
type StreamContextSetter interface {
	SetStreamContext(ctx StreamContext)
}

// NewSpecProgress creates a SpecProgress with optional initial items.
func NewSpecProgress(opts ProgressOptions) *SpecProgress {
	p := &SpecProgress{
		phaseTitle:   opts.PhaseTitle,
		prdPath:      opts.PRDPath,
		specsDir:     opts.SpecsDir,
		counterLabel: opts.CounterLabel,
		total:        opts.Total,
		items:        make([]*ProgressItem, 0),
		itemIndex:    make(map[string]int),
		started:      time.Now(),
		barWidth:     opts.BarWidth,
		spinnerFrames: []string{
			"⠋", "⠙", "⠹", "⠸", "⠼",
			"⠴", "⠦", "⠧", "⠇", "⠏",
		},
	}

	for _, item := range opts.InitialItems {
		p.upsertLocked(item, StatusPending, "")
	}
	if p.total < len(p.items) {
		p.total = len(p.items)
	}

	if p.counterLabel == "" {
		p.counterLabel = "items"
	}

	return p
}

// StartSpinner begins the spinner ticker.
func (p *SpecProgress) StartSpinner(onTick func()) {
	p.mu.Lock()
	if p.spinnerActive {
		p.mu.Unlock()
		return
	}
	p.spinnerActive = true
	p.spinnerStop = make(chan struct{})
	stopCh := p.spinnerStop
	p.mu.Unlock()

	go func() {
		ticker := time.NewTicker(100 * time.Millisecond)
		defer ticker.Stop()
		for {
			select {
			case <-ticker.C:
				p.mu.Lock()
				if !p.hasWorkingLocked() {
					p.mu.Unlock()
					continue
				}
				p.spinnerIndex = (p.spinnerIndex + 1) % len(p.spinnerFrames)
				p.mu.Unlock()
				if onTick != nil {
					onTick()
				}
			case <-stopCh:
				return
			}
		}
	}()
}

// StopSpinner halts the spinner ticker.
func (p *SpecProgress) StopSpinner() {
	p.mu.Lock()
	if !p.spinnerActive {
		p.mu.Unlock()
		return
	}
	stopCh := p.spinnerStop
	p.spinnerActive = false
	p.spinnerStop = nil
	p.mu.Unlock()
	close(stopCh)
}

// UpdateItem updates or adds a progress item.
func (p *SpecProgress) UpdateItem(path string, status ItemStatus, message string) bool {
	p.mu.Lock()
	changed := p.upsertLocked(path, status, message)
	p.mu.Unlock()
	return changed
}

// SetCurrentTask sets the current task being generated for a spec.
// Uses case-insensitive matching for the spec path (task dirs are lowercase, spec files may be uppercase).
// Returns true if the item was found and updated.
func (p *SpecProgress) SetCurrentTask(specPath, taskName string) bool {
	p.mu.Lock()
	defer p.mu.Unlock()

	idx := p.findItemIndexLocked(specPath)
	if idx < 0 {
		return false
	}
	item := p.items[idx]
	if item.CurrentTask != taskName {
		item.CurrentTask = taskName
		item.Updated = time.Now()
		return true
	}
	return false
}

// findItemIndexLocked finds an item by path, using case-insensitive matching as fallback.
// Returns -1 if not found. Caller must hold the lock.
func (p *SpecProgress) findItemIndexLocked(path string) int {
	// Try exact match first
	if idx, ok := p.itemIndex[path]; ok {
		return idx
	}
	// Fall back to case-insensitive match
	pathLower := strings.ToLower(path)
	for i, item := range p.items {
		if strings.ToLower(item.Path) == pathLower {
			return i
		}
	}
	return -1
}

// FindSpecPath finds the actual spec path using case-insensitive matching.
// Returns the canonical path and true if found, empty string and false otherwise.
func (p *SpecProgress) FindSpecPath(specPath string) (string, bool) {
	p.mu.Lock()
	defer p.mu.Unlock()

	idx := p.findItemIndexLocked(specPath)
	if idx < 0 {
		return "", false
	}
	return p.items[idx].Path, true
}

// ClearCurrentTask clears the current task for a spec.
func (p *SpecProgress) ClearCurrentTask(specPath string) bool {
	return p.SetCurrentTask(specPath, "")
}

// MarkAllDone marks any pending/working items as done.
func (p *SpecProgress) MarkAllDone() bool {
	p.mu.Lock()
	changed := false
	for _, item := range p.items {
		if item.Status == StatusPending || item.Status == StatusWorking {
			item.Status = StatusDone
			item.Updated = time.Now()
			changed = true
		}
	}
	p.mu.Unlock()
	return changed
}

// RenderLines renders the full progress UI as lines.
func (p *SpecProgress) RenderLines(termWidth int) []string {
	p.mu.Lock()
	defer p.mu.Unlock()

	lines := []string{}

	if p.phaseTitle != "" {
		header := "── " + p.phaseTitle + " "
		if termWidth > 0 {
			remaining := termWidth - runeCount(header)
			if remaining > 0 {
				header += strings.Repeat("─", remaining)
			}
		}
		lines = append(lines, header)
	}

	if p.prdPath != "" && p.specsDir != "" {
		lines = append(lines, fmt.Sprintf("PRD: %s → %s/", p.prdPath, p.specsDir))
	}

	barWidth := p.barWidth
	if barWidth <= 0 {
		barWidth = defaultBarWidth(termWidth)
	}
	lines = append(lines, p.renderBarLocked(barWidth))
	lines = append(lines, "")

	for _, item := range p.items {
		icon := p.statusIconLocked(item.Status)
		line := fmt.Sprintf("  %s %s", icon, item.Path)
		if item.CurrentTask != "" && item.Status == StatusWorking {
			line += " → " + item.CurrentTask
		} else if item.Message != "" && (item.Status == StatusWarning || item.Status == StatusError) {
			line += " - " + item.Message
		}
		lines = append(lines, line)
	}

	return lines
}

// Snapshot returns a copy of the items for external inspection.
func (p *SpecProgress) Snapshot() ([]*ProgressItem, int) {
	p.mu.Lock()
	defer p.mu.Unlock()
	items := make([]*ProgressItem, 0, len(p.items))
	for _, item := range p.items {
		c := *item
		items = append(items, &c)
	}
	return items, p.total
}

func (p *SpecProgress) upsertLocked(path string, status ItemStatus, message string) bool {
	if path == "" {
		return false
	}
	now := time.Now()
	if idx, ok := p.itemIndex[path]; ok {
		item := p.items[idx]
		changed := item.Status != status || item.Message != message
		item.Status = status
		item.Message = message
		item.Updated = now
		if item.Started.IsZero() && status == StatusWorking {
			item.Started = now
		}
		return changed
	}

	item := &ProgressItem{
		Path:    path,
		Status:  status,
		Message: message,
		Started: now,
		Updated: now,
	}
	p.itemIndex[path] = len(p.items)
	p.items = append(p.items, item)
	if p.total < len(p.items) {
		p.total = len(p.items)
	}
	return true
}

func (p *SpecProgress) hasWorkingLocked() bool {
	for _, item := range p.items {
		if item.Status == StatusWorking {
			return true
		}
	}
	return false
}

func (p *SpecProgress) countDoneLocked() int {
	count := 0
	for _, item := range p.items {
		switch item.Status {
		case StatusDone, StatusWarning, StatusError:
			count++
		}
	}
	return count
}

func (p *SpecProgress) statusIconLocked(status ItemStatus) string {
	switch status {
	case StatusWorking:
		if len(p.spinnerFrames) == 0 {
			return "…"
		}
		return p.spinnerFrames[p.spinnerIndex]
	case StatusDone:
		return "✓"
	case StatusWarning:
		return "⚠"
	case StatusError:
		return "✗"
	case StatusPending:
		fallthrough
	default:
		return "·"
	}
}

func (p *SpecProgress) renderBarLocked(width int) string {
	done := p.countDoneLocked()
	total := p.total
	if total < done {
		total = done
	}
	filled := 0
	if total > 0 {
		pct := float64(done) / float64(total)
		filled = int(pct * float64(width))
		if filled > width {
			filled = width
		}
	}
	bar := fmt.Sprintf("[%s%s]", strings.Repeat("█", filled), strings.Repeat("░", width-filled))
	return fmt.Sprintf("%s %d/%d %s", bar, done, total, p.counterLabel)
}

func defaultBarWidth(termWidth int) int {
	if termWidth <= 0 {
		return 20
	}
	if termWidth < 40 {
		return 12
	}
	if termWidth > 120 {
		return 30
	}
	return 20
}

func runeCount(s string) int {
	return utf8.RuneCountInString(s)
}
