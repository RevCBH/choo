package provider

import (
	"bufio"
	"encoding/json"
	"fmt"
	"io"
	"path/filepath"
	"regexp"
	"strings"
	"sync"
	"time"

	"golang.org/x/term"
)

// StreamEvent represents a parsed event from Claude's JSON stream.
type StreamEvent struct {
	Type      string          `json:"type"`
	Message   *MessageEvent   `json:"message,omitempty"`
	Index     int             `json:"index,omitempty"`
	Delta     *DeltaEvent     `json:"delta,omitempty"`
	Subtype   string          `json:"subtype,omitempty"`
	Error     *ErrorEvent     `json:"error,omitempty"`
	ToolUse   *ToolUseEvent   `json:"tool_use,omitempty"`
	Result    *ResultEvent    `json:"result,omitempty"`
	ToolUseID string          `json:"tool_use_id,omitempty"`
	Content   json.RawMessage `json:"content,omitempty"`
	Timestamp time.Time       `json:"-"`
}

// MessageEvent contains message-level information.
type MessageEvent struct {
	ID      string         `json:"id"`
	Role    string         `json:"role"`
	Model   string         `json:"model"`
	Content []ContentBlock `json:"content"`
}

// ContentBlock represents a content block in a message.
type ContentBlock struct {
	Type  string          `json:"type"`
	Text  string          `json:"text,omitempty"`
	ID    string          `json:"id,omitempty"`
	Name  string          `json:"name,omitempty"`
	Input json.RawMessage `json:"input,omitempty"`
}

// DeltaEvent contains incremental updates.
type DeltaEvent struct {
	Type        string `json:"type"`
	Text        string `json:"text,omitempty"`
	PartialJSON string `json:"partial_json,omitempty"`
}

// ErrorEvent contains error information.
type ErrorEvent struct {
	Type    string `json:"type"`
	Message string `json:"message"`
}

// ToolUseEvent contains information about a tool being used.
type ToolUseEvent struct {
	ID    string          `json:"id"`
	Name  string          `json:"name"`
	Input json.RawMessage `json:"input,omitempty"`
}

// ResultEvent contains the result of a tool use or operation.
type ResultEvent struct {
	Subtype string `json:"subtype"`
	Success bool   `json:"success"`
}

type toolKind int

const (
	toolOther toolKind = iota
	toolWrite
	toolEdit
)

type toolMeta struct {
	name     string
	kind     toolKind
	path     string  // spec path for direct spec writes
	specPath string  // parent spec path for task writes
	taskName string  // task file name for task writes
	printed  bool
}

// StreamOptions configures StreamHandler behavior.
type StreamOptions struct {
	Output         io.Writer
	Verbose        bool
	ShowAssistant  bool
	UseTUI         bool
	EnableProgress bool
	SpecsDir       string
	RepoRoot       string
	PhaseTitle     string
	CounterLabel   string
	PRDPath        string
	InitialItems   []string
	Total          int
}

// StreamHandler processes streaming events from Claude.
type StreamHandler struct {
	output        io.Writer
	verbose       bool
	showAssistant bool
	showToolUse   bool
	useTUI        bool

	progressEnabled bool
	progress        *SpecProgress
	progressLines   int
	logLines        int
	plainHeader     bool
	plainPrinted    map[string]ItemStatus
	counterLabel    string

	specsDir   string
	repoRoot   string
	phaseTitle string
	prdPath    string

	currentToolName  string
	currentToolID    string
	currentToolInput strings.Builder

	toolMeta map[string]*toolMeta
	textBuf  strings.Builder

	messageCount int
	toolCount    int

	renderMu sync.Mutex
}

var specPathRegex = regexp.MustCompile(`(?i)([\w./-]+\.md)`) // best-effort

