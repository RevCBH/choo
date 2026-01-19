package scheduler

import (
	"sync"
	"testing"
)

func TestReadyQueue_FIFO(t *testing.T) {
	q := NewReadyQueue()

	q.Push("a")
	q.Push("b")
	q.Push("c")

	if got := q.Pop(); got != "a" {
		t.Errorf("first Pop() = %q, want %q", got, "a")
	}
	if got := q.Pop(); got != "b" {
		t.Errorf("second Pop() = %q, want %q", got, "b")
	}
	if got := q.Pop(); got != "c" {
		t.Errorf("third Pop() = %q, want %q", got, "c")
	}
}

func TestReadyQueue_PopEmpty(t *testing.T) {
	q := NewReadyQueue()

	if got := q.Pop(); got != "" {
		t.Errorf("Pop() on empty queue = %q, want empty string", got)
	}
}

func TestReadyQueue_Peek(t *testing.T) {
	q := NewReadyQueue()

	q.Push("a")
	q.Push("b")

	// Peek should return first item without removing it
	if got := q.Peek(); got != "a" {
		t.Errorf("Peek() = %q, want %q", got, "a")
	}

	// Length should still be 2
	if got := q.Len(); got != 2 {
		t.Errorf("Len() after Peek() = %d, want 2", got)
	}

	// Pop should still return "a"
	if got := q.Pop(); got != "a" {
		t.Errorf("Pop() after Peek() = %q, want %q", got, "a")
	}
}

func TestReadyQueue_PeekEmpty(t *testing.T) {
	q := NewReadyQueue()

	if got := q.Peek(); got != "" {
		t.Errorf("Peek() on empty queue = %q, want empty string", got)
	}
}

func TestReadyQueue_Contains(t *testing.T) {
	q := NewReadyQueue()

	q.Push("a")
	q.Push("b")

	if !q.Contains("a") {
		t.Error("Contains(a) = false, want true")
	}
	if !q.Contains("b") {
		t.Error("Contains(b) = false, want true")
	}
	if q.Contains("c") {
		t.Error("Contains(c) = true, want false")
	}

	// After popping, should not contain
	q.Pop()
	if q.Contains("a") {
		t.Error("Contains(a) after Pop() = true, want false")
	}
}

func TestReadyQueue_Len(t *testing.T) {
	q := NewReadyQueue()

	if got := q.Len(); got != 0 {
		t.Errorf("initial Len() = %d, want 0", got)
	}

	q.Push("a")
	if got := q.Len(); got != 1 {
		t.Errorf("Len() after one Push() = %d, want 1", got)
	}

	q.Push("b")
	q.Push("c")
	if got := q.Len(); got != 3 {
		t.Errorf("Len() after three Push() = %d, want 3", got)
	}

	q.Pop()
	if got := q.Len(); got != 2 {
		t.Errorf("Len() after Pop() = %d, want 2", got)
	}
}

func TestReadyQueue_Remove(t *testing.T) {
	q := NewReadyQueue()

	q.Push("a")
	q.Push("b")
	q.Push("c")

	// Remove from middle
	if !q.Remove("b") {
		t.Error("Remove(b) = false, want true")
	}

	// Should not contain b anymore
	if q.Contains("b") {
		t.Error("Contains(b) after Remove() = true, want false")
	}

	// Length should be 2
	if got := q.Len(); got != 2 {
		t.Errorf("Len() after Remove() = %d, want 2", got)
	}

	// Pop should return a, then c (skipping removed b)
	if got := q.Pop(); got != "a" {
		t.Errorf("Pop() after Remove(b) = %q, want %q", got, "a")
	}
	if got := q.Pop(); got != "c" {
		t.Errorf("second Pop() after Remove(b) = %q, want %q", got, "c")
	}
}

func TestReadyQueue_RemoveNotFound(t *testing.T) {
	q := NewReadyQueue()

	q.Push("a")
	q.Push("b")

	// Try to remove item not in queue
	if q.Remove("c") {
		t.Error("Remove(c) = true, want false for non-existent item")
	}

	// Try to remove from empty queue
	q2 := NewReadyQueue()
	if q2.Remove("a") {
		t.Error("Remove(a) on empty queue = true, want false")
	}
}

func TestReadyQueue_PushDuplicate(t *testing.T) {
	q := NewReadyQueue()

	q.Push("a")
	q.Push("b")
	q.Push("a") // duplicate

	// Length should still be 2
	if got := q.Len(); got != 2 {
		t.Errorf("Len() after duplicate Push() = %d, want 2", got)
	}

	// Should only get one "a" when popping
	if got := q.Pop(); got != "a" {
		t.Errorf("first Pop() = %q, want %q", got, "a")
	}
	if got := q.Pop(); got != "b" {
		t.Errorf("second Pop() = %q, want %q", got, "b")
	}
	if got := q.Pop(); got != "" {
		t.Errorf("third Pop() = %q, want empty string", got)
	}
}

func TestReadyQueue_List(t *testing.T) {
	q := NewReadyQueue()

	q.Push("a")
	q.Push("b")
	q.Push("c")

	list := q.List()

	// Check contents
	want := []string{"a", "b", "c"}
	if len(list) != len(want) {
		t.Fatalf("List() length = %d, want %d", len(list), len(want))
	}

	for i, id := range want {
		if list[i] != id {
			t.Errorf("List()[%d] = %q, want %q", i, list[i], id)
		}
	}

	// Verify it's a copy - modifying list shouldn't affect queue
	list[0] = "modified"
	if q.Peek() != "a" {
		t.Error("modifying List() result affected queue")
	}
}

func TestReadyQueue_Concurrent(t *testing.T) {
	q := NewReadyQueue()
	var wg sync.WaitGroup

	// Number of goroutines
	numPushers := 50
	numPoppers := 50
	itemsPerPusher := 10

	// Push from multiple goroutines
	for i := 0; i < numPushers; i++ {
		wg.Add(1)
		go func(id int) {
			defer wg.Done()
			for j := 0; j < itemsPerPusher; j++ {
				q.Push(string(rune('a' + (id*itemsPerPusher+j)%26)))
			}
		}(i)
	}

	// Pop from multiple goroutines
	popped := make(chan string, numPushers*itemsPerPusher)
	for i := 0; i < numPoppers; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for {
				item := q.Pop()
				if item == "" {
					return
				}
				popped <- item
			}
		}()
	}

	// Wait for all pushes to complete
	wg.Wait()

	// Close channel and collect results
	close(popped)
	var count int
	for range popped {
		count++
	}

	// Should have popped some items (exact count depends on timing and duplicates)
	if count == 0 {
		t.Error("no items were popped in concurrent test")
	}

	// Queue should be empty or nearly empty
	if q.Len() > numPushers {
		t.Errorf("queue has %d items remaining, expected most to be popped", q.Len())
	}
}
