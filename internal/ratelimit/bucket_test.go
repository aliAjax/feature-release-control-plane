package ratelimit

import (
	"errors"
	"testing"
)

func TestBucketAllowReturnsErrLimited(t *testing.T) {
	b, err := NewBucket(0.001, 1)
	if err != nil {
		t.Fatal(err)
	}
	if ok, err := b.Allow(); !ok || err != nil {
		t.Fatalf("first allow should succeed, got ok=%v err=%v", ok, err)
	}
	ok, err := b.Allow()
	if ok || !errors.Is(err, ErrLimited) {
		t.Fatalf("second allow should be limited, got ok=%v err=%v", ok, err)
	}
}

func TestNewBucketWrapsInvalidConfig(t *testing.T) {
	_, err := NewBucket(0, 1)
	if !errors.Is(err, ErrInvalidConfig) {
		t.Fatalf("expected wrapped ErrInvalidConfig, got %v", err)
	}
}
