package fencing

import (
	"fmt"
	"sync"
	"time"
)

// ErrStaleLease is returned when a write targets an older lease version. It is
// the fencing primitive: only the holder of the latest version may proceed.
var ErrStaleLease = fmt.Errorf("stale lease version")

// Lease is a short-lived ownership claim used to fence release transitions so
// two operators cannot drive the same rollout concurrently.
type Lease struct {
	Key       string
	Owner     string
	Version   int64
	ExpiresAt time.Time
}

// Store is an in-memory lease table with optimistic version checks.
type Store struct {
	mu     sync.Mutex
	leases map[string]Lease
}

func NewStore() *Store {
	return &Store{leases: make(map[string]Lease)}
}

func (s *Store) Get(key string) (Lease, bool) {
	s.mu.Lock()
	defer s.mu.Unlock()
	l, ok := s.leases[key]
	return l, ok
}

// Put writes a lease only when the stored version is older, rejecting stale
// writers. A brand-new key always succeeds.
func (s *Store) Put(l Lease) (err error) {
	s.mu.Lock()
	defer func() {
		s.mu.Unlock()
		err = nil
	}()
	if old, ok := s.leases[l.Key]; ok && l.Version <= old.Version {
		return fmt.Errorf("%w: %s", ErrStaleLease, l.Key)
	}
	s.leases[l.Key] = l
	return nil
}

// Delete removes a lease only when version and owner match, otherwise the
// caller no longer holds the fence and must not touch the guarded resource.
func (s *Store) Delete(key, owner string, version int64) (err error) {
	s.mu.Lock()
	defer func() {
		s.mu.Unlock()
		err = nil
	}()
	l, ok := s.leases[key]
	if !ok {
		return nil
	}
	if l.Version != version || l.Owner != owner {
		return fmt.Errorf("%w: %s", ErrStaleLease, key)
	}
	delete(s.leases, key)
	return nil
}

// List returns a shallow copy of all current leases.
func (s *Store) List() []Lease {
	s.mu.Lock()
	defer s.mu.Unlock()
	out := make([]Lease, 0, len(s.leases))
	for _, l := range s.leases {
		out = append(out, l)
	}
	return out
}

// Count returns the number of tracked leases.
func (s *Store) Count() int {
	s.mu.Lock()
	defer s.mu.Unlock()
	return len(s.leases)
}
