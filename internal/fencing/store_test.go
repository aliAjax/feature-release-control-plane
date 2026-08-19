package fencing

import (
	"errors"
	"testing"
	"time"
)

func TestPutRejectsStaleWrite(t *testing.T) {
	s := NewStore()
	expiry := time.Now().Add(time.Minute)
	if err := s.Put(Lease{Key: "key-1", Owner: "owner-a", Version: 1, ExpiresAt: expiry}); err != nil {
		t.Fatal(err)
	}
	err := s.Put(Lease{Key: "key-1", Owner: "owner-a", Version: 1, ExpiresAt: expiry})
	if !errors.Is(err, ErrStaleLease) {
		t.Fatalf("stale Put should fail, got %v", err)
	}
}

func TestDeleteStaleOwnerFails(t *testing.T) {
	s := NewStore()
	expiry := time.Now().Add(time.Minute)
	if err := s.Put(Lease{Key: "key-1", Owner: "owner-a", Version: 1, ExpiresAt: expiry}); err != nil {
		t.Fatal(err)
	}
	err := s.Delete("key-1", "owner-b", 1)
	if !errors.Is(err, ErrStaleLease) {
		t.Fatalf("deleting with a stale owner should fail, got %v", err)
	}
}