// NewStreamHandler creates a new stream handler.
func NewStreamHandler(opts StreamOptions) *StreamHandler {
	showToolUse := opts.Verbose || !opts.EnableProgress
	h := &StreamHandler{
		output:          opts.Output,
		verbose:         opts.Verbose,
		showAssistant:   opts.ShowAssistant,
		showToolUse:     showToolUse,
		useTUI:          opts.UseTUI,
		progressEnabled: opts.EnableProgress,
		specsDir:        opts.SpecsDir,
		repoRoot:        opts.RepoRoot,
		phaseTitle:      opts.PhaseTitle,
		prdPath:         opts.PRDPath,
		counterLabel:    opts.CounterLabel,
		plainPrinted:    make(map[string]ItemStatus),
		toolMeta:        make(map[string]*toolMeta),
	}

	if h.progressEnabled {
		h.progress = NewSpecProgress(ProgressOptions{
			PhaseTitle:   opts.PhaseTitle,
			PRDPath:      opts.PRDPath,
			SpecsDir:     opts.SpecsDir,
			CounterLabel: opts.CounterLabel,
			Total:        opts.Total,
			InitialItems: opts.InitialItems,
		})
		if h.useTUI {
			h.progress.StartSpinner(h.renderProgress)
			h.renderProgress()
		} else {
			h.printPlainHeader()
		}
	}

	return h
}

// ProcessStream reads JSON lines from the reader and handles events.
func (h *StreamHandler) ProcessStream(reader io.Reader) error {
	defer h.finish()

	scanner := bufio.NewScanner(reader)
	// Increase buffer size for large JSON lines
	buf := make([]byte, 0, 64*1024)
	scanner.Buffer(buf, 1024*1024)

	for scanner.Scan() {
		line := scanner.Text()
		if line == "" {
			continue
		}

		var event StreamEvent
		if err := json.Unmarshal([]byte(line), &event); err != nil {
			// Not valid JSON, might be regular output
			if h.showAssistant {
				h.writeLog(line + "\n")
			}
			continue
		}

		event.Timestamp = time.Now()
		h.handleEvent(&event)
	}

	return scanner.Err()
}

// handleEvent processes a single stream event.
func (h *StreamHandler) handleEvent(event *StreamEvent) {
	switch event.Type {
	case "message_start":
		h.messageCount++

	case "content_block_start":
		if event.Content != nil {
			var block ContentBlock
			if err := json.Unmarshal(event.Content, &block); err == nil {
				h.handleContentBlockStart(&block)
			}
		}

	case "content_block_delta":
		if event.Delta != nil {
			h.handleDelta(event.Delta)
		}

	case "content_block_stop":
		h.handleContentBlockStop()

	case "message_delta":
		// Message-level updates (stop_reason, usage)

	case "message_stop":
		// End of message

	case "error":
		if event.Error != nil {
			h.handleError(event.Error)
		}

	// Handle tool-specific events (from --print-stream)
	case "tool_use":
		if event.ToolUse != nil {
			h.handleToolUse(event.ToolUse)
		}

	case "tool_result":
		h.handleToolResult(event)

	case "assistant":
		// Assistant response chunk
		if event.Message != nil {
			for _, block := range event.Message.Content {
				if block.Type == "text" && block.Text != "" {
					h.handleText(block.Text)
				}
			}
		}
	}
}

// handleContentBlockStart processes the start of a content block.
func (h *StreamHandler) handleContentBlockStart(block *ContentBlock) {
	switch block.Type {
	case "tool_use":
		h.currentToolName = block.Name
		h.currentToolID = block.ID
		h.currentToolInput.Reset()

		if len(block.Input) > 0 {
			h.currentToolInput.Write(block.Input)
			h.handleToolStart(block.ID, block.Name, block.Input)
		} else {
			h.handleToolStart(block.ID, block.Name, nil)
		}
		h.printToolLineOnce(block.ID, block.Name, block.Input)

	case "text":
		// Text block starting
	}
}

// handleContentBlockStop finalizes the current tool use if any.
func (h *StreamHandler) handleContentBlockStop() {
	if h.currentToolName == "" {
		return
	}

	if h.currentToolInput.Len() > 0 {
		h.handleToolStart(h.currentToolID, h.currentToolName, json.RawMessage(h.currentToolInput.String()))
	}

	h.completeTool(h.currentToolID, true, "")

	h.currentToolName = ""
	h.currentToolID = ""
	h.currentToolInput.Reset()
}

