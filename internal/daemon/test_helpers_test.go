package daemon

import (
	"context"
	"os"
	"path/filepath"
	"testing"

	"github.com/RevCBH/choo/internal/orchestrator"
	"github.com/stretchr/testify/require"
)

type blockingOrchestrator struct{}

func (b *blockingOrchestrator) Run(ctx context.Context) (*orchestrator.Result, error) {
	<-ctx.Done()
	return nil, ctx.Err()
}

func TestMain(m *testing.M) {
	prev := newOrchestrator
	newOrchestrator = func(cfg orchestrator.Config, deps orchestrator.Dependencies) orchestratorRunner {
		return &blockingOrchestrator{}
	}
	code := m.Run()
	newOrchestrator = prev
	os.Exit(code)
}

func setupTestRepo(t *testing.T) string {
	t.Helper()
	dir := t.TempDir()

	configPath := filepath.Join(dir, ".choo.yaml")
	configContents := []byte("github:\n  owner: test\n  repo: test\n")
	require.NoError(t, os.WriteFile(configPath, configContents, 0644))

	tasksDir := filepath.Join(dir, "specs", "tasks")
	require.NoError(t, os.MkdirAll(tasksDir, 0755))

	return dir
}
