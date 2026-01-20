package web

import (
	"context"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"
)

func TestIndexHandler_ServesHTML(t *testing.T) {
	handler := IndexHandler(staticFS)
	req := httptest.NewRequest("GET", "/", nil)
	w := httptest.NewRecorder()

	handler.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("expected status 200, got %d", w.Code)
	}

	contentType := w.Header().Get("Content-Type")
	if !strings.Contains(contentType, "text/html") {
		t.Errorf("expected Content-Type to contain text/html, got %s", contentType)
	}

	body := w.Body.String()
	if !strings.Contains(body, "Choo Orchestrator") {
		t.Errorf("expected body to contain 'Choo Orchestrator', got %s", body)
	}
}

func TestIndexHandler_ServesStaticFiles(t *testing.T) {
	handler := IndexHandler(staticFS)

	tests := []struct {
		path        string
		contentType string
		contains    string
	}{
		{"/style.css", "text/css", "font-family"},
		{"/app.js", "javascript", "EventSource"},
	}

	for _, tt := range tests {
		t.Run(tt.path, func(t *testing.T) {
			req := httptest.NewRequest("GET", tt.path, nil)
			w := httptest.NewRecorder()

			handler.ServeHTTP(w, req)

			if w.Code != http.StatusOK {
				t.Errorf("expected status 200, got %d", w.Code)
			}

			contentType := w.Header().Get("Content-Type")
			if !strings.Contains(contentType, tt.contentType) {
				t.Errorf("expected Content-Type to contain %s, got %s", tt.contentType, contentType)
			}

			body := w.Body.String()
			if !strings.Contains(body, tt.contains) {
				t.Errorf("expected body to contain '%s', got %s", tt.contains, body)
			}
		})
	}
}

func TestStateHandler_ReturnsJSON(t *testing.T) {
	store := NewStore()
	handler := StateHandler(store)

	req := httptest.NewRequest("GET", "/api/state", nil)
	w := httptest.NewRecorder()

	handler.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("expected status 200, got %d", w.Code)
	}

	contentType := w.Header().Get("Content-Type")
	if !strings.Contains(contentType, "application/json") {
		t.Errorf("expected Content-Type to contain application/json, got %s", contentType)
	}

	var snapshot StateSnapshot
	if err := json.NewDecoder(w.Body).Decode(&snapshot); err != nil {
		t.Errorf("failed to decode JSON: %v", err)
	}
}

func TestStateHandler_WaitingState(t *testing.T) {
	store := NewStore()
	handler := StateHandler(store)

	req := httptest.NewRequest("GET", "/api/state", nil)
	w := httptest.NewRecorder()

	handler.ServeHTTP(w, req)

	var snapshot StateSnapshot
	json.NewDecoder(w.Body).Decode(&snapshot)

	if snapshot.Status != "waiting" {
		t.Errorf("expected status 'waiting', got %s", snapshot.Status)
	}

	if snapshot.Connected {
		t.Errorf("expected connected to be false, got true")
	}

	if len(snapshot.Units) != 0 {
		t.Errorf("expected empty units array, got %d units", len(snapshot.Units))
	}
}

func TestStateHandler_RunningState(t *testing.T) {
	store := NewStore()
	handler := StateHandler(store)

	// Simulate orch.started event
	event := &Event{
		Type: "orch.started",
		Time: time.Now(),
		Payload: []byte(`{
			"unit_count": 2,
			"parallelism": 1,
			"graph": {
				"nodes": [
					{"id": "unit1", "level": 0},
					{"id": "unit2", "level": 1}
				],
				"edges": [
					{"from": "unit2", "to": "unit1"}
				],
				"levels": [["unit1"], ["unit2"]]
			}
		}`),
	}
	store.HandleEvent(event)

	req := httptest.NewRequest("GET", "/api/state", nil)
	w := httptest.NewRecorder()

	handler.ServeHTTP(w, req)

	var snapshot StateSnapshot
	json.NewDecoder(w.Body).Decode(&snapshot)

	if snapshot.Status != "running" {
		t.Errorf("expected status 'running', got %s", snapshot.Status)
	}

	if len(snapshot.Units) != 2 {
		t.Errorf("expected 2 units, got %d", len(snapshot.Units))
	}
}

