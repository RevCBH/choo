package cli

import (
	"context"
	"log"
	"os"
	"os/signal"
	"sync"
	"syscall"
	"time"
)

// SignalHandler manages graceful shutdown on interrupt
type SignalHandler struct {
	signals    chan os.Signal
	shutdown   chan struct{}
	stopCh     chan struct{} // closed by Stop to signal goroutine to exit
	done       chan struct{} // closed when goroutine exits
	stopOnce   sync.Once
	cancel     context.CancelFunc
	onShutdown []func()
	mu         sync.Mutex
}

// NewSignalHandler creates a signal handler with the given context cancel
func NewSignalHandler(cancel context.CancelFunc) *SignalHandler {
	return &SignalHandler{
		signals:    make(chan os.Signal, 1),
		shutdown:   make(chan struct{}),
		stopCh:     make(chan struct{}),
		done:       make(chan struct{}),
		cancel:     cancel,
		onShutdown: make([]func(), 0),
	}
}

// Start begins listening for signals
func (h *SignalHandler) Start() {
	h.StartWithNotify(true)
}

// StartWithNotify begins listening for signals, optionally registering with OS signal handling.
// Pass false for notify in unit tests to avoid global signal state interactions.
func (h *SignalHandler) StartWithNotify(notify bool) {
	if notify {
		signal.Notify(h.signals, syscall.SIGINT, syscall.SIGTERM)
	}

	started := make(chan struct{})
	go func() {
		defer close(h.done)
		close(started) // Signal that goroutine has started

		select {
		case sig := <-h.signals:
			log.Printf("Received signal: %v", sig)

			// Cancel context
			if h.cancel != nil {
				h.cancel()
			}

			// Execute callbacks in registration order
			h.mu.Lock()
			callbacks := make([]func(), len(h.onShutdown))
			copy(callbacks, h.onShutdown)
			h.mu.Unlock()

			for _, fn := range callbacks {
				fn()
			}

			// Close shutdown channel
			close(h.shutdown)
		case <-h.stopCh:
			// Stop was called, exit without doing anything
			return
		}
	}()

	// Wait for goroutine to start
	<-started
}

// OnShutdown registers a callback to run on shutdown
func (h *SignalHandler) OnShutdown(fn func()) {
	h.mu.Lock()
	defer h.mu.Unlock()
	h.onShutdown = append(h.onShutdown, fn)
}

// Wait blocks until shutdown is triggered
func (h *SignalHandler) Wait() {
	<-h.shutdown
}

// Stop stops the signal handler and cleans up
func (h *SignalHandler) Stop() {
	signal.Stop(h.signals)
	h.stopOnce.Do(func() {
		close(h.stopCh)
	})
	// Wait for goroutine to exit with a short timeout
	// This prevents blocking if the goroutine is in the middle of shutdown
	select {
	case <-h.done:
		// Goroutine exited cleanly
	case <-time.After(100 * time.Millisecond):
		// Timeout - goroutine may still be processing, but we've done our cleanup
	}
}