// handleDelta processes incremental content updates.
func (h *StreamHandler) handleDelta(delta *DeltaEvent) {
	switch delta.Type {
	case "text_delta":
		if delta.Text != "" {
			h.handleText(delta.Text)
		}

	case "input_json_delta":
		if h.currentToolName != "" {
			payload := delta.PartialJSON
			if payload == "" {
				payload = delta.Text
			}
			h.currentToolInput.WriteString(payload)
		}
	}
}

// handleToolUse processes a tool use event.
func (h *StreamHandler) handleToolUse(tool *ToolUseEvent) {
	h.handleToolStart(tool.ID, tool.Name, tool.Input)
	h.printToolLineOnce(tool.ID, tool.Name, tool.Input)
}

// handleToolResult processes tool completion events.
func (h *StreamHandler) handleToolResult(event *StreamEvent) {
	id := event.ToolUseID
	if id == "" && event.ToolUse != nil {
		id = event.ToolUse.ID
	}
	if id == "" {
		id = h.currentToolID
	}
	success := true
	if event.Result != nil {
		success = event.Result.Success
	}
	h.completeTool(id, success, "")
}

func (h *StreamHandler) handleError(errEvent *ErrorEvent) {
	msg := errEvent.Message
	if h.currentToolID != "" {
		h.completeTool(h.currentToolID, false, msg)
	}
	if h.showToolUse {
		h.writeLog(fmt.Sprintf("⚠ Error: %s\n", msg))
	}
}

func (h *StreamHandler) handleText(text string) {
	if h.showAssistant {
		h.writeLog(text)
	}
	h.textBuf.WriteString(text)
	for {
		data := h.textBuf.String()
		idx := strings.IndexByte(data, '\n')
		if idx == -1 {
			break
		}
		line := strings.TrimSpace(data[:idx])
		h.processTextLine(line)
		h.textBuf.Reset()
		h.textBuf.WriteString(data[idx+1:])
	}
}

func (h *StreamHandler) processTextLine(line string) {
	if line == "" || h.progress == nil {
		return
	}

	matches := specPathRegex.FindAllString(line, -1)
	if len(matches) == 0 {
		return
	}

	for _, match := range matches {
		path, ok := normalizeSpecPath(match, h.specsDir, h.repoRoot)
		if !ok {
			continue
		}

		status, msg := statusFromLine(line, path)
		if status == StatusPending {
			continue
		}
		if h.progress.UpdateItem(path, status, msg) {
			h.onProgressUpdate()
		}
	}
}

func statusFromLine(line, path string) (ItemStatus, string) {
	lower := strings.ToLower(line)
	message := extractMessage(line, path)

	switch {
	case strings.Contains(line, "✗") || strings.Contains(lower, "error") || strings.Contains(lower, "failed"):
		return StatusError, message
	case strings.Contains(line, "⚠") || strings.Contains(lower, "warning"):
		return StatusWarning, message
	case strings.Contains(line, "✓") || strings.Contains(lower, "fixed") || strings.Contains(lower, "ok"):
		return StatusDone, message
	default:
		return StatusPending, ""
	}
}

func extractMessage(line, path string) string {
	idx := strings.Index(line, path)
	if idx == -1 {
		return ""
	}
	msg := strings.TrimSpace(line[idx+len(path):])
	msg = strings.TrimLeft(msg, " -:\t")
	return strings.TrimSpace(msg)
}

func (h *StreamHandler) handleToolStart(id, name string, input json.RawMessage) {
	if id == "" {
		id = fmt.Sprintf("tool-%d", h.toolCount+1)
	}
	meta, exists := h.toolMeta[id]
	if !exists {
		meta = &toolMeta{name: name, kind: classifyTool(name)}
		h.toolMeta[id] = meta
		h.toolCount++
	}
	if name != "" {
		meta.name = name
		meta.kind = classifyTool(name)
	}
	if len(input) > 0 {
		path := extractPathFromToolInput(input)
		if path != "" {
			// First check if it's a task file (e.g., specs/tasks/auth/01-setup.md)
			if specPath, taskName, ok := parseTaskPath(path, h.specsDir, h.repoRoot); ok {
				meta.taskName = taskName
				if meta.kind == toolWrite || meta.kind == toolEdit {
					if h.progress != nil {
						// Find canonical spec path (case-insensitive match)
						// Task dirs are lowercase but spec files may be uppercase
						canonicalPath, found := h.progress.FindSpecPath(specPath)
						if found {
							meta.specPath = canonicalPath
							// Mark the parent spec as working and set current task
							h.progress.UpdateItem(canonicalPath, StatusWorking, "")
							if h.progress.SetCurrentTask(canonicalPath, taskName) {
								h.onProgressUpdate()
							}
						}
					}
				}
			} else if normalized, ok := normalizeSpecPath(path, h.specsDir, h.repoRoot); ok {
				// It's a direct spec file write
				meta.path = normalized
				if meta.kind == toolWrite || meta.kind == toolEdit {
					if h.progress != nil && h.progress.UpdateItem(normalized, StatusWorking, "") {
						h.onProgressUpdate()
					}
				}
			}
		}
	}
}

