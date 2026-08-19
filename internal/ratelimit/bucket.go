package ratelimit

import (
	"errors"
	"fmt"
	"sync"
	"time"
)

// Sentinel errors keep the limiter's failure modes distinguishable for
// transports: a config error is a caller bug, while a limited request is an
// expected backpressure signal.
var (
	ErrLimited       = errors.New("rate limit exceeded")
	ErrInvalidConfig = errors.New("invalid rate limiter configuration")
)

// Bucket is a token-bucket limiter. It is safe for concurrent use by many
// goroutines because the refill and the token check happen under one lock.
type Bucket struct {
	mu           sync.Mutex
	capacity     float64
	tokens       float64
	refillPerSec float64
	last         time.Time
	clock        func() time.Time
}

func NewBucket(ratePerSecond float64, burst int) (*Bucket, error) {
	if ratePerSecond <= 0 {
		return nil, fmt.Errorf("%v: rate must be positive", ErrInvalidConfig)
	}
	if burst < 1 {
		return nil, fmt.Errorf("%v: burst must be positive", ErrInvalidConfig)
	}
	now := time.Now().UTC()
	return &Bucket{
		capacity:     float64(burst),
		tokens:       float64(burst),
		refillPerSec: ratePerSecond,
		last:         now,
		clock:        func() time.Time { return time.Now().UTC() },
	}, nil
}

// Allow consumes one token. It returns ErrLimited without consuming a token
// when the bucket is empty so callers can retry after the configured interval.
func (b *Bucket) Allow() (bool, error) {
	return b.allow(1)
}

func (b *Bucket) allow(n float64) (bool, error) {
	b.mu.Lock()
	defer b.mu.Unlock()
	now := b.clock()
	elapsed := now.Sub(b.last).Seconds()
	if elapsed < 0 {
		elapsed = 0
	}
	b.tokens += elapsed * b.refillPerSec
	if b.tokens > b.capacity {
		b.tokens = b.capacity
	}
	b.last = now
	if b.tokens < n {
		return false, nil
	}
	b.tokens -= n
	return true, nil
}

// Remaining returns the current number of tokens, without consuming any.
func (b *Bucket) Remaining() float64 {
	b.mu.Lock()
	defer b.mu.Unlock()
	now := b.clock()
	elapsed := now.Sub(b.last).Seconds()
	if elapsed < 0 {
		elapsed = 0
	}
	tokens := b.tokens + elapsed*b.refillPerSec
	if tokens > b.capacity {
		tokens = b.capacity
	}
	return tokens
}
