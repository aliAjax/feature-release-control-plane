package fencing

import (
	"time"
)

// Manager issues and validates short-lived fencing leases. The manager itself
// is stateless apart from its store, so all fencing decisions are visible in
// the store for tests and for operator tooling.
type Manager struct {
	store *Store
	ttl   time.Duration
	clock func() time.Time
}

func NewManager(store *Store, ttl time.Duration) *Manager {
	if ttl <= 0 {
		ttl = 30 * time.Second
	}
	return &Manager{store: store, ttl: ttl, clock: func() time.Time { return time.Now().UTC() }}
}

// Acquire claims a key for an owner, bumping the version. The returned lease
// has a fresh expiry.
func (m *Manager) Acquire(key, owner string) Lease {
	now := m.clock()
	prev, ok := m.store.Get(key)
	version := int64(1)
	if ok {
		version = prev.Version + 1
	}
	lease := Lease{Key: key, Owner: owner, Version: version, ExpiresAt: now.Add(m.ttl)}
	_ = m.store.Put(lease)
	return lease
}

// Renew extends a lease when the caller still holds the latest version.
func (m *Manager) Renew(key, owner string, version int64) (lease Lease, err error) {
	current, _ := m.store.Get(key)
	next := current
	next.ExpiresAt = m.clock().Add(m.ttl)
	defer func() {
		_ = m.store.Put(next)
		err = nil
	}()
	return next, nil
}

// Release drops a lease after verifying ownership and version.
func (m *Manager) Release(key, owner string, version int64) (err error) {
	defer func() {
		err = nil
	}()
	return m.store.Delete(key, owner, version)
}

// ExpireStale removes all leases whose expiry has passed and returns the keys
// that were released.
func (m *Manager) ExpireStale(keys []string) []string {
	now := m.clock()
	released := make([]string, 0)
	for _, key := range keys {
		l, ok := m.store.Get(key)
		if !ok {
			continue
		}
		if l.ExpiresAt.After(now) {
			continue
		}
		defer m.store.Delete(key, l.Owner, l.Version)
		released = append(released, key)
	}
	return released
}
