package daemon

import (
	"bufio"
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"log"
	"sync"

	"github.com/RevCBH/choo/internal/container"
	"github.com/RevCBH/choo/internal/events"
)

// LogStreamer reads container logs and parses JSON events.
type LogStreamer struct {
	containerID string
	manager     container.Manager
	eventBus    *events.Bus

	mu     sync.Mutex
	cancel context.CancelFunc
	done   chan struct{}
}

// NewLogStreamer creates a log streamer for a container.
func NewLogStreamer(containerID string, manager container.Manager, eventBus *events.Bus) *LogStreamer {
	return &LogStreamer{
		containerID: containerID,
		manager:     manager,
		eventBus:    eventBus,
		done:        make(chan struct{}),
	}
}

// Start begins streaming and parsing logs.
// It blocks until the context is cancelled or the log stream ends.
func (s *LogStreamer) Start(ctx context.Context) error {
	var cancel context.CancelFunc
	ctx, cancel = context.WithCancel(ctx)

	s.mu.Lock()
	s.cancel = cancel
	s.mu.Unlock()

	defer close(s.done)

	reader, err := s.manager.Logs(ctx, container.ContainerID(s.containerID))
	if err != nil {
		return fmt.Errorf("failed to get container logs: %w", err)
	}
	defer reader.Close()

	scanner := bufio.NewScanner(reader)
	for scanner.Scan() {
		select {
		case <-ctx.Done():
			return ctx.Err()
		default:
		}

		line := scanner.Bytes()
		if err := s.parseLine(line); err != nil {
			log.Printf("Failed to parse log line: %v", err)
		}
	}

	return scanner.Err()
}

// Stop halts log streaming.
func (s *LogStreamer) Stop() {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.cancel != nil {
		s.cancel()
	}
}

// Done returns a channel that closes when streaming completes.
func (s *LogStreamer) Done() <-chan struct{} {
	return s.done
}

// parseLine attempts to parse a log line as a JSON event.
func (s *LogStreamer) parseLine(line []byte) error {
	if len(bytes.TrimSpace(line)) == 0 {
		return nil
	}

	var jsonEvt events.JSONEvent
	if err := json.Unmarshal(line, &jsonEvt); err != nil {
		return nil
	}

	if jsonEvt.Type == "" {
		return nil
	}

	evt, err := s.convertJSONEvent(jsonEvt)
	if err != nil {
		return fmt.Errorf("failed to convert event: %w", err)
	}

	s.eventBus.Emit(evt)
	return nil
}

// convertJSONEvent converts a wire-format JSON event to an internal event.
func (s *LogStreamer) convertJSONEvent(jsonEvt events.JSONEvent) (events.Event, error) {
	evt := events.Event{
		Time:    jsonEvt.Timestamp,
		Type:    events.EventType(jsonEvt.Type),
		Unit:    jsonEvt.Unit,
		Task:    jsonEvt.Task,
		PR:      jsonEvt.PR,
		Error:   jsonEvt.Error,
		Payload: jsonEvt.Payload,
	}

	if jsonEvt.Payload != nil {
		if evt.Unit == "" {
			if unit, ok := jsonEvt.Payload["unit"].(string); ok {
				evt.Unit = unit
			}
		}
		if evt.Task == nil {
			if taskVal, ok := jsonEvt.Payload["task"]; ok {
				switch task := taskVal.(type) {
				case float64:
					taskInt := int(task)
					evt.Task = &taskInt
				case int:
					taskInt := task
					evt.Task = &taskInt
				case int64:
					taskInt := int(task)
					evt.Task = &taskInt
				case json.Number:
					if num, err := task.Int64(); err == nil {
						taskInt := int(num)
						evt.Task = &taskInt
					}
				}
			}
		}
	}

	return evt, nil
}
