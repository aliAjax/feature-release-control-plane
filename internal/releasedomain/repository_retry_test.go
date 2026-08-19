package releasedomain

import (
	"context"
	"testing"
)

func TestMemoryRepositoryUpdatePersistsState(t *testing.T) {
	m := NewMemoryRepository()
	r := Release{ID: "rel-1", Scope: "scope", VersionRefs: []VersionRef{{ConfigID: "c", Version: 1}}, Waves: []Wave{{Number: 1, Percentage: 100, MaxFailureRate: 0.1}}, State: Pending, Revision: 1}
	if err := m.Create(context.Background(), r); err != nil {
		t.Fatal(err)
	}
	r.State = Running
	if err := m.Update(context.Background(), r, 1); err != nil {
		t.Fatal(err)
	}
	got, err := m.Get(context.Background(), "rel-1")
	if err != nil {
		t.Fatal(err)
	}
	if got.State != Running {
		t.Fatalf("expected Running after Update, got %s", got.State)
	}
}
