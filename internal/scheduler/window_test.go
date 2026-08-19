package scheduler

import (
	"context"
	"errors"
	"testing"
	"time"
)

func TestScanActiveReturnsActiveWindows(t *testing.T) {
	at := time.Date(2026, 8, 20, 10, 0, 0, 0, time.UTC)
	start := at.Add(-time.Minute)
	end := at.Add(time.Minute)
	windows := []Window{
		{StartsAt: &start, EndsAt: &end},
		{},
	}
	out, err := ScanActive(context.Background(), windows, at)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(out) != 2 {
		t.Fatalf("expected 2 active windows, got %d", len(out))
	}
}

func TestScanActiveStopsOnCancelledCtx(t *testing.T) {
	at := time.Date(2026, 8, 20, 10, 0, 0, 0, time.UTC)
	start := at.Add(-time.Minute)
	windows := []Window{{StartsAt: &start}}
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	_, err := ScanActive(ctx, windows, at)
	if !errors.Is(err, context.Canceled) {
		t.Fatalf("expected context.Canceled, got %v", err)
	}
}
