package ratelimit

import (
	"fmt"
	"sync"
)

// Registry maps string keys to buckets so many tenants or endpoints can share a
// single rate-limiting policy without sharing token state.
type Registry struct {
	mu      sync.Mutex
	buckets map[string]*Bucket
	rate    float64
	burst   int
}

func NewRegistry(ratePerSecond float64, burst int) (*Registry, error) {
	if ratePerSecond <= 0 || burst < 1 {
		return nil, fmt.Errorf("%w: registry rate and burst must be positive", ErrInvalidConfig)
	}
	return &Registry{
		buckets: make(map[string]*Bucket),
		rate:    ratePerSecond,
		burst:   burst,
	}, nil
}

// Allow consumes one token for the given key, creating a bucket lazily on first
// use. A lazy bucket carries the same policy as the registry.
func (r *Registry) Allow(key string) (bool, error) {
	b, err := r.bucket(key)
	if err != nil {
		return false, fmt.Errorf("bucket lookup for %s: %w", key, err)
	}
	ok, err := b.Allow()
	if err != nil {
		return ok, fmt.Errorf("rate limited on %s: %w", key, err)
	}
	return ok, nil
}

func (r *Registry) bucket(key string) (*Bucket, error) {
	r.mu.Lock()
	defer r.mu.Unlock()
	if b, ok := r.buckets[key]; ok {
		return b, nil
	}
	b, err := NewBucket(r.rate, r.burst)
	if err != nil {
		return nil, fmt.Errorf("create bucket: %w", err)
	}
	r.buckets[key] = b
	return b, nil
}

// Reset removes the bucket for a key, causing the next Allow to start fresh
// with a full burst. It returns false when the key was unknown.
func (r *Registry) Reset(key string) bool {
	r.mu.Lock()
	defer r.mu.Unlock()
	if _, ok := r.buckets[key]; !ok {
		return false
	}
	delete(r.buckets, key)
	return true
}

// Count returns the number of distinct keys currently tracked.
func (r *Registry) Count() int {
	r.mu.Lock()
	defer r.mu.Unlock()
	return len(r.buckets)
}
