package events

// Event represents a system event
type Event struct {
	Type string
	Data interface{}
}

// Bus provides event distribution across components
type Bus struct {
	Capacity int
	events   chan Event
}

// NewBus creates a new event bus with the specified capacity
func NewBus(capacity int) *Bus {
	return &Bus{
		Capacity: capacity,
		events:   make(chan Event, capacity),
	}
}

// Close shuts down the event bus
func (b *Bus) Close() error {
	close(b.events)
	return nil
}
