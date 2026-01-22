package daemon

import (
	"bytes"
	"context"
	"errors"
	"io"
	"testing"
	"time"

	"github.com/RevCBH/choo/internal/container"
	"github.com/RevCBH/choo/internal/events"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

type mockManager struct {
	logsReader io.ReadCloser
	logsFunc   func(ctx context.Context, id container.ContainerID) (io.ReadCloser, error)
}

var _ container.Manager = (*mockManager)(nil)

func (m *mockManager) Create(ctx context.Context, cfg container.ContainerConfig) (container.ContainerID, error) {
	return "", nil
}

func (m *mockManager) Start(ctx context.Context, id container.ContainerID) error {
	return nil
}

func (m *mockManager) Wait(ctx context.Context, id container.ContainerID) (int, error) {
	return 0, nil
}

func (m *mockManager) Logs(ctx context.Context, id container.ContainerID) (io.ReadCloser, error) {
	if m.logsFunc != nil {
		return m.logsFunc(ctx, id)
	}
	return m.logsReader, nil
}

func (m *mockManager) Stop(ctx context.Context, id container.ContainerID, timeout time.Duration) error {
	return nil
}

func (m *mockManager) Remove(ctx context.Context, id container.ContainerID) error {
	return nil
}

func TestLogStreamer_ParseJSONEvent(t *testing.T) {
	jsonLine := `{"type":"unit.started","timestamp":"2024-01-15T10:00:00Z","payload":{"unit":"test-unit"}}`
	reader := io.NopCloser(bytes.NewBufferString(jsonLine + "\n"))

	bus := events.NewBus(10)
	defer bus.Close()
	collector := events.NewEventCollector(bus)

	manager := &mockManager{logsReader: reader}
	streamer := NewLogStreamer("test-container", manager, bus)

	ctx, cancel := context.WithTimeout(context.Background(), time.Second)
	defer cancel()

	err := streamer.Start(ctx)
	require.NoError(t, err)

	bus.Wait()
	received := collector.Get()
	require.Len(t, received, 1)
	assert.Equal(t, events.EventType("unit.started"), received[0].Type)
	assert.Equal(t, "test-unit", received[0].Unit)
}

func TestLogStreamer_NonJSONIgnored(t *testing.T) {
	reader := io.NopCloser(bytes.NewBufferString("Starting orchestrator...\n"))

	bus := events.NewBus(10)
	defer bus.Close()
	collector := events.NewEventCollector(bus)

	manager := &mockManager{logsReader: reader}
	streamer := NewLogStreamer("test-container", manager, bus)

	ctx, cancel := context.WithTimeout(context.Background(), time.Second)
	defer cancel()

	err := streamer.Start(ctx)
	require.NoError(t, err)

	bus.Wait()
	assert.Len(t, collector.Get(), 0)
}

func TestLogStreamer_EmptyLineIgnored(t *testing.T) {
	reader := io.NopCloser(bytes.NewBufferString("\n\n   \n"))

	bus := events.NewBus(10)
	defer bus.Close()
	collector := events.NewEventCollector(bus)

	manager := &mockManager{logsReader: reader}
	streamer := NewLogStreamer("test-container", manager, bus)

	ctx, cancel := context.WithTimeout(context.Background(), time.Second)
	defer cancel()

	err := streamer.Start(ctx)
	require.NoError(t, err)

	bus.Wait()
	assert.Len(t, collector.Get(), 0)
}

func TestLogStreamer_MalformedJSONIgnored(t *testing.T) {
	reader := io.NopCloser(bytes.NewBufferString(`{"type":"broken` + "\n"))

	bus := events.NewBus(10)
	defer bus.Close()
	collector := events.NewEventCollector(bus)

	manager := &mockManager{logsReader: reader}
	streamer := NewLogStreamer("test-container", manager, bus)

	ctx, cancel := context.WithTimeout(context.Background(), time.Second)
	defer cancel()

	err := streamer.Start(ctx)
	require.NoError(t, err)

	bus.Wait()
	assert.Len(t, collector.Get(), 0)
}

func TestLogStreamer_ExtractsUnitAndTask(t *testing.T) {
	jsonLine := `{"type":"task.completed","timestamp":"2024-01-15T10:00:00Z","payload":{"unit":"test-unit","task":2}}`
	reader := io.NopCloser(bytes.NewBufferString(jsonLine + "\n"))

	bus := events.NewBus(10)
	defer bus.Close()
	collector := events.NewEventCollector(bus)

	manager := &mockManager{logsReader: reader}
	streamer := NewLogStreamer("test-container", manager, bus)

	ctx, cancel := context.WithTimeout(context.Background(), time.Second)
	defer cancel()

	err := streamer.Start(ctx)
	require.NoError(t, err)

	bus.Wait()
	received := collector.Get()
	require.Len(t, received, 1)
	assert.Equal(t, "test-unit", received[0].Unit)
	if assert.NotNil(t, received[0].Task) {
		assert.Equal(t, 2, *received[0].Task)
	}
}

func TestLogStreamer_EventParsingOverhead(t *testing.T) {
	bus := events.NewBus(2000)
	defer bus.Close()
	streamer := &LogStreamer{eventBus: bus}

	eventJSON := []byte(`{"type":"task.completed","timestamp":"2024-01-15T10:00:00Z","payload":{"task":1,"unit":"test"}}`)

	iterations := 1000
	start := time.Now()
	for i := 0; i < iterations; i++ {
		streamer.parseLine(eventJSON)
	}
	elapsed := time.Since(start)

	avgLatency := elapsed / time.Duration(iterations)
	if avgLatency > 10*time.Millisecond {
		t.Errorf("Event parsing overhead %v exceeds 10ms target", avgLatency)
	}
}

func TestLogStreamer_Stop(t *testing.T) {
	bus := events.NewBus(10)
	defer bus.Close()

	manager := &mockManager{
		logsFunc: func(ctx context.Context, id container.ContainerID) (io.ReadCloser, error) {
			pr, pw := io.Pipe()
			go func() {
				<-ctx.Done()
				_ = pw.Close()
			}()
			return pr, nil
		},
	}

	streamer := NewLogStreamer("test-container", manager, bus)
	ctx := context.Background()

	errCh := make(chan error, 1)
	go func() {
		errCh <- streamer.Start(ctx)
	}()

	time.Sleep(10 * time.Millisecond)
	streamer.Stop()

	select {
	case <-streamer.Done():
	case <-time.After(time.Second):
		t.Fatal("timeout waiting for streamer to stop")
	}

	select {
	case err := <-errCh:
		if err != nil && !errors.Is(err, context.Canceled) {
			t.Fatalf("unexpected error: %v", err)
		}
	case <-time.After(time.Second):
		t.Fatal("timeout waiting for start to return")
	}
}

func TestLogStreamer_Done(t *testing.T) {
	reader := io.NopCloser(bytes.NewBufferString(`{"type":"unit.started","timestamp":"2024-01-15T10:00:00Z"}` + "\n"))

	bus := events.NewBus(10)
	defer bus.Close()

	manager := &mockManager{logsReader: reader}
	streamer := NewLogStreamer("test-container", manager, bus)

	ctx, cancel := context.WithTimeout(context.Background(), time.Second)
	defer cancel()

	errCh := make(chan error, 1)
	go func() {
		errCh <- streamer.Start(ctx)
	}()

	select {
	case <-streamer.Done():
	case <-time.After(time.Second):
		t.Fatal("timeout waiting for Done channel")
	}

	err := <-errCh
	require.NoError(t, err)
}
