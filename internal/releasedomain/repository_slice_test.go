package releasedomain

import (
	"context"
	"testing"
)

func TestCreateCopiesInputSlices(t *testing.T) {
	m := NewMemoryRepository()
	refs := []VersionRef{{ConfigID: "cfg-1", Version: 1}}
	waves := []Wave{{Number: 1, Percentage: 100, MaxFailureRate: 0.1}}
	r := Release{ID: "rel-1", Scope: "scope", VersionRefs: refs, Waves: waves, State: Pending, Revision: 1}
	if err := m.Create(context.Background(), r); err != nil {
		t.Fatal(err)
	}
	refs[0].ConfigID = "mutated"
	waves[0].Percentage = 50
	got, err := m.Get(context.Background(), "rel-1")
	if err != nil {
		t.Fatal(err)
	}
	if got.VersionRefs[0].ConfigID != "cfg-1" {
		t.Fatalf("stored VersionRefs was aliased: %+v", got.VersionRefs)
	}
	if got.Waves[0].Percentage != 100 {
		t.Fatalf("stored Waves was aliased: %+v", got.Waves)
	}
}

func TestGetCopiesStoredSlices(t *testing.T) {
	m := NewMemoryRepository()
	r := Release{
		ID: "rel-1", Scope: "scope",
		VersionRefs: []VersionRef{{ConfigID: "cfg-1", Version: 1}},
		Waves:       []Wave{{Number: 1, Percentage: 100, MaxFailureRate: 0.1}},
		State:       Pending, Revision: 1,
	}
	if err := m.Create(context.Background(), r); err != nil {
		t.Fatal(err)
	}
	got, err := m.Get(context.Background(), "rel-1")
	if err != nil {
		t.Fatal(err)
	}
	got.VersionRefs[0].ConfigID = "mutated"
	got.Waves[0].Percentage = 50
	again, err := m.Get(context.Background(), "rel-1")
	if err != nil {
		t.Fatal(err)
	}
	if again.VersionRefs[0].ConfigID != "cfg-1" || again.Waves[0].Percentage != 100 {
		t.Fatalf("stored release was mutated through Get: %+v", again)
	}
}
