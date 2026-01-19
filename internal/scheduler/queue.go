package scheduler

import "sync"

// ReadyQueue manages units ready for dispatch
type ReadyQueue struct {
	// queue of ready unit IDs (FIFO within same priority)
	queue []string

	// set for O(1) membership checks
	set map[string]bool

	mu sync.Mutex
}

// NewReadyQueue creates an empty ready queue
func NewReadyQueue() *ReadyQueue {
	return &ReadyQueue{
		queue: []string{},
		set:   make(map[string]bool),
	}
}

// Push adds a unit ID to the ready queue
// No-op if unit is already in queue
func (q *ReadyQueue) Push(unitID string) {
	q.mu.Lock()
	defer q.mu.Unlock()

	// Check if already in queue
	if q.set[unitID] {
		return
	}

	// Add to queue and set
	q.queue = append(q.queue, unitID)
	q.set[unitID] = true
}

// Pop removes and returns the next ready unit ID
// Returns empty string if queue is empty
func (q *ReadyQueue) Pop() string {
	q.mu.Lock()
	defer q.mu.Unlock()

	if len(q.queue) == 0 {
		return ""
	}

	// Get first item
	unitID := q.queue[0]
	q.queue = q.queue[1:]
	delete(q.set, unitID)

	return unitID
}

// Peek returns the next ready unit ID without removing it
// Returns empty string if queue is empty
func (q *ReadyQueue) Peek() string {
	q.mu.Lock()
	defer q.mu.Unlock()

	if len(q.queue) == 0 {
		return ""
	}

	return q.queue[0]
}

// Contains checks if a unit ID is in the queue
func (q *ReadyQueue) Contains(unitID string) bool {
	q.mu.Lock()
	defer q.mu.Unlock()

	return q.set[unitID]
}

// Len returns the number of units in the queue
func (q *ReadyQueue) Len() int {
	q.mu.Lock()
	defer q.mu.Unlock()

	return len(q.queue)
}

// Remove removes a specific unit from the queue
// Returns true if unit was found and removed
func (q *ReadyQueue) Remove(unitID string) bool {
	q.mu.Lock()
	defer q.mu.Unlock()

	// Check if unit is in set
	if !q.set[unitID] {
		return false
	}

	// Find and remove from queue
	for i, id := range q.queue {
		if id == unitID {
			q.queue = append(q.queue[:i], q.queue[i+1:]...)
			delete(q.set, unitID)
			return true
		}
	}

	return false
}

// List returns a copy of all unit IDs currently in queue
func (q *ReadyQueue) List() []string {
	q.mu.Lock()
	defer q.mu.Unlock()

	// Return a copy to prevent external modification
	result := make([]string, len(q.queue))
	copy(result, q.queue)
	return result
}
