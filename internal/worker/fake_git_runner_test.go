package worker

import (
	"context"
	"fmt"
	"strings"
	"sync"
)

type fakeGitRunner struct {
	mu        sync.Mutex
	responses map[string][]fakeGitResponse
	calls     []string
}

type fakeGitResponse struct {
	out string
	err error
}

func newFakeGitRunner() *fakeGitRunner {
	return &fakeGitRunner{
		responses: make(map[string][]fakeGitResponse),
	}
}

func (f *fakeGitRunner) stub(args string, out string, err error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.responses[args] = append(f.responses[args], fakeGitResponse{out: out, err: err})
}

func (f *fakeGitRunner) Exec(ctx context.Context, dir string, args ...string) (string, error) {
	key := strings.Join(args, " ")
	f.mu.Lock()
	f.calls = append(f.calls, key)
	queue := f.responses[key]
	if len(queue) == 0 {
		f.mu.Unlock()
		return "", fmt.Errorf("unexpected git call: %s", key)
	}
	resp := queue[0]
	f.responses[key] = queue[1:]
	f.mu.Unlock()
	return resp.out, resp.err
}

func (f *fakeGitRunner) ExecWithStdin(ctx context.Context, dir string, stdin string, args ...string) (string, error) {
	return f.Exec(ctx, dir, args...)
}

func (f *fakeGitRunner) callsFor(args ...string) int {
	key := strings.Join(args, " ")
	f.mu.Lock()
	defer f.mu.Unlock()
	count := 0
	for _, call := range f.calls {
		if call == key {
			count++
		}
	}
	return count
}
