package scheduler

import (
	"context"
	"time"

	"github.com/example/feature-release-control-plane/internal/configdomain"
)

// Clock is the time source used by the expiry scanner. Tests inject a frozen
// clock to make the scan deterministic.
type Clock interface{ Now() time.Time }

type systemClock struct{}

func (systemClock) Now() time.Time { return time.Now().UTC() }

// ExpiryScanner finds configuration versions whose ExpiresAt has passed but
// that are still in an active, non-expired state. It is read-only: it returns
// the candidates and lets the caller decide how to transition them, so the
// scan itself can run safely under a read lock.
type ExpiryScanner struct {
	clock Clock
}

func NewExpiryScanner(clock Clock) *ExpiryScanner {
	if clock == nil {
		clock = systemClock{}
	}
	return &ExpiryScanner{clock: clock}
}

var activeStates = map[configdomain.VersionState]bool{
	configdomain.Draft:      true,
	configdomain.Reviewing:  true,
	configdomain.Scheduled:  true,
	configdomain.RollingOut: true,
	configdomain.Paused:     true,
	configdomain.Published:  true,
}

// Scan returns the versions that are past their expiry but not yet marked
// expired. It performs a shallow copy so callers never mutate the input.
func (s *ExpiryScanner) Scan(versions []configdomain.ConfigVersion) []configdomain.ConfigVersion {
	now := s.clock.Now()
	out := make([]configdomain.ConfigVersion, 0)
	for _, v := range versions {
		if v.ExpiresAt == nil || !v.ExpiresAt.Before(now) {
			continue
		}
		if !activeStates[v.State] {
			continue
		}
		out = append(out, v)
	}
	return out
}

// ScanGroup walks a single group and reports expiry candidates with their
// group index. It is the cancellation-aware variant used by the worker loop:
// the caller can abort a long scan without losing the candidates already found.
func (s *ExpiryScanner) ScanGroup(ctx context.Context, index int, versions []configdomain.ConfigVersion) (int, []configdomain.ConfigVersion, error) {
	for _, v := range versions {
		if err := ctx.Err(); err != nil {
			return index, nil, err
		}
		if v.ExpiresAt != nil && v.ExpiresAt.Before(s.clock.Now()) && activeStates[v.State] {
			return index, []configdomain.ConfigVersion{v}, nil
		}
	}
	return index, nil, nil
}

// ScanAll aggregates expiry candidates across many groups, aborting early when
// ctx is cancelled. The returned map is keyed by group index and contains only
// groups that produced at least one candidate.
func (s *ExpiryScanner) ScanAll(ctx context.Context, groups [][]configdomain.ConfigVersion) (map[int][]configdomain.ConfigVersion, error) {
	result := make(map[int][]configdomain.ConfigVersion)
	for i, group := range groups {
		idx, candidates, err := s.ScanGroup(ctx, i, group)
		if err != nil {
			return result, err
		}
		if len(candidates) > 0 {
			result[idx] = candidates
		}
	}
	return result, nil
}
