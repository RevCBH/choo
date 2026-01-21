package feature

import (
	"os"
	"path/filepath"
	"sync"
	"testing"

	"github.com/RevCBH/choo/internal/events"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// setupTestRepo creates a temporary directory with test PRD files
func setupTestRepo(t *testing.T) (string, func()) {
	dir := t.TempDir()

	// Create test PRD files
	featureA := `---
prd_id: feature-a
title: Feature A
status: draft
---
This is feature A body content.
`
	featureB := `---
prd_id: feature-b
title: Feature B
status: approved
---
This is feature B body content.
`
	featureC := `---
prd_id: feature-c
title: Feature C
status: in_progress
---
This is feature C body content.
`

	require.NoError(t, os.WriteFile(filepath.Join(dir, "feature-a.md"), []byte(featureA), 0644))
	require.NoError(t, os.WriteFile(filepath.Join(dir, "feature-b.md"), []byte(featureB), 0644))
	require.NoError(t, os.WriteFile(filepath.Join(dir, "feature-c.md"), []byte(featureC), 0644))

	return dir, func() {
		// cleanup is handled by t.TempDir()
	}
}

func TestRepository_NewEmpty(t *testing.T) {
	repo := NewPRDRepository("/test/dir")

	assert.NotNil(t, repo)
	assert.Equal(t, "/test/dir", repo.baseDir)
	assert.NotNil(t, repo.cache)
	assert.Equal(t, 0, len(repo.cache))
	assert.Nil(t, repo.bus)
}

func TestRepository_Refresh(t *testing.T) {
	dir, cleanup := setupTestRepo(t)
	defer cleanup()

	repo := NewPRDRepository(dir)

	// Cache should be empty initially
	assert.Equal(t, 0, len(repo.cache))

	// Refresh should populate cache
	err := repo.Refresh()
	require.NoError(t, err)

	// Cache should now contain 3 PRDs
	assert.Equal(t, 3, len(repo.cache))

	// Verify PRDs are cached by ID
	prdA, ok := repo.cache["feature-a"]
	assert.True(t, ok)
	assert.Equal(t, "Feature A", prdA.Title)
	assert.Equal(t, "draft", prdA.Status)

	prdB, ok := repo.cache["feature-b"]
	assert.True(t, ok)
	assert.Equal(t, "Feature B", prdB.Title)
	assert.Equal(t, "approved", prdB.Status)

	prdC, ok := repo.cache["feature-c"]
	assert.True(t, ok)
	assert.Equal(t, "Feature C", prdC.Title)
	assert.Equal(t, "in_progress", prdC.Status)
}

func TestRepository_Get_Found(t *testing.T) {
	dir, cleanup := setupTestRepo(t)
	defer cleanup()

	repo := NewPRDRepository(dir)
	require.NoError(t, repo.Refresh())

	// Get existing PRD
	prd, err := repo.Get("feature-a")
	require.NoError(t, err)
	assert.NotNil(t, prd)
	assert.Equal(t, "feature-a", prd.ID)
	assert.Equal(t, "Feature A", prd.Title)
}

func TestRepository_Get_NotFound(t *testing.T) {
	dir, cleanup := setupTestRepo(t)
	defer cleanup()

	repo := NewPRDRepository(dir)
	require.NoError(t, repo.Refresh())

	// Get non-existent PRD
	prd, err := repo.Get("feature-x")
	assert.Error(t, err)
	assert.Nil(t, prd)
	assert.Contains(t, err.Error(), "PRD not found: feature-x")
}

func TestRepository_List_All(t *testing.T) {
	dir, cleanup := setupTestRepo(t)
	defer cleanup()

	repo := NewPRDRepository(dir)
	require.NoError(t, repo.Refresh())

	// List all PRDs (empty filter)
	prds, err := repo.List([]string{})
	require.NoError(t, err)
	assert.Equal(t, 3, len(prds))

	// List all PRDs (nil filter)
	prds, err = repo.List(nil)
	require.NoError(t, err)
	assert.Equal(t, 3, len(prds))
}

func TestRepository_List_Filtered(t *testing.T) {
	dir, cleanup := setupTestRepo(t)
	defer cleanup()

	repo := NewPRDRepository(dir)
	require.NoError(t, repo.Refresh())

	// Filter by single status
	prds, err := repo.List([]string{"draft"})
	require.NoError(t, err)
	assert.Equal(t, 1, len(prds))
	assert.Equal(t, "feature-a", prds[0].ID)

	// Filter by multiple statuses
	prds, err = repo.List([]string{"approved", "in_progress"})
	require.NoError(t, err)
	assert.Equal(t, 2, len(prds))

	// Verify the PRDs have correct statuses
	statuses := make(map[string]bool)
	for _, prd := range prds {
		statuses[prd.Status] = true
	}
	assert.True(t, statuses["approved"])
	assert.True(t, statuses["in_progress"])

	// Filter by non-existent status
	prds, err = repo.List([]string{"complete"})
	require.NoError(t, err)
	assert.Equal(t, 0, len(prds))
}

func TestRepository_CheckDrift_NoChange(t *testing.T) {
	dir, cleanup := setupTestRepo(t)
	defer cleanup()

	repo := NewPRDRepository(dir)
	require.NoError(t, repo.Refresh())

	// Check drift when nothing has changed
	hasDrift, err := repo.CheckDrift("feature-a")
	require.NoError(t, err)
	assert.False(t, hasDrift)
}

func TestRepository_CheckDrift_Changed(t *testing.T) {
	dir, cleanup := setupTestRepo(t)
	defer cleanup()

	repo := NewPRDRepository(dir)
	require.NoError(t, repo.Refresh())

	// Modify the body content of feature-a
	modifiedContent := `---
prd_id: feature-a
title: Feature A
status: draft
---
This is MODIFIED feature A body content.
`
	filePath := filepath.Join(dir, "feature-a.md")
	require.NoError(t, os.WriteFile(filePath, []byte(modifiedContent), 0644))

	// Check drift - should detect the change
	hasDrift, err := repo.CheckDrift("feature-a")
	require.NoError(t, err)
	assert.True(t, hasDrift)
}

func TestRepository_CheckDrift_NotFound(t *testing.T) {
	dir, cleanup := setupTestRepo(t)
	defer cleanup()

	repo := NewPRDRepository(dir)
	require.NoError(t, repo.Refresh())

	// Check drift for non-existent PRD
	hasDrift, err := repo.CheckDrift("feature-x")
	assert.Error(t, err)
	assert.False(t, hasDrift)
	assert.Contains(t, err.Error(), "PRD not found: feature-x")
}

func TestRepository_Concurrent(t *testing.T) {
	dir, cleanup := setupTestRepo(t)
	defer cleanup()

	repo := NewPRDRepository(dir)
	require.NoError(t, repo.Refresh())

	var wg sync.WaitGroup
	iterations := 100

	// Concurrent reads
	for i := 0; i < iterations; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			_, _ = repo.Get("feature-a")
			_, _ = repo.List([]string{"draft"})
		}()
	}

	// Concurrent writes
	for i := 0; i < 10; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			_ = repo.Refresh()
		}()
	}

	// Wait for all goroutines to finish
	wg.Wait()

	// Verify repository is still functional
	prd, err := repo.Get("feature-a")
	require.NoError(t, err)
	assert.NotNil(t, prd)
}

