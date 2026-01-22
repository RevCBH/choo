package git

import "sync"

// repoLocks is a global lock registry keyed by canonical repo path.
var repoLocks = struct {
	sync.Mutex
	locks map[string]*sync.Mutex
}{locks: make(map[string]*sync.Mutex)}

// getRepoLock returns (or creates) a mutex for the given repo path.
// The path must be canonical (absolute, resolved symlinks, cleaned).
func getRepoLock(path string) *sync.Mutex {
	repoLocks.Lock()
	defer repoLocks.Unlock()
	if repoLocks.locks[path] == nil {
		repoLocks.locks[path] = &sync.Mutex{}
	}
	return repoLocks.locks[path]
}
