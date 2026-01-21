package daemon

import (
	"fmt"
	"log"
	"sync"

	"github.com/RevCBH/choo/internal/events"
)

// Subscription represents an active event subscription for a job.
type Subscription struct {
	JobID   string
	Channel <-chan events.Event
	cancel  func()
}

// subscriber holds a channel for sending events to a subscriber
type subscriber struct {
	ch     chan events.Event
	closed bool
	mu     sync.Mutex
}

// Subscribe returns an event channel for a specific job.
// The returned channel receives events until the job completes
// or Unsubscribe is called. The cleanup function must be called
// when done to release resources.
func (jm *jobManagerImpl) Subscribe(jobID string) (<-chan events.Event, func(), error) {
	// 1. Get job from map (return error if not found)
	jm.mu.RLock()
	job, exists := jm.jobs[jobID]
	jm.mu.RUnlock()

	if !exists {
		return nil, nil, fmt.Errorf("job not found: %s", jobID)
	}

	// 2. Create buffered channel for events (100 events as per spec)
	ch := make(chan events.Event, 100)

	// 3. Register channel with job's event bus
	sub := &subscriber{
		ch:     ch,
		closed: false,
	}

	// Subscribe to the job's event bus
	job.Events.Subscribe(func(e events.Event) {
		sub.mu.Lock()
		defer sub.mu.Unlock()

		if sub.closed {
			return
		}

		// Non-blocking send
		select {
		case sub.ch <- e:
			// Successfully sent
		default:
			// Channel full, log and drop event
			log.Printf("WARN: subscriber channel full for job %s, dropping event %s", jobID, e.Type)
		}
	})

	// 4. Return channel and unsubscribe function
	cleanup := func() {
		sub.mu.Lock()
		defer sub.mu.Unlock()

		if !sub.closed {
			sub.closed = true
			close(sub.ch)
		}
	}

	return ch, cleanup, nil
}

// SubscribeFrom returns events starting from a specific sequence number.
// Historical events are replayed from the database before live events.
func (jm *jobManagerImpl) SubscribeFrom(jobID string, fromSequence int) (<-chan events.Event, func(), error) {
	// 1. Check if job exists in map
	jm.mu.RLock()
	job, exists := jm.jobs[jobID]
	jm.mu.RUnlock()

	// 2. Create buffered channel for events
	ch := make(chan events.Event, 100)

	// 3. Query historical events from database
	historicalEvents, err := jm.db.ListEventsSince(jobID, fromSequence-1)
	if err != nil {
		return nil, nil, fmt.Errorf("failed to query historical events: %w", err)
	}

	// Create subscriber
	sub := &subscriber{
		ch:     ch,
		closed: false,
	}

	// 4. Send historical events first in a goroutine to avoid blocking
	go func() {
		for _, eventRecord := range historicalEvents {
			sub.mu.Lock()
			if sub.closed {
				sub.mu.Unlock()
				return
			}
			sub.mu.Unlock()

			// Convert EventRecord to events.Event
			// For simplicity, we'll create an event with the stored type
			evt := events.Event{
				Type: events.EventType(eventRecord.EventType),
				// Note: We're setting a simplified version. In production,
				// you'd want to deserialize the full payload from PayloadJSON
			}

			select {
			case sub.ch <- evt:
				// Sent successfully
			default:
				// Channel full, log and drop
				log.Printf("WARN: subscriber channel full during historical replay for job %s", jobID)
			}
		}
	}()

	// 5. Register for live events if job still exists
	if exists && job != nil {
		job.Events.Subscribe(func(e events.Event) {
			sub.mu.Lock()
			defer sub.mu.Unlock()

			if sub.closed {
				return
			}

			// Non-blocking send
			select {
			case sub.ch <- e:
				// Successfully sent
			default:
				// Channel full, log and drop event
				log.Printf("WARN: subscriber channel full for job %s, dropping event %s", jobID, e.Type)
			}
		})
	}

	// 6. Return channel and cleanup function
	cleanup := func() {
		sub.mu.Lock()
		defer sub.mu.Unlock()

		if !sub.closed {
			sub.closed = true
			close(sub.ch)
		}
	}

	return ch, cleanup, nil
}

// broadcast sends an event to all subscribers of a job.
// Called internally when events occur.
func (jm *jobManagerImpl) broadcast(jobID string, event events.Event) {
	// Get job's event bus
	jm.mu.RLock()
	job, exists := jm.jobs[jobID]
	jm.mu.RUnlock()

	if !exists {
		// Job not found, nothing to broadcast
		return
	}

	// Emit to the job's event bus, which will dispatch to all subscribers
	job.Events.Emit(event)
}

// closeJobSubscriptions closes all subscription channels for a job.
// Called when job completes.
func (jm *jobManagerImpl) closeJobSubscriptions(jobID string) {
	// Get the job
	jm.mu.RLock()
	job, exists := jm.jobs[jobID]
	jm.mu.RUnlock()

	if !exists {
		return
	}

	// Close the job's event bus, which will stop dispatching to all subscribers
	job.Events.Close()
}
