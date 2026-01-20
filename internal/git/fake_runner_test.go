package git

import (
	"context"
	"fmt"
	"strings"
	"sync"
)

type fakeRunner struct {
	mu        sync.Mutex
	responses map[string][]fakeResponse
	calls     []fakeCall
}

type fakeResponse struct {
	out string
	err error
}

type fakeCall struct {
	dir  string
	args []string
}

func newFakeRunner() *fakeRunner {
	return &fakeRunner{
		responses: make(map[string][]fakeResponse),
	}
}

func (f *fakeRunner) stub(args string, out string, err error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.responses[args] = append(f.responses[args], fakeResponse{out: out, err: err})
}

func (f *fakeRunner) Exec(ctx context.Context, dir string, args ...string) (string, error) {
	key := strings.Join(args, " ")
	f.mu.Lock()
	f.calls = append(f.calls, fakeCall{dir: dir, args: append([]string(nil), args...)})
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

func (f *fakeRunner) ExecWithStdin(ctx context.Context, dir string, stdin string, args ...string) (string, error) {
	return f.Exec(ctx, dir, args...)
}

func (f *fakeRunner) callsFor(args ...string) int {
	key := strings.Join(args, " ")
	f.mu.Lock()
	defer f.mu.Unlock()
	count := 0
	for _, call := range f.calls {
		if strings.Join(call.args, " ") == key {
			count++
		}
	}
	return count
}