func TestGraphHandler_NoGraph(t *testing.T) {
	store := NewStore()
	handler := GraphHandler(store)

	req := httptest.NewRequest("GET", "/api/graph", nil)
	w := httptest.NewRecorder()

	handler.ServeHTTP(w, req)

	var graph GraphData
	json.NewDecoder(w.Body).Decode(&graph)

	if len(graph.Nodes) != 0 {
		t.Errorf("expected empty nodes, got %d", len(graph.Nodes))
	}

	if len(graph.Edges) != 0 {
		t.Errorf("expected empty edges, got %d", len(graph.Edges))
	}

	if len(graph.Levels) != 0 {
		t.Errorf("expected empty levels, got %d", len(graph.Levels))
	}
}

func TestGraphHandler_WithGraph(t *testing.T) {
	store := NewStore()
	handler := GraphHandler(store)

	// Simulate orch.started event with graph
	event := &Event{
		Type: "orch.started",
		Time: time.Now(),
		Payload: []byte(`{
			"unit_count": 2,
			"parallelism": 1,
			"graph": {
				"nodes": [
					{"id": "unit1", "level": 0},
					{"id": "unit2", "level": 1}
				],
				"edges": [
					{"from": "unit2", "to": "unit1"}
				],
				"levels": [["unit1"], ["unit2"]]
			}
		}`),
	}
	store.HandleEvent(event)

	req := httptest.NewRequest("GET", "/api/graph", nil)
	w := httptest.NewRecorder()

	handler.ServeHTTP(w, req)

	var graph GraphData
	json.NewDecoder(w.Body).Decode(&graph)

	if len(graph.Nodes) != 2 {
		t.Errorf("expected 2 nodes, got %d", len(graph.Nodes))
	}

	if len(graph.Edges) != 1 {
		t.Errorf("expected 1 edge, got %d", len(graph.Edges))
	}

	if len(graph.Levels) != 2 {
		t.Errorf("expected 2 levels, got %d", len(graph.Levels))
	}
}

func TestEventsHandler_SetsHeaders(t *testing.T) {
	hub := NewHub()
	go hub.Run()
	defer hub.Stop()

	handler := EventsHandler(hub)

	req := httptest.NewRequest("GET", "/api/events", nil)
	w := httptest.NewRecorder()

	// Use a context that cancels immediately to avoid blocking
	ctx, cancel := context.WithCancel(req.Context())
	cancel()
	req = req.WithContext(ctx)

	handler.ServeHTTP(w, req)

	contentType := w.Header().Get("Content-Type")
	if contentType != "text/event-stream" {
		t.Errorf("expected Content-Type 'text/event-stream', got %s", contentType)
	}

	cacheControl := w.Header().Get("Cache-Control")
	if cacheControl != "no-cache" {
		t.Errorf("expected Cache-Control 'no-cache', got %s", cacheControl)
	}

	connection := w.Header().Get("Connection")
	if connection != "keep-alive" {
		t.Errorf("expected Connection 'keep-alive', got %s", connection)
	}
}

