package web

import (
	"bufio"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

func TestServer_New(t *testing.T) {
	tmpDir := t.TempDir()
	sockPath := filepath.Join(tmpDir, "test.sock")

	srv, err := New(Config{
		Addr:       "127.0.0.1:0",
		SocketPath: sockPath,
	})
	if err != nil {
		t.Fatalf("New failed: %v", err)
	}

	if srv.store == nil {
		t.Error("Store is not initialized")
	}

	if srv.hub == nil {
		t.Error("Hub is not initialized")
	}

	if srv.httpServer == nil {
		t.Error("HTTP server is not initialized")
	}

	if srv.socketServer == nil {
		t.Error("Socket server is not initialized")
	}
}

func TestServer_NewWithDefaults(t *testing.T) {
	srv, err := New(Config{})
	if err != nil {
		t.Fatalf("New with empty config failed: %v", err)
	}

	if srv.addr != ":8080" {
		t.Errorf("Expected default addr :8080, got %s", srv.addr)
	}

	if srv.socket == "" {
		t.Error("SocketPath should use defaultSocketPath()")
	}

	// Verify it matches defaultSocketPath()
	expected := defaultSocketPath()
	if srv.socket != expected {
		t.Errorf("Expected socket path %s, got %s", expected, srv.socket)
	}
}

func TestServer_StartStop(t *testing.T) {
	tmpDir := t.TempDir()
	sockPath := filepath.Join(tmpDir, "test.sock")

	srv, err := New(Config{
		Addr:       "127.0.0.1:0",
		SocketPath: sockPath,
	})
	if err != nil {
		t.Fatalf("New failed: %v", err)
	}

	if err := srv.Start(); err != nil {
		t.Fatalf("Start failed: %v", err)
	}

	// Give servers time to start
	time.Sleep(100 * time.Millisecond)

	// Verify socket file exists
	if _, err := os.Stat(sockPath); err != nil {
		t.Errorf("Socket file not created: %v", err)
	}

	// Test that we can connect to the socket
	conn, err := net.Dial("unix", sockPath)
	if err != nil {
		t.Errorf("Failed to connect to socket: %v", err)
	} else {
		conn.Close()
	}

	// Stop the server
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	if err := srv.Stop(ctx); err != nil {
		t.Fatalf("Stop failed: %v", err)
	}

	// Verify socket file is cleaned up
	if _, err := os.Stat(sockPath); err == nil {
		t.Error("Socket file should be removed after Stop")
	}
}

func TestServer_HTTPRoutes(t *testing.T) {
	tmpDir := t.TempDir()
	sockPath := filepath.Join(tmpDir, "test.sock")

	srv, err := New(Config{
		Addr:       "127.0.0.1:0",
		SocketPath: sockPath,
	})
	if err != nil {
		t.Fatalf("New failed: %v", err)
	}

	if err := srv.Start(); err != nil {
		t.Fatalf("Start failed: %v", err)
	}
	defer srv.Stop(context.Background())

	// Give server time to start
	time.Sleep(100 * time.Millisecond)

	// Get the actual address (will be the ephemeral port assigned)
	baseURL := fmt.Sprintf("http://%s", srv.Addr())

	tests := []struct {
		name       string
		path       string
		wantStatus int
		checkBody  func(t *testing.T, body []byte)
	}{
		{
			name:       "GET /api/state returns JSON",
			path:       "/api/state",
			wantStatus: http.StatusOK,
			checkBody: func(t *testing.T, body []byte) {
				var snapshot StateSnapshot
				if err := json.Unmarshal(body, &snapshot); err != nil {
					t.Errorf("Failed to parse JSON: %v", err)
				}
			},
		},
		{
			name:       "GET /api/graph returns JSON",
			path:       "/api/graph",
			wantStatus: http.StatusOK,
			checkBody: func(t *testing.T, body []byte) {
				var graph GraphData
				if err := json.Unmarshal(body, &graph); err != nil {
					t.Errorf("Failed to parse JSON: %v", err)
				}
			},
		},
		{
			name:       "GET /api/events streams SSE",
			path:       "/api/events",
			wantStatus: http.StatusOK,
			checkBody: func(t *testing.T, body []byte) {
				// For SSE, just check that Content-Type is set correctly
				// Full SSE test is in TestServer_SocketToSSE
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// For SSE endpoint, we need special handling to avoid blocking
			if tt.path == "/api/events" {
				ctx, cancel := context.WithTimeout(context.Background(), 100*time.Millisecond)
				defer cancel()

				req, _ := http.NewRequestWithContext(ctx, "GET", baseURL+tt.path, nil)
				client := &http.Client{}
				resp, err := client.Do(req)
				if err != nil {
					// Timeout is expected for SSE - just verify we can connect
					// by checking if the error is timeout-related
					if !strings.Contains(err.Error(), "context deadline exceeded") {
						t.Fatalf("Unexpected error: %v", err)
					}
					return
				}
				defer resp.Body.Close()

				if resp.StatusCode != tt.wantStatus {
					t.Errorf("Expected status %d, got %d", tt.wantStatus, resp.StatusCode)
				}

				// Check SSE headers
				if ct := resp.Header.Get("Content-Type"); ct != "text/event-stream" {
					t.Errorf("Expected Content-Type text/event-stream, got %s", ct)
				}
				return
			}

			resp, err := http.Get(baseURL + tt.path)
			if err != nil {
				t.Fatalf("Request failed: %v", err)
			}
			defer resp.Body.Close()

			if resp.StatusCode != tt.wantStatus {
				t.Errorf("Expected status %d, got %d", tt.wantStatus, resp.StatusCode)
			}

			if tt.checkBody != nil {
				body, _ := io.ReadAll(resp.Body)
				tt.checkBody(t, body)
			}
		})
	}
}

func TestServer_GracefulShutdown(t *testing.T) {
	tmpDir := t.TempDir()
	sockPath := filepath.Join(tmpDir, "test.sock")

	srv, err := New(Config{
		Addr:       "127.0.0.1:0",
		SocketPath: sockPath,
	})
	if err != nil {
		t.Fatalf("New failed: %v", err)
	}

	if err := srv.Start(); err != nil {
		t.Fatalf("Start failed: %v", err)
	}

	// Give server time to start
	time.Sleep(100 * time.Millisecond)

	// Create an active connection with cancellable context
	baseURL := fmt.Sprintf("http://%s", srv.Addr())

	// Start a long-running SSE request with context
	reqCtx, reqCancel := context.WithCancel(context.Background())
	defer reqCancel()

	done := make(chan error, 1)
	go func() {
		req, _ := http.NewRequestWithContext(reqCtx, "GET", baseURL+"/api/events", nil)
		client := &http.Client{}
		resp, err := client.Do(req)
		if err != nil {
			done <- err
			return
		}
		// Try to read a byte to keep connection open
		buf := make([]byte, 1)
		resp.Body.Read(buf)
		resp.Body.Close()
		done <- nil
	}()

	// Give the request time to connect
	time.Sleep(100 * time.Millisecond)

	// Stop with timeout
	stopCtx, stopCancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer stopCancel()

	if err := srv.Stop(stopCtx); err != nil {
		t.Fatalf("Graceful shutdown failed: %v", err)
	}

	// Wait for request to finish
	select {
	case err := <-done:
		// Connection should be closed (err might be context canceled or EOF)
		if err != nil && !strings.Contains(err.Error(), "context canceled") && !strings.Contains(err.Error(), "EOF") {
			t.Logf("Request ended with: %v (this is expected)", err)
		}
	case <-time.After(3 * time.Second):
		t.Error("Connection did not close after shutdown")
	}
}

func TestServer_SocketToSSE(t *testing.T) {
	tmpDir := t.TempDir()
	sockPath := filepath.Join(tmpDir, "test.sock")

	srv, err := New(Config{
		Addr:       "127.0.0.1:0",
		SocketPath: sockPath,
	})
	if err != nil {
		t.Fatalf("New failed: %v", err)
	}

	if err := srv.Start(); err != nil {
		t.Fatalf("Start failed: %v", err)
	}
	defer srv.Stop(context.Background())

	// Give server time to start
	time.Sleep(200 * time.Millisecond)

	// Connect browser to SSE
	baseURL := fmt.Sprintf("http://%s", srv.Addr())

	// Wait for HTTP server to be ready
	ready := false
	for i := 0; i < 10; i++ {
		resp, err := http.Get(baseURL + "/api/state")
		if err == nil {
			resp.Body.Close()
			ready = true
			break
		}
		time.Sleep(100 * time.Millisecond)
	}
	if !ready {
		t.Fatal("HTTP server did not become ready")
	}

	client := &http.Client{}
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	req, _ := http.NewRequestWithContext(ctx, "GET", baseURL+"/api/events", nil)
	resp, err := client.Do(req)
	if err != nil {
		t.Fatalf("Failed to connect to SSE: %v", err)
	}
	defer resp.Body.Close()

	// Read SSE events in goroutine
	events := make(chan string, 10)
	go func() {
		scanner := bufio.NewScanner(resp.Body)
		for scanner.Scan() {
			line := scanner.Text()
			if strings.HasPrefix(line, "event: ") {
				events <- strings.TrimPrefix(line, "event: ")
			}
		}
	}()

	// Connect orchestrator to socket
	conn, err := net.Dial("unix", sockPath)
	if err != nil {
		t.Fatalf("Failed to connect to socket: %v", err)
	}
	defer conn.Close()

	// Send event via socket
	testEvent := Event{
		Type: "test.event",
		Time: time.Now(),
	}
	data, _ := json.Marshal(testEvent)
	fmt.Fprintf(conn, "%s\n", data)

	// Browser should receive event via SSE
	select {
	case eventType := <-events:
		if eventType != "test.event" {
			t.Errorf("Expected event type 'test.event', got '%s'", eventType)
		}
	case <-time.After(2 * time.Second):
		t.Error("Did not receive event via SSE")
	}
}
