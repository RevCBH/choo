package git

import (
	"sync"
	"testing"
)

func TestRepoLock_ReturnsSameLock(t *testing.T) {
	lock1 := getRepoLock("/path/to/repo")
	lock2 := getRepoLock("/path/to/repo")

	if lock1 != lock2 {
		t.Error("expected same lock instance for same path")
	}
}

func TestRepoLock_DifferentPaths(t *testing.T) {
	lock1 := getRepoLock("/path/to/repo1")
	lock2 := getRepoLock("/path/to/repo2")

	if lock1 == lock2 {
		t.Error("expected different lock instances for different paths")
	}
}

func TestRepoLock_ConcurrentAccess(t *testing.T) {
	var wg sync.WaitGroup
	locks := make([]*sync.Mutex, 100)

	for i := 0; i < 100; i++ {
		wg.Add(1)
		go func(idx int) {
			defer wg.Done()
			locks[idx] = getRepoLock("/concurrent/test/path")
		}(i)
	}

	wg.Wait()

	// All should be the same lock
	for i := 1; i < 100; i++ {
		if locks[i] != locks[0] {
			t.Errorf("lock %d differs from lock 0", i)
		}
	}
}
