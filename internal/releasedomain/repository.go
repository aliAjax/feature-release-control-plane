package releasedomain

import (
	"context"
	"fmt"
	"github.com/example/feature-release-control-plane/internal/platform/httpx"
	"sort"
	"sync"
)

type Repository interface {
	Create(context.Context, Release) error
	Get(context.Context, string) (Release, error)
	Update(context.Context, Release, int64) error
	List(context.Context, int, int) ([]Release, int, error)
}
type MemoryRepository struct {
	mu          sync.RWMutex
	releases    map[string]Release
	idempotency map[string]string
}

func NewMemoryRepository() *MemoryRepository {
	return &MemoryRepository{releases: map[string]Release{}, idempotency: map[string]string{}}
}
func (m *MemoryRepository) Create(_ context.Context, r Release) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	if r.IdempotencyKey != "" {
		if id := m.idempotency[r.IdempotencyKey]; id != "" {
			return fmt.Errorf("%w: request already created release %s", httpx.ErrConflict, id)
		}
	}
	if _, ok := m.releases[r.ID]; ok {
		return fmt.Errorf("%w: release", httpx.ErrConflict)
	}
	m.releases[r.ID] = r
	if r.IdempotencyKey != "" {
		m.idempotency[r.IdempotencyKey] = r.ID
	}
	return nil
}
func (m *MemoryRepository) Get(_ context.Context, id string) (Release, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()
	r, ok := m.releases[id]
	if !ok {
		return r, fmt.Errorf("%w: release", httpx.ErrNotFound)
	}
	return r, nil
}
func (m *MemoryRepository) Update(_ context.Context, r Release, expected int64) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	old, ok := m.releases[r.ID]
	if !ok {
		return fmt.Errorf("%w: release", httpx.ErrNotFound)
	}
	if old.Revision != expected {
		return fmt.Errorf("%w: release revision", httpx.ErrPrecondition)
	}
	m.releases[r.ID] = r
	return nil
}
func (m *MemoryRepository) List(_ context.Context, page, size int) ([]Release, int, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()
	all := []Release{}
	for _, r := range m.releases {
		all = append(all, r)
	}
	sort.Slice(all, func(i, j int) bool { return all[i].CreatedAt.Before(all[j].CreatedAt) })
	total := len(all)
	from := (page - 1) * size
	if from >= total {
		return []Release{}, total, nil
	}
	to := from + size
	if to > total {
		to = total
	}
	return all[from:to], total, nil
}
