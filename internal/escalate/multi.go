package escalate

import (
	"context"
	"sync"
)

// Multi wraps multiple escalators and fans out to all of them
type Multi struct {
	escalators []Escalator
}

// NewMulti creates a Multi escalator that sends to all provided backends
func NewMulti(escalators ...Escalator) *Multi {
	return &Multi{escalators: escalators}
}

// Escalate sends the escalation to all backends concurrently.
// Returns the first error encountered, but continues sending to all backends.
func (m *Multi) Escalate(ctx context.Context, e Escalation) error {
	if len(m.escalators) == 0 {
		return nil
	}

	var (
		wg       sync.WaitGroup
		mu       sync.Mutex
		firstErr error
	)

	for _, esc := range m.escalators {
		wg.Add(1)
		go func(esc Escalator) {
			defer wg.Done()
			if err := esc.Escalate(ctx, e); err != nil {
				mu.Lock()
				if firstErr == nil {
					firstErr = err
				}
				mu.Unlock()
			}
		}(esc)
	}

	wg.Wait()
	return firstErr
}

// Name returns "multi"
func (m *Multi) Name() string {
	return "multi"
}
