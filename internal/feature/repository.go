package feature

import (
	"fmt"
	"os"
	"sync"

	"github.com/RevCBH/choo/internal/events"
	"gopkg.in/yaml.v3"
)

// PRDRepository provides access to PRDs with caching
type PRDRepository struct {
	baseDir string
	cache   map[string]*PRD
	mu      sync.RWMutex
	bus     *events.Bus // optional, for event emission
}

// NewPRDRepository creates a repository for the given base directory
func NewPRDRepository(baseDir string) *PRDRepository {
	return &PRDRepository{
		baseDir: baseDir,
		cache:   make(map[string]*PRD),
	}
}

// NewPRDRepositoryWithBus creates a repository with event emission support
func NewPRDRepositoryWithBus(baseDir string, bus *events.Bus) *PRDRepository {
	return &PRDRepository{
		baseDir: baseDir,
		cache:   make(map[string]*PRD),
		bus:     bus,
	}
}

// Get retrieves a PRD by ID, using cache if available
// Returns nil, error if PRD not found
func (r *PRDRepository) Get(id string) (*PRD, error) {
	r.mu.RLock()
	prd, ok := r.cache[id]
	r.mu.RUnlock()

	if !ok {
		return nil, fmt.Errorf("PRD not found: %s", id)
	}
	return prd, nil
}

// List returns all PRDs, optionally filtered by status
// Empty filter returns all PRDs
func (r *PRDRepository) List(statusFilter []string) ([]*PRD, error) {
	r.mu.RLock()
	defer r.mu.RUnlock()

	// If no filter, return all PRDs
	if len(statusFilter) == 0 {
		prds := make([]*PRD, 0, len(r.cache))
		for _, prd := range r.cache {
			prds = append(prds, prd)
		}
		return prds, nil
	}

	// Build filter set
	filterSet := make(map[string]bool)
	for _, s := range statusFilter {
		filterSet[s] = true
	}

	// Return filtered PRDs
	prds := make([]*PRD, 0)
	for _, prd := range r.cache {
		if filterSet[prd.Status] {
			prds = append(prds, prd)
		}
	}
	return prds, nil
}

// Refresh clears the cache and re-discovers PRDs from filesystem
// Emits PRDDiscovered events if bus is configured
func (r *PRDRepository) Refresh() error {
	prds, err := DiscoverPRDs(r.baseDir)
	if err != nil {
		return err
	}

	r.mu.Lock()
	defer r.mu.Unlock()

	r.cache = make(map[string]*PRD)
	for _, prd := range prds {
		r.cache[prd.ID] = prd
		if r.bus != nil {
			r.bus.Emit(events.NewEvent(events.PRDDiscovered, "").
				WithPayload(map[string]string{
					"prd_id": prd.ID,
					"status": prd.Status,
					"path":   prd.FilePath,
				}))
		}
	}

	return nil
}

// CheckDrift compares current body hash against cached hash
// Returns true if PRD body has changed since last refresh
func (r *PRDRepository) CheckDrift(id string) (bool, error) {
	r.mu.RLock()
	cached, ok := r.cache[id]
	r.mu.RUnlock()

	if !ok {
		return false, fmt.Errorf("PRD not found: %s", id)
	}

	current, err := ParsePRD(cached.FilePath)
	if err != nil {
		return false, err
	}

	return cached.BodyHash != current.BodyHash, nil
}

// WritePRDFrontmatter updates the frontmatter in a PRD file
// Preserves the body content unchanged
func WritePRDFrontmatter(prd *PRD) error {
	// Read the current file to get the original body
	content, err := os.ReadFile(prd.FilePath)
	if err != nil {
		return fmt.Errorf("read PRD file: %w", err)
	}

	// Parse frontmatter to get the body
	_, body, err := parseFrontmatter(content)
	if err != nil {
		return fmt.Errorf("parse frontmatter: %w", err)
	}

	// Serialize the PRD frontmatter
	frontmatterData, err := yaml.Marshal(prd)
	if err != nil {
		return fmt.Errorf("marshal frontmatter: %w", err)
	}

	// Construct the new file content with frontmatter + body
	var newContent []byte
	newContent = append(newContent, []byte("---\n")...)
	newContent = append(newContent, frontmatterData...)
	newContent = append(newContent, []byte("---\n")...)
	newContent = append(newContent, body...)

	// Write the file back
	if err := os.WriteFile(prd.FilePath, newContent, 0644); err != nil {
		return fmt.Errorf("write PRD file: %w", err)
	}

	return nil
}
