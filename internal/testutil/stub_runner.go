package testutil

import (
	"context"
	"fmt"
	"strings"
	"sync"
)

type StubRunner struct {
	mu       sync.Mutex
	stubs    map[string][]stubResponse
	defaults map[string]stubResponse
	calls    []string
}

type stubResponse struct {
	out string
	err error
}

func NewStubRunner() *StubRunner {
	return &StubRunner{
		stubs:    make(map[string][]stubResponse),
		defaults: make(map[string]stubResponse),
	}
}

func (s *StubRunner) Stub(args string, out string, err error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.stubs[args] = append(s.stubs[args], stubResponse{out: out, err: err})
}

func (s *StubRunner) StubDefault(args string, out string, err error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.defaults[args] = stubResponse{out: out, err: err}
}

func (s *StubRunner) Exec(ctx context.Context, dir string, args ...string) (string, error) {
	key := strings.Join(args, " ")
	s.mu.Lock()
	s.calls = append(s.calls, key)
	queue := s.stubs[key]
	if len(queue) == 0 {
		if resp, ok := s.defaults[key]; ok {
			s.mu.Unlock()
			return resp.out, resp.err
		}
		s.mu.Unlock()
		return "", fmt.Errorf("unexpected git call: %s", key)
	}
	resp := queue[0]
	s.stubs[key] = queue[1:]
	s.mu.Unlock()
	return resp.out, resp.err
}

func (s *StubRunner) ExecWithStdin(ctx context.Context, dir string, stdin string, args ...string) (string, error) {
	return s.Exec(ctx, dir, args...)
}

func (s *StubRunner) CallsFor(args ...string) int {
	key := strings.Join(args, " ")
	s.mu.Lock()
	defer s.mu.Unlock()
	count := 0
	for _, call := range s.calls {
		if call == key {
			count++
		}
	}
	return count
}
