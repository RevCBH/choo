package worker

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"sync/atomic"
	"testing"
	"time"

	"github.com/RevCBH/choo/internal/discovery"
	"github.com/RevCBH/choo/internal/events"
	"github.com/RevCBH/choo/internal/git"
)

type noopGitRunner struct{}

func (noopGitRunner) Exec(ctx context.Context, dir string, args ...string) (string, error) {
	// Simulate worktree creation by creating the directory path.
	if len(args) >= 5 && args[0] == "worktree" && args[1] == "add" {
		worktreePath := args[4]
		_ = os.MkdirAll(worktreePath, 0755)
	}
	return "", nil
}

func (noopGitRunner) ExecWithStdin(ctx context.Context, dir string, stdin string, args ...string) (string, error) {
	return "", nil
}

// setupRepo creates a temporary directory to act as a repo root for testing.
func setupRepo(t *testing.T) (repoDir string, cleanup func()) {
	t.Helper()

	tmpDir, err := os.MkdirTemp("", "pool-test-*")
	if err != nil {
		t.Fatalf("failed to create temp dir: %v", err)
	}

	repoDir = filepath.Join(tmpDir, "repo")
	if err := os.MkdirAll(repoDir, 0755); err != nil {
		t.Fatalf("failed to create repo dir: %v", err)
	}

	cleanup = func() {
		os.RemoveAll(tmpDir)
	}

	return repoDir, cleanup
}

// mockDeps creates mock dependencies for testing
func mockDeps(t *testing.T) WorkerDeps {
	t.Helper()

	repoDir, cleanup := setupRepo(t)
	t.Cleanup(cleanup)

	git.SetDefaultRunner(noopGitRunner{})

	t.Cleanup(func() {
		git.SetDefaultRunner(nil)
	})

	return WorkerDeps{
		Events: events.NewBus(100),
		Git:    git.NewWorktreeManager(repoDir, nil),
		GitHub: nil,
		Claude: nil,
	}
}

func TestPool_Submit_SingleUnit(t *testing.T) {
	deps := mockDeps(t)
	pool := NewPool(2, WorkerConfig{
		NoPR:         true,
		TargetBranch: "main",
	}, deps)

	unit := &discovery.Unit{
		ID:    "test-unit",
		Tasks: []*discovery.Task{},
	}

	err := pool.Submit(unit)
	if err != nil {
		t.Fatalf("submit failed: %v", err)
	}

	err = pool.Wait()
	if err != nil {
		t.Errorf("wait failed: %v", err)
	}

	stats := pool.Stats()
	if stats.TotalTasks != 0 {
		t.Errorf("expected 0 total tasks, got %d", stats.TotalTasks)
	}
}

func TestPool_Submit_MultipleUnits(t *testing.T) {
	deps := mockDeps(t)
	pool := NewPool(2, WorkerConfig{
		NoPR:         true,
		TargetBranch: "main",
	}, deps)

	for i := 0; i < 3; i++ {
		unit := &discovery.Unit{
			ID:    fmt.Sprintf("unit-%d", i),
			Tasks: []*discovery.Task{},
		}
		err := pool.Submit(unit)
		if err != nil {
			t.Fatalf("submit failed for unit %d: %v", i, err)
		}
	}

	err := pool.Wait()
	if err != nil {
		t.Errorf("wait failed: %v", err)
	}
}

func TestPool_Submit_RespectsMaxWorkers(t *testing.T) {
	deps := mockDeps(t)
	pool := NewPool(2, WorkerConfig{
		NoPR:         true,
		TargetBranch: "main",
	}, deps)

	// Submit 5 units
	for i := 0; i < 5; i++ {
		unit := &discovery.Unit{
			ID:    fmt.Sprintf("unit-%d", i),
			Tasks: []*discovery.Task{},
		}
		pool.Submit(unit)
	}

	// Check that we never exceed 2 concurrent workers
	// Sample the active workers count multiple times
	var maxObserved int32
	done := make(chan bool)

	go func() {
		for {
			select {
			case <-done:
				return
			default:
				stats := pool.Stats()
				current := int32(stats.ActiveWorkers)
				for {
					max := atomic.LoadInt32(&maxObserved)
					if current <= max {
						break
					}
					if atomic.CompareAndSwapInt32(&maxObserved, max, current) {
						break
					}
				}
				time.Sleep(1 * time.Millisecond)
			}
		}
	}()

	pool.Wait()
	close(done)

	if atomic.LoadInt32(&maxObserved) > 2 {
		t.Errorf("exceeded max workers: %d > 2", maxObserved)
	}
}

func TestPool_Submit_DuplicateUnit(t *testing.T) {
	deps := mockDeps(t)
	pool := NewPool(2, WorkerConfig{
		NoPR:         true,
		TargetBranch: "main",
	}, deps)

	unit := &discovery.Unit{ID: "same-id", Tasks: []*discovery.Task{}}

	err := pool.Submit(unit)
	if err != nil {
		t.Fatalf("first submit should succeed: %v", err)
	}

	err = pool.Submit(unit)
	if err == nil {
		t.Error("expected error for duplicate unit")
	}

	pool.Wait()
}

func TestPool_Wait_BlocksUntilComplete(t *testing.T) {
	deps := mockDeps(t)
	pool := NewPool(2, WorkerConfig{
		NoPR:         true,
		TargetBranch: "main",
	}, deps)

	unit := &discovery.Unit{
		ID:    "test-unit",
		Tasks: []*discovery.Task{},
	}

	pool.Submit(unit)

	err := pool.Wait()
	if err != nil {
		t.Errorf("wait failed: %v", err)
	}
}

func TestPool_Stats_Accurate(t *testing.T) {
	deps := mockDeps(t)
	pool := NewPool(2, WorkerConfig{
		NoPR:         true,
		TargetBranch: "main",
	}, deps)

	for i := 0; i < 3; i++ {
		pool.Submit(&discovery.Unit{
			ID: fmt.Sprintf("unit-%d", i),
			Tasks: []*discovery.Task{
				{Number: 1},
				{Number: 2},
			},
		})
	}

	pool.Wait()

	stats := pool.Stats()
	if stats.TotalTasks != 6 {
		t.Errorf("expected 6 total tasks, got %d", stats.TotalTasks)
	}
}

func TestPool_Shutdown_GracefulStop(t *testing.T) {
	deps := mockDeps(t)
	pool := NewPool(2, WorkerConfig{
		NoPR:         true,
		TargetBranch: "main",
	}, deps)

	// Submit unit
	pool.Submit(&discovery.Unit{ID: "unit", Tasks: []*discovery.Task{}})

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	err := pool.Shutdown(ctx)
	if err != nil {
		t.Errorf("shutdown failed: %v", err)
	}
}

func TestPool_Shutdown_Timeout(t *testing.T) {
	// This test is complex to implement without blocking workers
	// Skipping for now as it requires workers that intentionally don't complete
	t.Skip("Requires blocking worker implementation")
}
