package web

import (
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io/fs"
	"net/http"
)

// IndexHandler serves the embedded HTML UI.
// Serves index.html for "/" and static files for other paths.
func IndexHandler(staticFS fs.FS) http.Handler {
	subFS, _ := fs.Sub(staticFS, "static")
	return http.FileServer(http.FS(subFS))
}

// StateHandler returns the current state snapshot as JSON.
// GET /api/state
func StateHandler(store *Store) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		snapshot := store.Snapshot()
		json.NewEncoder(w).Encode(snapshot)
	}
}

// GraphHandler returns the dependency graph as JSON.
// GET /api/graph
// Returns empty graph if orchestrator not connected.
func GraphHandler(store *Store) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		graph := store.Graph()
		if graph == nil {
			graph = &GraphData{
				Nodes:  []GraphNode{},
				Edges:  []GraphEdge{},
				Levels: [][]string{},
			}
		}
		json.NewEncoder(w).Encode(graph)
	}
}

// EventsHandler provides the SSE event stream.
// GET /api/events
// Sets appropriate headers and streams events to browser.
func EventsHandler(hub *Hub) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		flusher, ok := w.(http.Flusher)
		if !ok {
			http.Error(w, "SSE not supported", http.StatusInternalServerError)
			return
		}

		w.Header().Set("Content-Type", "text/event-stream")
		w.Header().Set("Cache-Control", "no-cache")
		w.Header().Set("Connection", "keep-alive")
		w.Header().Set("Access-Control-Allow-Origin", "*")

		// Send initial comment to establish connection
		fmt.Fprintf(w, ": connected\n\n")
		flusher.Flush()

		client := NewClient(generateID())
		hub.Register(client)
		defer hub.Unregister(client)

		ctx := r.Context()
		for {
			select {
			case <-ctx.Done():
				return
			case event, ok := <-client.events:
				if !ok {
					return
				}
				data, _ := json.Marshal(event)
				fmt.Fprintf(w, "event: %s\ndata: %s\n\n", event.Type, data)
				flusher.Flush()
			}
		}
	}
}

// generateID generates a random client ID.
func generateID() string {
	bytes := make([]byte, 8)
	if _, err := rand.Read(bytes); err != nil {
		// Fallback to a less random but still unique ID
		return hex.EncodeToString([]byte("fallback"))
	}
	return hex.EncodeToString(bytes)
}