func (h *StreamHandler) completeTool(id string, success bool, message string) {
	if id == "" {
		return
	}
	meta, ok := h.toolMeta[id]
	if !ok {
		return
	}
	if h.progress == nil {
		return
	}
	if meta.kind != toolWrite && meta.kind != toolEdit {
		return
	}

	// Handle task file completion (clear current task but don't mark spec as done)
	if meta.specPath != "" {
		if h.progress.ClearCurrentTask(meta.specPath) {
			h.onProgressUpdate()
		}
		return
	}

	// Handle direct spec file completion
	if meta.path == "" {
		return
	}
	status := StatusDone
	msg := ""
	if !success {
		status = StatusError
		msg = message
	}
	if h.progress.UpdateItem(meta.path, status, msg) {
		h.onProgressUpdate()
	}
}

func classifyTool(name string) toolKind {
	switch strings.ToLower(name) {
	case "write", "write_file", "writefile", "create", "create_file":
		return toolWrite
	case "edit", "edit_file", "editfile", "update":
		return toolEdit
	default:
		return toolOther
	}
}

func extractPathFromToolInput(input json.RawMessage) string {
	if len(input) == 0 {
		return ""
	}
	var data map[string]any
	if err := json.Unmarshal(input, &data); err != nil {
		return ""
	}
	keys := []string{"file_path", "path", "filepath", "filename", "target"}
	for _, key := range keys {
		if val, ok := data[key]; ok {
			if str, ok := val.(string); ok {
				return str
			}
		}
	}
	return ""
}

// parseTaskPath checks if a path is a task file and extracts the spec path and task name.
// Task paths look like: specs/tasks/AUTH/01-setup.md
// Returns (specPath, taskName, ok) where specPath is "specs/AUTH.md".
func parseTaskPath(path, specsDir, repoRoot string) (specPath, taskName string, ok bool) {
	if path == "" || specsDir == "" {
		return "", "", false
	}

	cleaned := filepath.Clean(path)
	if filepath.IsAbs(cleaned) {
		if repoRoot == "" {
			return "", "", false
		}
		rel, err := filepath.Rel(repoRoot, cleaned)
		if err != nil {
			return "", "", false
		}
		cleaned = rel
	}
	cleaned = filepath.ToSlash(cleaned)
	specsDir = filepath.ToSlash(specsDir)

	// Check if it's a task path: <specsDir>/tasks/<SPEC-NAME>/<task>.md
	tasksPrefix := specsDir + "/tasks/"
	if !strings.HasPrefix(cleaned, tasksPrefix) {
		return "", "", false
	}
	if !strings.HasSuffix(cleaned, ".md") {
		return "", "", false
	}

	// Extract spec name and task name from path
	// cleaned = "specs/tasks/AUTH/01-setup.md"
	// remainder = "AUTH/01-setup.md"
	remainder := cleaned[len(tasksPrefix):]
	parts := strings.SplitN(remainder, "/", 2)
	if len(parts) != 2 {
		return "", "", false
	}

	specName := parts[0]        // "AUTH"
	taskFileName := parts[1]    // "01-setup.md"

	// Construct the spec path: specs/AUTH.md
	specPath = specsDir + "/" + specName + ".md"

	return specPath, taskFileName, true
}

