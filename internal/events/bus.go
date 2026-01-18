package events

import (
	"log"
	"sync"
	"time"
)

// Handler is a function that processes events
type Handler func(Event)

// Bus is the central event dispatcher
type Bus struct {
	// handlers is the list of registered event handlers
	handlers []Handler

	// ch is the buffered channel for event delivery
	ch chan Event

	// done signals the dispatch loop to exit
	done chan struct{}

	// closed tracks if Close has been called
	closed bool

	// mu protects handler registration and closed flag
	mu sync.RWMutex
}

// NewBus creates a new event bus with the specified buffer size
// Starts the dispatch goroutine automatically
func NewBus(bufferSize int) *Bus {
	if bufferSize <= 0 {
		bufferSize = 1000
	}

	b := &Bus{
		ch:   make(chan Event, bufferSize),
		done: make(chan struct{}),
	}

	go b.loop()

	return b
}

// Subscribe registers a handler to receive all events
// Handlers are called in registration order
// Safe to call from multiple goroutines
func (b *Bus) Subscribe(h Handler) {
	b.mu.Lock()
	defer b.mu.Unlock()
	b.handlers = append(b.handlers, h)
}

// Emit publishes an event to all handlers
// Sets event.Time to current time
// Non-blocking: drops event if buffer is full (logs warning)
// Safe to call from multiple goroutines
func (b *Bus) Emit(e Event) {
	e.Time = time.Now()
	select {
	case b.ch <- e:
		// Delivered
	default:
		// Buffer full, drop event
		log.Printf("WARN: event buffer full, dropping %s", e.Type)
	}
}

// Close stops the dispatch loop and releases resources
// Blocks until all pending events are processed
// Safe to call multiple times (subsequent calls are no-op)
func (b *Bus) Close() {
	b.mu.Lock()
	if b.closed {
		b.mu.Unlock()
		return
	}
	b.closed = true
	b.mu.Unlock()

	close(b.done)
}

// Len returns the current number of pending events in the buffer
func (b *Bus) Len() int {
	return len(b.ch)
}

// loop runs in a dedicated goroutine and processes events sequentially
func (b *Bus) loop() {
	for {
		select {
		case e := <-b.ch:
			b.dispatch(e)
		case <-b.done:
			// Drain remaining events before exiting
			for {
				select {
				case e := <-b.ch:
					b.dispatch(e)
				default:
					return
				}
			}
		}
	}
}

// dispatch calls all handlers with the event (recovers from panics)
func (b *Bus) dispatch(e Event) {
	b.mu.RLock()
	handlers := b.handlers
	b.mu.RUnlock()

	for _, h := range handlers {
		// Recover from handler panic
		func() {
			defer func() {
				if r := recover(); r != nil {
					log.Printf("WARN: handler panicked: %v", r)
				}
			}()
			h(e)
		}()
	}
}
