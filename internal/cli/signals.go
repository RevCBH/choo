package cli

import (
	"context"
	"log"
	"os"
	"os/signal"
	"sync"
	"syscall"
)

// SignalHandler manages graceful shutdown on interrupt
type SignalHandler struct {
	signals    chan os.Signal
	shutdown   chan struct{}
	cancel     context.CancelFunc
	onShutdown []func()
	mu         sync.Mutex
}

// NewSignalHandler creates a signal handler with the given context cancel
func NewSignalHandler(cancel context.CancelFunc) *SignalHandler {
	return &SignalHandler{
		signals:    make(chan os.Signal, 1),
		shutdown:   make(chan struct{}),
		cancel:     cancel,
		onShutdown: make([]func(), 0),
	}
}

// Start begins listening for signals
func (h *SignalHandler) Start() {
	signal.Notify(h.signals, syscall.SIGINT, syscall.SIGTERM)

	go func() {
		sig := <-h.signals
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
	}()
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
}