func normalizeSpecPath(path, specsDir, repoRoot string) (string, bool) {
	if path == "" || specsDir == "" {
		return "", false
	}
	cleaned := filepath.Clean(path)
	if filepath.IsAbs(cleaned) {
		if repoRoot == "" {
			return "", false
		}
		rel, err := filepath.Rel(repoRoot, cleaned)
		if err != nil {
			return "", false
		}
		cleaned = rel
	}
	cleaned = filepath.ToSlash(cleaned)
	specsDir = filepath.ToSlash(specsDir)
	if strings.HasPrefix(cleaned, "../") || cleaned == ".." {
		return "", false
	}
	if !strings.HasSuffix(cleaned, ".md") {
		return "", false
	}
	if strings.Contains(cleaned, "/tasks/") {
		return "", false
	}
	if strings.HasPrefix(cleaned, specsDir+"/") {
		return cleaned, true
	}
	if !strings.Contains(cleaned, "/") {
		candidate := filepath.ToSlash(filepath.Join(specsDir, cleaned))
		if !strings.Contains(candidate, "/tasks/") {
			return candidate, true
		}
	}
	return "", false
}

func (h *StreamHandler) onProgressUpdate() {
	if h.progress == nil {
		return
	}
	if h.useTUI {
		h.renderProgress()
	} else {
		h.emitPlainUpdates()
	}
}

func (h *StreamHandler) finish() {
	if h.textBuf.Len() > 0 {
		line := strings.TrimSpace(h.textBuf.String())
		if line != "" {
			h.processTextLine(line)
		}
		h.textBuf.Reset()
	}
	if h.progress == nil {
		return
	}
	if h.progress.MarkAllDone() {
		h.onProgressUpdate()
	}
	if h.useTUI {
		h.progress.StopSpinner()
		h.renderProgress()
		h.writeLog("\n")
		return
	}
	// Non-TTY summary
	items, _ := h.progress.Snapshot()
	done := 0
	for _, item := range items {
		switch item.Status {
		case StatusDone, StatusWarning, StatusError:
			done++
		}
	}
	if h.counterLabel == "" {
		h.counterLabel = "items"
	}
	fmt.Fprintf(h.output, "Done: %d %s\n", done, h.counterLabel)
}

func (h *StreamHandler) emitPlainUpdates() {
	if h.progress == nil {
		return
	}
	items, _ := h.progress.Snapshot()
	for _, item := range items {
		if item.Status != StatusDone && item.Status != StatusWarning && item.Status != StatusError {
			continue
		}
		last, ok := h.plainPrinted[item.Path]
		if ok && last == item.Status && item.Message == "" {
			continue
		}
		h.plainPrinted[item.Path] = item.Status
		icon := plainStatusIcon(item.Status)
		if item.Message != "" && (item.Status == StatusWarning || item.Status == StatusError) {
			fmt.Fprintf(h.output, "  %s %s - %s\n", item.Path, icon, item.Message)
			continue
		}
		fmt.Fprintf(h.output, "  %s %s\n", item.Path, icon)
	}
}

func plainStatusIcon(status ItemStatus) string {
	switch status {
	case StatusDone:
		return "✓"
	case StatusWarning:
		return "⚠"
	case StatusError:
		return "✗"
	default:
		return "·"
	}
}

func (h *StreamHandler) printPlainHeader() {
	if h.plainHeader {
		return
	}
	h.plainHeader = true
	if h.phaseTitle != "" {
		fmt.Fprintln(h.output, h.phaseTitle)
	}
}

func (h *StreamHandler) renderProgress() {
	if !h.progressEnabled || h.progress == nil || !h.useTUI {
		return
	}

	h.renderMu.Lock()
	defer h.renderMu.Unlock()

	lines := h.progress.RenderLines(h.terminalWidth())
	if h.progressLines > 0 {
		moveUp := h.progressLines + h.logLines
		if moveUp > 0 {
			fmt.Fprintf(h.output, "\033[%dF", moveUp)
		}
	}
	for i, line := range lines {
		fmt.Fprint(h.output, "\r\033[K")
		fmt.Fprint(h.output, line)
		if i < len(lines)-1 {
			fmt.Fprint(h.output, "\n")
		}
	}
	fmt.Fprint(h.output, "\n")
	if h.logLines > 0 {
		fmt.Fprintf(h.output, "\033[%dE", h.logLines)
	}
	h.progressLines = len(lines)
}

