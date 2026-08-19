package metrics

import "sync"

// Store keeps a reservoir per release, lazily initialised on first use. It is
// safe for concurrent use by many ingestion goroutines.
type Store struct {
	mu         sync.RWMutex
	sampler    *Sampler
	reservoirs map[string]*Reservoir
	seed       int64
}

func NewStore(sampler *Sampler, seed int64) *Store {
	return &Store{
		sampler:    sampler,
		seed:       seed,
		reservoirs: make(map[string]*Reservoir),
	}
}

// Reservoir returns the reservoir for a release, creating it on first use.
func (s *Store) Reservoir(releaseID string) *Reservoir {
	s.mu.RLock()
	r := s.reservoirs[releaseID]
	s.mu.RUnlock()
	if r != nil {
		return r
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	s.reservoirs[releaseID] = s.sampler.NewReservoir(s.seed)
	return s.reservoirs[releaseID]
}