func TestEventsHandler_StreamsEvents(t *testing.T) {
	hub := NewHub()
	go hub.Run()
	defer hub.Stop()

	handler := EventsHandler(hub)

	// Create a request with a pipe to read streamed data
	req := httptest.NewRequest("GET", "/api/events", nil)
	ctx, cancel := context.WithTimeout(req.Context(), 2*time.Second)
	defer cancel()
	req = req.WithContext(ctx)

	// Use a custom ResponseWriter that implements Flusher
	pr, pw := io.Pipe()
	defer pr.Close()

	w := &sseResponseWriter{
		header: make(http.Header),
		body:   pw,
	}

	// Start handler in goroutine
	done := make(chan struct{})
	go func() {
		handler.ServeHTTP(w, req)
		pw.Close()
		close(done)
	}()

	// Read connection comment first
	connBuf := make([]byte, 256)
	connDone := make(chan struct{})
	go func() {
		pr.Read(connBuf)
		close(connDone)
	}()

	select {
	case <-connDone:
		// Connection comment received
	case <-time.After(500 * time.Millisecond):
		t.Fatal("timeout waiting for connection comment")
	}

	// Start reader BEFORE broadcast (pipe is synchronous - writes block until read)
	readDone := make(chan struct{})
	var output string
	go func() {
		buf := make([]byte, 1024)
		n, err := pr.Read(buf)
		if err == nil {
			output = string(buf[:n])
		}
		close(readDone)
	}()

	// Give reader goroutine time to start and block on Read
	time.Sleep(10 * time.Millisecond)

	// Broadcast an event
	event := &Event{
		Type: "test.event",
		Time: time.Now(),
	}
	hub.Broadcast(event)

	select {
	case <-readDone:
		if !strings.Contains(output, "event: test.event") {
			t.Errorf("expected event type 'test.event', got %s", output)
		}

		if !strings.Contains(output, "data: ") {
			t.Errorf("expected data field, got %s", output)
		}
	case <-time.After(1 * time.Second):
		t.Fatal("timeout waiting for event")
	}

	cancel()
	<-done
}

func TestEventsHandler_SSEFormat(t *testing.T) {
	hub := NewHub()
	go hub.Run()
	defer hub.Stop()

	handler := EventsHandler(hub)

	// Create a request with a pipe to read streamed data
	req := httptest.NewRequest("GET", "/api/events", nil)
	ctx, cancel := context.WithTimeout(req.Context(), 2*time.Second)
	defer cancel()
	req = req.WithContext(ctx)

	pr, pw := io.Pipe()
	defer pr.Close()

	w := &sseResponseWriter{
		header: make(http.Header),
		body:   pw,
	}

	// Start handler in goroutine
	done := make(chan struct{})
	go func() {
		handler.ServeHTTP(w, req)
		pw.Close()
		close(done)
	}()

	// Read connection comment first
	connBuf := make([]byte, 256)
	connDone := make(chan struct{})
	go func() {
		pr.Read(connBuf)
		close(connDone)
	}()

	select {
	case <-connDone:
		// Connection comment received
	case <-time.After(500 * time.Millisecond):
		t.Fatal("timeout waiting for connection comment")
	}

	// Start reader BEFORE broadcast (pipe is synchronous - writes block until read)
	readDone := make(chan struct{})
	var output string
	go func() {
		buf := make([]byte, 1024)
		n, err := pr.Read(buf)
		if err == nil {
			output = string(buf[:n])
		}
		close(readDone)
	}()

	// Give reader goroutine time to start and block on Read
	time.Sleep(10 * time.Millisecond)

	// Broadcast an event
	event := &Event{
		Type: "unit.started",
		Time: time.Now(),
		Unit: "test-unit",
	}
	hub.Broadcast(event)

	select {
	case <-readDone:
		// Check SSE format: "event: type\ndata: json\n\n"
		lines := strings.Split(output, "\n")
		if len(lines) < 3 {
			t.Errorf("expected at least 3 lines in SSE format, got %d", len(lines))
		}

		if !strings.HasPrefix(lines[0], "event: ") {
			t.Errorf("expected first line to start with 'event: ', got %s", lines[0])
		}

		if !strings.HasPrefix(lines[1], "data: ") {
			t.Errorf("expected second line to start with 'data: ', got %s", lines[1])
		}

		if lines[2] != "" {
			t.Errorf("expected third line to be empty, got %s", lines[2])
		}
	case <-time.After(1 * time.Second):
		t.Fatal("timeout waiting for event")
	}

	cancel()
	<-done
}

// sseResponseWriter implements http.ResponseWriter and http.Flusher for testing SSE
type sseResponseWriter struct {
	header http.Header
	body   io.Writer
}

func (w *sseResponseWriter) Header() http.Header {
	return w.header
}

func (w *sseResponseWriter) Write(b []byte) (int, error) {
	return w.body.Write(b)
}

func (w *sseResponseWriter) WriteHeader(statusCode int) {
	// No-op for testing
}

func (w *sseResponseWriter) Flush() {
	// No-op for testing
}
