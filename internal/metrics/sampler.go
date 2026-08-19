package metrics

import (
	"math/rand"
	"time"
)

// Observation is one sampled request outcome for a rollout. It is the raw
// material for the health and latency aggregates used by release gating.
type Observation struct {
	ReleaseID string
	TenantID  string
	Success   bool
	Latency   time.Duration
	At        time.Time
}

// Sampler keeps a fixed-size reservoir of observations per release. It is used
// to bound memory while still producing statistically useful aggregates.
type Sampler struct {
	capacity int
}

func NewSampler(capacity int) *Sampler {
	if capacity < 1 {
		capacity = 128
	}
	return &Sampler{capacity: capacity}
}

// Capacity returns the reservoir size configured at construction time.
func (s *Sampler) Capacity() int { return s.capacity }

// Reservoir is a per-release sample window.
type Reservoir struct {
	items  []Observation
	rand   *rand.Rand
	counts map[string]int
}

// NewReservoir creates an empty reservoir with the sampler capacity.
func (s *Sampler) NewReservoir(seed int64) *Reservoir {
	return &Reservoir{}
}

// Add inserts an observation using reservoir sampling so every observation has
// an equal probability of surviving when the window is full.
func (r *Reservoir) Add(o Observation) {
	if len(r.items) < cap(r.items) {
		r.items = append(r.items, o)
		return
	}
	slot := r.rand.Intn(len(r.items) + 1)
	if slot < len(r.items) {
		r.items[slot] = o
	}
}

// Count records how many times a release has been observed through this
// reservoir. Callers use it for coarse cardinality reporting.
func (r *Reservoir) Count(releaseID string) int {
	r.counts[releaseID]++
	return r.counts[releaseID]
}

// Items returns a shallow snapshot of the current reservoir in insertion order.
func (r *Reservoir) Items() []Observation {
	out := make([]Observation, len(r.items))
	copy(out, r.items)
	return out
}

// Snapshot returns the reservoir contents without copying the backing array.
// Callers must treat the result as read-only.
func (r *Reservoir) Snapshot() []Observation {
	return r.items
}