func TestRepository_EventEmission(t *testing.T) {
	dir, cleanup := setupTestRepo(t)
	defer cleanup()

	// Create bus and subscribe to events
	bus := events.NewBus(100)
	defer bus.Close()

	var emittedEvents []events.Event
	var mu sync.Mutex
	bus.Subscribe(func(e events.Event) {
		mu.Lock()
		defer mu.Unlock()
		emittedEvents = append(emittedEvents, e)
	})

	// Create repository with bus
	repo := NewPRDRepositoryWithBus(dir, bus)

	// Refresh should emit PRDDiscovered events
	err := repo.Refresh()
	require.NoError(t, err)

	// Give time for events to be processed
	// Since the bus processes events asynchronously, we need to wait
	for i := 0; i < 50; i++ {
		mu.Lock()
		count := len(emittedEvents)
		mu.Unlock()
		if count >= 3 {
			break
		}
		// Small sleep to allow event processing
		os.WriteFile(filepath.Join(dir, ".tmp"), []byte(""), 0644)
		os.Remove(filepath.Join(dir, ".tmp"))
	}

	mu.Lock()
	defer mu.Unlock()

	// Should have emitted 3 PRDDiscovered events
	assert.Equal(t, 3, len(emittedEvents))

	// Verify event types and payloads
	for _, e := range emittedEvents {
		assert.Equal(t, events.PRDDiscovered, e.Type)
		assert.NotNil(t, e.Payload)

		payload, ok := e.Payload.(map[string]string)
		assert.True(t, ok)
		assert.NotEmpty(t, payload["prd_id"])
		assert.NotEmpty(t, payload["status"])
		assert.NotEmpty(t, payload["path"])
	}
}

func TestWritePRDFrontmatter_Preserves(t *testing.T) {
	dir, cleanup := setupTestRepo(t)
	defer cleanup()

	repo := NewPRDRepository(dir)
	require.NoError(t, repo.Refresh())

	// Get a PRD
	prd, err := repo.Get("feature-a")
	require.NoError(t, err)

	// Store original body
	originalBody := prd.Body

	// Modify frontmatter fields
	prd.Status = "approved"
	prd.EstimatedUnits = 5

	// Write the updated frontmatter
	err = WritePRDFrontmatter(prd)
	require.NoError(t, err)

	// Re-parse the file
	updated, err := ParsePRD(prd.FilePath)
	require.NoError(t, err)

	// Verify frontmatter was updated
	assert.Equal(t, "approved", updated.Status)
	assert.Equal(t, 5, updated.EstimatedUnits)

	// Verify body was preserved
	assert.Equal(t, originalBody, updated.Body)
}
