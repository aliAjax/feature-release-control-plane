package ratelimit

import (
	"errors"
	"testing"
)

func TestRegistryAllowReturnsErrLimited(t *testing.T) {
	r, err := NewRegistry(0.001, 1)
	if err != nil {
		t.Fatal(err)
	}
	if ok, err := r.Allow("tenant-a"); !ok || err != nil {
		t.Fatalf("first allow should succeed, got ok=%v err=%v", ok, err)
	}
	ok, err := r.Allow("tenant-a")
	if ok || !errors.Is(err, ErrLimited) {
		t.Fatalf("second allow should be limited, got ok=%v err=%v", ok, err)
	}
}

func TestResetUnknownKeyReturnsFalse(t *testing.T) {
	r, _ := NewRegistry(1, 2)
	if r.Reset("missing") {
		t.Fatal("Reset on an unknown key should report false")
	}
}
