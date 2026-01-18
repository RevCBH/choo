package cli

import (
	"context"
	"os"
	"sync"
	"syscall"
	"testing"
	"time"
)

func TestSignalHandler_New(t *testing.T) {
	_, cancel := context.WithCancel(context.Background())
	defer cancel()

	handler := NewSignalHandler(cancel)

	if handler == nil {
		t.Fatal("NewSignalHandler(cancel) should not return nil")
	}

	if handler.cancel == nil {
		t.Error("SignalHandler.cancel should be set")
	}

	if handler.signals == nil {
		t.Error("SignalHandler.signals channel should be initialized")
	}

	if handler.shutdown == nil {
		t.Error("SignalHandler.shutdown channel should be initialized")
	}

	if handler.onShutdown == nil {
		t.Error("SignalHandler.onShutdown slice should be initialized")
	}
}

func TestSignalHandler_GracefulShutdown(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	handler := NewSignalHandler(cancel)

	callbackCalled := false
	contextCancelled := false

	handler.OnShutdown(func() {
		callbackCalled = true
	})

	handler.Start()

	// Check context cancellation in a separate goroutine
	go func() {
		<-ctx.Done()
		contextCancelled = true
	}()

	// Send SIGINT
	handler.signals <- syscall.SIGINT

	// Wait for shutdown to complete
	select {
	case <-handler.shutdown:
		// Shutdown completed
	case <-time.After(1 * time.Second):
		t.Fatal("Shutdown did not complete in time")
	}

	if !callbackCalled {
		t.Error("SIGINT should trigger callback execution")
	}

	// Give a moment for context cancellation to propagate
	time.Sleep(10 * time.Millisecond)

	if !contextCancelled {
		t.Error("SIGINT should trigger context cancellation")
	}
}

func TestSignalHandler_MultipleCallbacks(t *testing.T) {
	_, cancel := context.WithCancel(context.Background())
	defer cancel()

	handler := NewSignalHandler(cancel)

	var mu sync.Mutex
	callOrder := []int{}

	handler.OnShutdown(func() {
		mu.Lock()
		callOrder = append(callOrder, 1)
		mu.Unlock()
	})

	handler.OnShutdown(func() {
		mu.Lock()
		callOrder = append(callOrder, 2)
		mu.Unlock()
	})

	handler.OnShutdown(func() {
		mu.Lock()
		callOrder = append(callOrder, 3)
		mu.Unlock()
	})

	handler.Start()

	// Send SIGTERM
	handler.signals <- syscall.SIGTERM

	// Wait for shutdown to complete
	select {
	case <-handler.shutdown:
		// Shutdown completed
	case <-time.After(1 * time.Second):
		t.Fatal("Shutdown did not complete in time")
	}

	mu.Lock()
	defer mu.Unlock()

	if len(callOrder) != 3 {
		t.Errorf("Expected 3 callbacks to be called, got %d", len(callOrder))
	}

	// Verify callbacks were called in registration order
	for i, expected := range []int{1, 2, 3} {
		if i >= len(callOrder) {
			t.Errorf("Missing callback at index %d", i)
			continue
		}
		if callOrder[i] != expected {
			t.Errorf("Callback %d: expected %d, got %d", i, expected, callOrder[i])
		}
	}
}

func TestSignalHandler_Wait(t *testing.T) {
	_, cancel := context.WithCancel(context.Background())
	defer cancel()

	handler := NewSignalHandler(cancel)
	handler.Start()

	waitCompleted := false
	var wg sync.WaitGroup
	wg.Add(1)

	go func() {
		defer wg.Done()
		handler.Wait()
		waitCompleted = true
	}()

	// Give Wait a moment to start blocking
	time.Sleep(50 * time.Millisecond)

	if waitCompleted {
		t.Error("Wait should block until shutdown is triggered")
	}

	// Send signal to trigger shutdown
	handler.signals <- syscall.SIGINT

	// Wait for the goroutine to complete
	done := make(chan struct{})
	go func() {
		wg.Wait()
		close(done)
	}()

	select {
	case <-done:
		// Wait completed successfully
	case <-time.After(1 * time.Second):
		t.Fatal("Wait did not unblock after shutdown was triggered")
	}

	if !waitCompleted {
		t.Error("Wait should have completed after shutdown")
	}
}

func TestSignalHandler_ContextCancelled(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	handler := NewSignalHandler(cancel)

	handler.Start()

	// Send SIGINT
	handler.signals <- syscall.SIGINT

	// Wait for shutdown to complete
	select {
	case <-handler.shutdown:
		// Shutdown completed
	case <-time.After(1 * time.Second):
		t.Fatal("Shutdown did not complete in time")
	}

	// Verify context was cancelled
	select {
	case <-ctx.Done():
		// Context was cancelled
	case <-time.After(100 * time.Millisecond):
		t.Error("Context should be cancelled on signal")
	}

	if ctx.Err() != context.Canceled {
		t.Errorf("Expected context.Canceled error, got %v", ctx.Err())
	}
}

func TestSignalHandler_Stop(t *testing.T) {
	_, cancel := context.WithCancel(context.Background())
	defer cancel()

	handler := NewSignalHandler(cancel)
	handler.Start()

	// Stop should not panic
	handler.Stop()

	// Verify that sending a signal after Stop doesn't cause issues
	// This is more of a cleanup test
	handler.signals <- os.Interrupt

	// Give it a moment to ensure nothing bad happens
	time.Sleep(50 * time.Millisecond)
}
