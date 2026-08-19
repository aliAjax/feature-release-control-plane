package notify

import (
	"context"
	"time"
)

// RetryPolicy describes how transient delivery failures are retried. The
// policy is a value object: Dispatch copies it so concurrent deliveries never
// mutate shared retry state.
type RetryPolicy struct {
	MaxAttempts    int
	InitialBackoff time.Duration
	MaxBackoff     time.Duration
}

func (p RetryPolicy) normalized() RetryPolicy {
	if p.MaxAttempts < 1 {
		p.MaxAttempts = 1
	}
	if p.InitialBackoff <= 0 {
		p.InitialBackoff = 100 * time.Millisecond
	}
	if p.MaxBackoff < p.InitialBackoff {
		p.MaxBackoff = p.InitialBackoff
	}
	return p
}

// Do runs fn up to MaxAttempts times with exponential backoff. It stops
// immediately when ctx is cancelled, preserving the original error so callers
// can distinguish a genuine failure from an aborted attempt.
func (p RetryPolicy) Do(ctx context.Context, fn func(context.Context) error) error {
	p = p.normalized()
	var lastErr error
	for attempt := 0; attempt < p.MaxAttempts; attempt++ {
		if err := ctx.Err(); err != nil {
			return err
		}
		err := fn(ctx)
		if err == nil {
			return nil
		}
		lastErr = err
		if attempt == p.MaxAttempts-1 {
			break
		}
		backoff := p.InitialBackoff << attempt
		if backoff > p.MaxBackoff {
			backoff = p.MaxBackoff
		}
		select {
		case <-ctx.Done():
			return ctx.Err()
		case <-time.After(backoff):
		}
	}
	return lastErr
}
