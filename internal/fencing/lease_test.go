package fencing

import (
	"errors"
	"testing"
	"time"
)

func TestReleaseStaleOwnerFails(t *testing.T) {
	m := NewManager(NewStore(), time.Minute)
	m.Acquire("key-1", "owner-a")
	m.Acquire("key-1", "owner-b")
	err := m.Release("key-1", "owner-a", 1)
	if !errors.Is(err, ErrStaleLease) {
		t.Fatalf("releasing with a stale owner should fail, got %v", err)
	}
}

func TestRenewStaleVersionFails(t *testing.T) {
	m := NewManager(NewStore(), time.Minute)
	m.Acquire("key-1", "owner-a")
	m.Acquire("key-1", "owner-b")
	_, err := m.Renew("key-1", "owner-a", 1)
	if !errors.Is(err, ErrStaleLease) {
		t.Fatalf("renewing a stale version should fail, got %v", err)
	}
}
