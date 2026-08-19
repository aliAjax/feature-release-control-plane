package repository

import (
	"context"
	"fmt"
	"sort"
	"sync"

	"github.com/example/feature-release-control-plane/internal/audit"

	"github.com/example/feature-release-control-plane/internal/configdomain"
	"github.com/example/feature-release-control-plane/internal/platform/httpx"
)

type ConfigRepository interface {
	Create(context.Context, configdomain.Config) error
	Get(context.Context, string) (configdomain.Config, error)
	List(context.Context, configdomain.Scope, int, int) ([]configdomain.Config, int, error)
	Update(context.Context, configdomain.Config, int64) error
	PutVersion(context.Context, configdomain.ConfigVersion) error
	GetVersion(context.Context, string, int64) (configdomain.ConfigVersion, error)
	ListVersions(context.Context, string) ([]configdomain.ConfigVersion, error)
	UpdateVersion(context.Context, configdomain.ConfigVersion, int64) error
	Published(context.Context, configdomain.Scope) ([]configdomain.ConfigVersion, error)
}

// Memory is the in-memory adapter used for deterministic local verification.
type Memory struct {
	mu         sync.RWMutex
	configs    map[string]configdomain.Config
	configKeys map[string]string
	versions   map[string]map[int64]configdomain.ConfigVersion
	audits     []audit.Record
	outbox     map[string]OutboxEvent
}

func NewMemory() *Memory {
	return &Memory{configs: map[string]configdomain.Config{}, configKeys: map[string]string{}, versions: map[string]map[int64]configdomain.ConfigVersion{}, outbox: map[string]OutboxEvent{}}
}
func (m *Memory) Create(_ context.Context, c configdomain.Config) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	if _, ok := m.configs[c.ID]; ok {
		return fmt.Errorf("%w: config id exists", httpx.ErrConflict)
	}
	key := c.Scope.Key() + "/" + c.Key
	if _, ok := m.configKeys[key]; ok {
		return fmt.Errorf("%w: configuration key exists in scope", httpx.ErrConflict)
	}
	m.configs[c.ID] = c
	m.configKeys[key] = c.ID
	return nil
}
func (m *Memory) Get(_ context.Context, id string) (configdomain.Config, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()
	c, ok := m.configs[id]
	if !ok {
		return c, fmt.Errorf("%w: configuration", httpx.ErrNotFound)
	}
	return c, nil
}
func (m *Memory) List(_ context.Context, s configdomain.Scope, page, size int) ([]configdomain.Config, int, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()
	all := []configdomain.Config{}
	for _, c := range m.configs {
		if c.Scope == s {
			all = append(all, c)
		}
	}
	sort.Slice(all, func(i, j int) bool { return all[i].Key < all[j].Key })
	return pageOf(all, page, size)
}
func pageOf[T any](items []T, page, size int) ([]T, int, error) {
	total := len(items)
	start := (page - 1) * size
	if start > total {
		return []T{}, total, nil
	}
	end := start + size
	if end > total {
		end = total
	}
	return items[start:end], total, nil
}
func (m *Memory) Update(_ context.Context, c configdomain.Config, expected int64) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	existing, ok := m.configs[c.ID]
	if !ok {
		return fmt.Errorf("%w: configuration", httpx.ErrNotFound)
	}
	if existing.Version != expected {
		return fmt.Errorf("%w: configuration etag does not match", httpx.ErrPrecondition)
	}
	m.configs[c.ID] = c
	return nil
}
func (m *Memory) PutVersion(_ context.Context, v configdomain.ConfigVersion) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	if _, ok := m.configs[v.ConfigID]; !ok {
		return fmt.Errorf("%w: configuration", httpx.ErrNotFound)
	}
	if m.versions[v.ConfigID] == nil {
		m.versions[v.ConfigID] = map[int64]configdomain.ConfigVersion{}
	}
	if _, ok := m.versions[v.ConfigID][v.Number]; ok {
		return fmt.Errorf("%w: version exists", httpx.ErrConflict)
	}
	m.versions[v.ConfigID][v.Number] = v
	return nil
}
func (m *Memory) GetVersion(_ context.Context, id string, num int64) (configdomain.ConfigVersion, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()
	v, ok := m.versions[id][num]
	if !ok {
		return v, fmt.Errorf("%w: configuration version", httpx.ErrNotFound)
	}
	return v, nil
}
func (m *Memory) ListVersions(_ context.Context, id string) ([]configdomain.ConfigVersion, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()
	all := []configdomain.ConfigVersion{}
	for _, v := range m.versions[id] {
		all = append(all, v)
	}
	sort.Slice(all, func(i, j int) bool { return all[i].Number < all[j].Number })
	return all, nil
}
func (m *Memory) UpdateVersion(_ context.Context, v configdomain.ConfigVersion, expected int64) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	old, ok := m.versions[v.ConfigID][v.Number]
	if !ok {
		return fmt.Errorf("%w: configuration version", httpx.ErrNotFound)
	}
	if old.Revision != expected {
		return fmt.Errorf("%w: version revision mismatch", httpx.ErrPrecondition)
	}
	m.versions[v.ConfigID][v.Number] = v
	return nil
}
func (m *Memory) Published(_ context.Context, s configdomain.Scope) ([]configdomain.ConfigVersion, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()
	result := []configdomain.ConfigVersion{}
	for id, c := range m.configs {
		if c.Scope != s || c.Archived {
			continue
		}
		best := configdomain.ConfigVersion{}
		for _, v := range m.versions[id] {
			if v.State == configdomain.Published && (best.Number == 0 || v.Number > best.Number) {
				best = v
			}
		}
		if best.Number > 0 {
			result = append(result, best)
		}
	}
	sort.Slice(result, func(i, j int) bool { return result[i].ConfigID < result[j].ConfigID })
	return result, nil
}
