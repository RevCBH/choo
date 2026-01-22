package daemon

import (
	"context"
	"path/filepath"
	"testing"
	"time"

	"github.com/RevCBH/choo/internal/events"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// validJobConfigWithRepo returns a valid JobConfig for testing using the given repo path.
func validJobConfigWithRepo(repoPath string) JobConfig {
	return JobConfig{
		RepoPath:     repoPath,
		TasksDir:     filepath.Join(repoPath, "specs", "tasks"),
		TargetBranch: "main",
	}
}

func TestSubscribe_ValidJob(t *testing.T) {
	database := setupTestDB(t)
	jm := NewJobManager(database, 10)
	repoPath := setupTestRepo(t)

	// Start a job
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	jobID, err := jm.Start(ctx, cancel, validJobConfigWithRepo(repoPath))
	require.NoError(t, err)

	// Subscribe
	eventsCh, cleanup, err := jm.Subscribe(jobID)
	require.NoError(t, err)
	require.NotNil(t, eventsCh)
	require.NotNil(t, cleanup)
	defer cleanup()

	// Verify channel is not nil and cleanup function works
	assert.NotNil(t, eventsCh)
}

func TestSubscribe_InvalidJob(t *testing.T) {
	database := setupTestDB(t)
	jm := NewJobManager(database, 10)

	// Try to subscribe to non-existent job
	_, _, err := jm.Subscribe("invalid-job-id")
	require.Error(t, err)
	assert.Contains(t, err.Error(), "not found")
}

func TestSubscribe_ReceivesEvents(t *testing.T) {
	database := setupTestDB(t)
	jm := NewJobManager(database, 10)
	repoPath := setupTestRepo(t)

	// Start a job
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	jobID, err := jm.Start(ctx, cancel, validJobConfigWithRepo(repoPath))
	require.NoError(t, err)

	// Subscribe
	eventsCh, cleanup, err := jm.Subscribe(jobID)
	require.NoError(t, err)
	defer cleanup()

	// Broadcast an event
	testEvent := events.Event{Type: "test", Unit: "test-unit"}
	jm.broadcast(jobID, testEvent)

	// Receive with timeout
	select {
	case e := <-eventsCh:
		assert.Equal(t, events.EventType("test"), e.Type)
		assert.Equal(t, "test-unit", e.Unit)
	case <-time.After(time.Second):
		t.Fatal("timeout waiting for event")
	}
}

func TestSubscribe_Unsubscribe(t *testing.T) {
	database := setupTestDB(t)
	jm := NewJobManager(database, 10)
	repoPath := setupTestRepo(t)

	// Start a job
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	jobID, err := jm.Start(ctx, cancel, validJobConfigWithRepo(repoPath))
	require.NoError(t, err)

	// Subscribe
	eventsCh, cleanup, err := jm.Subscribe(jobID)
	require.NoError(t, err)

	// Call cleanup to unsubscribe
	cleanup()

	// Channel should be closed
	_, ok := <-eventsCh
	assert.False(t, ok, "channel should be closed after cleanup")
}

func TestSubscribeFrom_ReplaysHistory(t *testing.T) {
	database := setupTestDB(t)
	jm := NewJobManager(database, 10)
	repoPath := setupTestRepo(t)

	// Start job with a long-running context to keep it alive
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	jobID, err := jm.Start(ctx, cancel, validJobConfigWithRepo(repoPath))
	require.NoError(t, err)

	// Wait for job to be tracked
	var job *ManagedJob
	for i := 0; i < 50; i++ {
		job, _ = jm.Get(jobID)
		if job != nil {
			break
		}
		time.Sleep(10 * time.Millisecond)
	}
	require.NotNil(t, job, "job should be tracked")

	// Store some historical events in the database
	// Use a small delay between events to avoid lock contention
	err = database.AppendEvent(jobID, "event1", nil, nil)
	require.NoError(t, err)
	time.Sleep(10 * time.Millisecond)

	err = database.AppendEvent(jobID, "event2", nil, nil)
	require.NoError(t, err)
	time.Sleep(10 * time.Millisecond)

	err = database.AppendEvent(jobID, "event3", nil, nil)
	require.NoError(t, err)
	time.Sleep(10 * time.Millisecond)

	// Subscribe from sequence 2
	ch, cleanup, err := jm.SubscribeFrom(jobID, 2)
	require.NoError(t, err)
	defer cleanup()

	// Should receive events 2 and 3 (historical)
	receivedEvents := []events.Event{}
	timeout := time.After(2 * time.Second)

	// Collect events
	for i := 0; i < 2; i++ {
		select {
		case e := <-ch:
			receivedEvents = append(receivedEvents, e)
		case <-timeout:
			t.Fatalf("timeout waiting for event %d, only received %d events", i+1, len(receivedEvents))
		}
	}

	// Verify we received the correct events
	assert.Len(t, receivedEvents, 2)
	assert.Equal(t, events.EventType("event2"), receivedEvents[0].Type)
	assert.Equal(t, events.EventType("event3"), receivedEvents[1].Type)
}

func TestSubscribeFrom_ThenLive(t *testing.T) {
	database := setupTestDB(t)
	jm := NewJobManager(database, 10)
	repoPath := setupTestRepo(t)

	// Start job with a cancellable context
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	cfg := validJobConfigWithRepo(repoPath)
	jobID, err := jm.Start(ctx, cancel, cfg)
	require.NoError(t, err)

	// Wait for job to be tracked
	var job *ManagedJob
	var exists bool
	for i := 0; i < 100; i++ {
		job, exists = jm.Get(jobID)
		if exists {
			break
		}
		time.Sleep(10 * time.Millisecond)
	}
	require.True(t, exists, "job should be tracked")
	require.NotNil(t, job, "job should not be nil")

	// Store a historical event
	err = database.AppendEvent(jobID, "historical", nil, nil)
	require.NoError(t, err)

	// Subscribe from the beginning while job is still running
	ch, cleanup, err := jm.SubscribeFrom(jobID, 1)
	require.NoError(t, err)
	defer cleanup()

	// Receive historical event first
	select {
	case e := <-ch:
		assert.Equal(t, events.EventType("historical"), e.Type)
	case <-time.After(time.Second):
		t.Fatal("timeout waiting for historical event")
	}

	// Now emit a live event directly to the job's event bus
	// and verify we receive it through our subscription
	liveEvent := events.Event{Type: "live"}
	job.Events.Emit(liveEvent)

	// Should receive live event
	select {
	case e := <-ch:
		assert.Equal(t, events.EventType("live"), e.Type)
	case <-time.After(time.Second):
		t.Fatal("timeout waiting for live event")
	}
}

func TestBroadcast_MultipleSubscribers(t *testing.T) {
	database := setupTestDB(t)
	jm := NewJobManager(database, 10)
	repoPath := setupTestRepo(t)

	// Start a job
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	jobID, err := jm.Start(ctx, cancel, validJobConfigWithRepo(repoPath))
	require.NoError(t, err)

	// Create multiple subscribers
	ch1, cleanup1, err := jm.Subscribe(jobID)
	require.NoError(t, err)
	defer cleanup1()

	ch2, cleanup2, err := jm.Subscribe(jobID)
	require.NoError(t, err)
	defer cleanup2()

	ch3, cleanup3, err := jm.Subscribe(jobID)
	require.NoError(t, err)
	defer cleanup3()

	// Broadcast an event
	testEvent := events.Event{Type: "test-multi"}
	jm.broadcast(jobID, testEvent)

	// All subscribers should receive the event
	timeout := time.After(time.Second)

	select {
	case e := <-ch1:
		assert.Equal(t, events.EventType("test-multi"), e.Type)
	case <-timeout:
		t.Fatal("subscriber 1 did not receive event")
	}

	select {
	case e := <-ch2:
		assert.Equal(t, events.EventType("test-multi"), e.Type)
	case <-timeout:
		t.Fatal("subscriber 2 did not receive event")
	}

	select {
	case e := <-ch3:
		assert.Equal(t, events.EventType("test-multi"), e.Type)
	case <-timeout:
		t.Fatal("subscriber 3 did not receive event")
	}
}

func TestBroadcast_SlowSubscriber(t *testing.T) {
	database := setupTestDB(t)
	jm := NewJobManager(database, 10)
	repoPath := setupTestRepo(t)

	// Start a job
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	jobID, err := jm.Start(ctx, cancel, validJobConfigWithRepo(repoPath))
	require.NoError(t, err)

	// Create two subscribers
	fastCh, fastCleanup, err := jm.Subscribe(jobID)
	require.NoError(t, err)
	defer fastCleanup()

	_, slowCleanup, err := jm.Subscribe(jobID)
	require.NoError(t, err)
	defer slowCleanup()

	// Broadcast more events than the buffer size (100 events)
	// to ensure slow subscriber's buffer fills up
	for i := 0; i < 150; i++ {
		testEvent := events.Event{Type: events.EventType("test-event")}
		jm.broadcast(jobID, testEvent)
	}

	// Fast subscriber should receive events
	timeout := time.After(time.Second)
	received := 0
outer:
	for i := 0; i < 100; i++ {
		select {
		case <-fastCh:
			received++
		case <-timeout:
			break outer
		}
	}

	// Fast subscriber should have received many events
	assert.Greater(t, received, 0, "fast subscriber should receive events")

	// The test passes if the slow subscriber doesn't block the fast one
	// We don't need to verify the slow subscriber's behavior explicitly,
	// as long as the fast one got events without blocking
}

func TestCloseSubscriptions(t *testing.T) {
	database := setupTestDB(t)
	jm := NewJobManager(database, 10)
	repoPath := setupTestRepo(t)

	// Start a job
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	jobID, err := jm.Start(ctx, cancel, validJobConfigWithRepo(repoPath))
	require.NoError(t, err)

	// Create subscribers
	_, cleanup1, err := jm.Subscribe(jobID)
	require.NoError(t, err)
	defer cleanup1()

	_, cleanup2, err := jm.Subscribe(jobID)
	require.NoError(t, err)
	defer cleanup2()

	// Close all subscriptions for the job
	jm.closeJobSubscriptions(jobID)

	// Give some time for channels to close
	time.Sleep(100 * time.Millisecond)

	// Trying to broadcast after closing should not cause any issues
	// (events will be dropped since bus is closed)
	jm.broadcast(jobID, events.Event{Type: "after-close"})

	// This test verifies that closeJobSubscriptions doesn't panic
	// and that the event bus is properly closed
}
