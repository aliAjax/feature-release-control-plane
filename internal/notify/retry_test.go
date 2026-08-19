package notify

import (
	"context"
	"errors"
	"testing"
	"time"
)

func TestRetryReturnsErrorAfterAllAttempts(t *testing.T) {
	p := RetryPolicy{MaxAttempts: 3, InitialBackoff: time.Millisecond, MaxBackoff: time.Millisecond}
	calls := 0
	err := p.Do(context.Background(), func(ctx context.Context) error {
		calls++
		return errors.New("boom")
	})
	if err == nil {
		t.Fatal("expected a non-nil error after all attempts fail")
	}
	if calls != 3 {
		t.Fatalf("expected 3 attempts, got %d", calls)
	}
}

func TestRetryHonorsCancelledContext(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	p := RetryPolicy{MaxAttempts: 3, InitialBackoff: time.Millisecond, MaxBackoff: time.Millisecond}
	calls := 0
	err := p.Do(ctx, func(ctx context.Context) error {
		calls++
		return nil
	})
	if !errors.Is(err, context.Canceled) {
		t.Fatalf("expected context.Canceled, got %v", err)
	}
	if calls != 0 {
		t.Fatalf("cancelled context must not invoke the callback, got %d calls", calls)
	}
}
