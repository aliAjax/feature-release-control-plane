package metrics

import "testing"

func TestStoreReservoirDoesNotPanic(t *testing.T) {
	s := NewStore(NewSampler(4), 7)
	defer func() {
		if rec := recover(); rec != nil {
			t.Fatalf("Store.Reservoir panicked: %v", rec)
		}
	}()
	r := s.Reservoir("rel-1")
	if r == nil {
		t.Fatal("expected a reservoir for rel-1")
	}
	if s.Reservoir("rel-1") != r {
		t.Fatal("expected the same reservoir for the same release")
	}
}
