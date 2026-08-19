package scheduler

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/example/feature-release-control-plane/internal/configdomain"
)

func fixedClock(at time.Time) Clock { return clockFunc(func() time.Time { return at }) }

type clockFunc func() time.Time

func (f clockFunc) Now() time.Time { return f() }

func TestScanGroupReturnsExpiredCandidate(t *testing.T) {
	now := time.Date(2026, 8, 20, 10, 0, 0, 0, time.UTC)
	expired := now.Add(-time.Minute)
	scanner := NewExpiryScanner(fixedClock(now))
	versions := []configdomain.ConfigVersion{
		{Number: 1, State: configdomain.Published, ExpiresAt: &expired},
		{Number: 2, State: configdomain.Published},
	}
	idx, out, err := scanner.ScanGroup(context.Background(), 0, versions)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if idx != 0 || len(out) != 1 || out[0].Number != 1 {
		t.Fatalf("expected one candidate, got idx=%d len=%d", idx, len(out))
	}
}

func TestScanAllStopsOnCancelledCtx(t *testing.T) {
	now := time.Date(2026, 8, 20, 10, 0, 0, 0, time.UTC)
	expired := now.Add(-time.Minute)
	scanner := NewExpiryScanner(fixedClock(now))
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	groups := [][]configdomain.ConfigVersion{
		{{Number: 1, State: configdomain.Published, ExpiresAt: &expired}},
		{{Number: 2, State: configdomain.Published, ExpiresAt: &expired}},
	}
	_, err := scanner.ScanAll(ctx, groups)
	if !errors.Is(err, context.Canceled) {
		t.Fatalf("expected context.Canceled, got %v", err)
	}
}
