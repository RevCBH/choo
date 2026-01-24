package tui

import (
	"bytes"
	"strings"
	"sync"

	tea "github.com/charmbracelet/bubbletea"
)

// LogMsg is emitted when a log line should be appended to the TUI.
type LogMsg struct {
	Line string
}

// LogWriter streams log output into the TUI.
type LogWriter struct {
	program *tea.Program
	mu      sync.Mutex
	buffer  bytes.Buffer
	maxLine int
	lines   chan string
}

// NewLogWriter creates a LogWriter that sends log lines into the program.
func NewLogWriter(program *tea.Program) *LogWriter {
	w := &LogWriter{
		program: program,
		maxLine: 2000,
		lines:   make(chan string, 200),
	}
	go func() {
		for line := range w.lines {
			if w.program != nil {
				w.program.Send(LogMsg{Line: line})
			}
		}
	}()
	return w
}

// Write implements io.Writer, splitting log output into lines.
func (w *LogWriter) Write(p []byte) (int, error) {
	w.mu.Lock()
	defer w.mu.Unlock()

	_, _ = w.buffer.Write(p)

	for {
		data := w.buffer.Bytes()
		idx := bytes.IndexByte(data, '\n')
		if idx == -1 {
			break
		}

		line := string(data[:idx])
		w.buffer.Next(idx + 1)
		w.sendLine(line)
	}

	return len(p), nil
}

// Flush sends any buffered partial line.
func (w *LogWriter) Flush() {
	w.mu.Lock()
	defer w.mu.Unlock()
	if w.buffer.Len() == 0 {
		return
	}
	line := w.buffer.String()
	w.buffer.Reset()
	w.sendLine(line)
}

func (w *LogWriter) sendLine(line string) {
	line = strings.TrimRight(line, "\r")
	if line == "" {
		return
	}
	if w.maxLine > 0 && len(line) > w.maxLine {
		line = line[:w.maxLine] + "..."
	}
	select {
	case w.lines <- line:
	default:
	}
}