func (h *StreamHandler) terminalWidth() int {
	if !h.useTUI {
		return 0
	}
	if f, ok := h.output.(interface{ Fd() uintptr }); ok {
		if w, _, err := term.GetSize(int(f.Fd())); err == nil {
			return w
		}
	}
	return 0
}

func (h *StreamHandler) writeLog(s string) {
	if s == "" {
		return
	}
	fmt.Fprint(h.output, s)
	if h.progressEnabled && h.useTUI {
		h.logLines += countLines(s)
	}
}

func countLines(s string) int {
	if s == "" {
		return 0
	}
	count := strings.Count(s, "\n")
	if !strings.HasSuffix(s, "\n") {
		count++
	}
	return count
}

// Stats returns summary statistics.
func (h *StreamHandler) Stats() (messages, tools int) {
	return h.messageCount, h.toolCount
}

// toolIcon returns an appropriate icon for a tool.
func toolIcon(name string) string {
	switch name {
	case "Read", "read":
		return "📖"
	case "Write", "write":
		return "✏️"
	case "Edit", "edit":
		return "📝"
	case "Bash", "bash":
		return "💻"
	case "Glob", "glob":
		return "🔍"
	case "Grep", "grep":
		return "🔎"
	case "Task", "task":
		return "🤖"
	case "TodoWrite":
		return "📋"
	case "WebFetch":
		return "🌐"
	default:
		return "🔧"
	}
}

// formatToolName formats a tool name for display.
func formatToolName(name string) string {
	return name
}

func (h *StreamHandler) printToolLineOnce(id, name string, input json.RawMessage) {
	if !h.showToolUse {
		return
	}
	if id != "" {
		if meta, ok := h.toolMeta[id]; ok {
			if meta.printed {
				return
			}
			meta.printed = true
		} else {
			h.toolMeta[id] = &toolMeta{name: name, kind: classifyTool(name), printed: true}
		}
	}
	h.printToolLine(name, input)
}

func (h *StreamHandler) printToolLine(name string, input json.RawMessage) {
	icon := toolIcon(name)
	line := fmt.Sprintf("%s %s", icon, formatToolName(name))
	if len(input) > 0 {
		summary := summarizeToolInput(name, input)
		if summary != "" {
			line += ": " + summary
		}
	}
	h.writeLog(line + "\n")
}

// summarizeToolInput extracts key information from tool input for display.
func summarizeToolInput(toolName string, input json.RawMessage) string {
	if len(input) == 0 {
		return ""
	}

	var data map[string]any
	if err := json.Unmarshal(input, &data); err != nil {
		return ""
	}

	switch toolName {
	case "Read", "read":
		if path, ok := data["file_path"].(string); ok {
			return truncatePath(path)
		}
	case "Write", "write":
		if path, ok := data["file_path"].(string); ok {
			return truncatePath(path)
		}
	case "Edit", "edit":
		if path, ok := data["file_path"].(string); ok {
			return truncatePath(path)
		}
	case "Bash", "bash":
		if cmd, ok := data["command"].(string); ok {
			return truncateString(cmd, 60)
		}
	case "Glob", "glob":
		if pattern, ok := data["pattern"].(string); ok {
			return pattern
		}
	case "Grep", "grep":
		if pattern, ok := data["pattern"].(string); ok {
			return truncateString(pattern, 40)
		}
	case "Task", "task":
		if desc, ok := data["description"].(string); ok {
			return truncateString(desc, 50)
		}
	}

	return ""
}

// truncatePath shortens a file path for display.
func truncatePath(path string) string {
	// Show just the last 2-3 path components
	parts := strings.Split(path, "/")
	if len(parts) <= 3 {
		return path
	}
	return ".../" + strings.Join(parts[len(parts)-3:], "/")
}

// truncateString truncates a string to maxLen with ellipsis.
func truncateString(s string, maxLen int) string {
	// Remove newlines
	s = strings.ReplaceAll(s, "\n", " ")
	s = strings.TrimSpace(s)

	if len(s) <= maxLen {
		return s
	}
	return s[:maxLen-3] + "..."
}
