package web

import (
	"sync"
	"testing"
	"time"
)

func TestHub_NewHub(t *testing.T) {
	hub := NewHub()

	if hub == nil {
		t.Fatal("NewHub returned nil")
	}

	if hub.clients == nil {
		t.Error("clients map not initialized")
	}

	if len(hub.clients) != 0 {
		t.Errorf("clients map should be empty, got %d clients", len(hub.clients))
	}

	if hub.register == nil {
		t.Error("register channel not initialized")
	}

	if hub.unregister == nil {
		t.Error("unregister channel not initialized")
	}

	if hub.broadcast == nil {
		t.Error("broadcast channel not initialized")
	}

	if hub.done == nil {
		t.Error("done channel not initialized")
	}
}

func TestHub_ClientRegistration(t *testing.T) {
	hub := NewHub()
	go hub.Run()
	defer hub.Stop()

	client1 := NewClient("client1")
	hub.Register(client1)

	// Give the event loop time to process
	time.Sleep(10 * time.Millisecond)

	count := hub.Count()
	if count != 1 {
		t.Errorf("Count should be 1 after registration, got %d", count)
	}

	client2 := NewClient("client2")
	hub.Register(client2)

	time.Sleep(10 * time.Millisecond)

	count = hub.Count()
	if count != 2 {
		t.Errorf("Count should be 2 after second registration, got %d", count)
	}

	hub.Unregister(client1)

	time.Sleep(10 * time.Millisecond)

	count = hub.Count()
	if count != 1 {
		t.Errorf("Count should be 1 after unregister, got %d", count)
	}

	hub.Unregister(client2)

	time.Sleep(10 * time.Millisecond)

	count = hub.Count()
	if count != 0 {
		t.Errorf("Count should be 0 after all unregistered, got %d", count)
	}
}

func TestHub_Broadcast(t *testing.T) {
	hub := NewHub()
	go hub.Run()
	defer hub.Stop()

	client := NewClient("client1")
	hub.Register(client)

	time.Sleep(10 * time.Millisecond)

	event := &Event{Type: "test", Time: time.Now()}
	hub.Broadcast(event)

	select {
	case received := <-client.events:
		if received.Type != "test" {
			t.Errorf("Expected event type 'test', got '%s'", received.Type)
		}
	case <-time.After(100 * time.Millisecond):
		t.Error("Client did not receive event")
	}
}

func TestHub_BroadcastMultipleClients(t *testing.T) {
	hub := NewHub()
	go hub.Run()
	defer hub.Stop()

	client1 := NewClient("client1")
	client2 := NewClient("client2")
	client3 := NewClient("client3")

	hub.Register(client1)
	hub.Register(client2)
	hub.Register(client3)

	time.Sleep(10 * time.Millisecond)

	event := &Event{Type: "broadcast_test", Time: time.Now()}
	hub.Broadcast(event)

	clients := []*Client{client1, client2, client3}
	for i, client := range clients {
		select {
		case received := <-client.events:
			if received.Type != "broadcast_test" {
				t.Errorf("Client %d: Expected event type 'broadcast_test', got '%s'", i+1, received.Type)
			}
		case <-time.After(100 * time.Millisecond):
			t.Errorf("Client %d did not receive event", i+1)
		}
	}
}

func TestHub_BroadcastDropsWhenFull(t *testing.T) {
	hub := NewHub()
	go hub.Run()
	defer hub.Stop()

	client := NewClient("client1")
	hub.Register(client)

	time.Sleep(10 * time.Millisecond)

	// Fill the client's buffer (256 events)
	for i := 0; i < 256; i++ {
		event := &Event{Type: "filler", Time: time.Now()}
		hub.Broadcast(event)
	}

	time.Sleep(10 * time.Millisecond)

	// Try to send one more event - this should be dropped
	event := &Event{Type: "dropped", Time: time.Now()}

	done := make(chan bool)
	go func() {
		hub.Broadcast(event)
		done <- true
	}()

	select {
	case <-done:
		// Broadcast returned without blocking - good
	case <-time.After(100 * time.Millisecond):
		t.Error("Broadcast blocked when client buffer was full")
	}

	// Verify buffer is still full with original events
	select {
	case received := <-client.events:
		if received.Type != "filler" {
			t.Errorf("Expected first event to be 'filler', got '%s'", received.Type)
		}
	default:
		t.Error("Client buffer should still have events")
	}
}

func TestHub_UnregisterClosesChannel(t *testing.T) {
	hub := NewHub()
	go hub.Run()
	defer hub.Stop()

	client := NewClient("client1")
	hub.Register(client)

	time.Sleep(10 * time.Millisecond)

	hub.Unregister(client)

	time.Sleep(10 * time.Millisecond)

	// Check if channel is closed by reading from it
	_, ok := <-client.events
	if ok {
		t.Error("Client events channel should be closed after unregister")
	}
}

func TestHub_Stop(t *testing.T) {
	hub := NewHub()

	done := make(chan bool)
	go func() {
		hub.Run()
		done <- true
	}()

	client1 := NewClient("client1")
	client2 := NewClient("client2")

	hub.Register(client1)
	hub.Register(client2)

	time.Sleep(10 * time.Millisecond)

	count := hub.Count()
	if count != 2 {
		t.Errorf("Expected 2 clients before stop, got %d", count)
	}

	hub.Stop()

	select {
	case <-done:
		// Run loop exited - good
	case <-time.After(100 * time.Millisecond):
		t.Error("Run loop did not exit after Stop")
	}

	// Verify all clients are disconnected
	count = hub.Count()
	if count != 0 {
		t.Errorf("Expected 0 clients after stop, got %d", count)
	}

	// Verify client channels are closed
	_, ok := <-client1.events
	if ok {
		t.Error("Client1 events channel should be closed after stop")
	}

	_, ok = <-client2.events
	if ok {
		t.Error("Client2 events channel should be closed after stop")
	}
}

func TestHub_ConcurrentOperations(t *testing.T) {
	hub := NewHub()
	go hub.Run()
	defer hub.Stop()

	var wg sync.WaitGroup
	numGoroutines := 10

	// Concurrent registrations
	for i := 0; i < numGoroutines; i++ {
		wg.Add(1)
		go func(id int) {
			defer wg.Done()
			client := NewClient(string(rune(id)))
			hub.Register(client)
		}(i)
	}

	wg.Wait()
	time.Sleep(20 * time.Millisecond)

	initialCount := hub.Count()
	if initialCount != numGoroutines {
		t.Errorf("Expected %d clients after concurrent registration, got %d", numGoroutines, initialCount)
	}

	// Concurrent broadcasts
	for i := 0; i < numGoroutines; i++ {
		wg.Add(1)
		go func(id int) {
			defer wg.Done()
			event := &Event{Type: "concurrent", Time: time.Now()}
			hub.Broadcast(event)
		}(i)
	}

	wg.Wait()

	// Concurrent Count calls
	for i := 0; i < numGoroutines; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			hub.Count()
		}()
	}

	wg.Wait()
}
