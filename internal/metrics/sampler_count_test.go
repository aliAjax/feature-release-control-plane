package metrics

import "testing"

func TestReservoirCountDoesNotPanic(t *testing.T) {
	r := NewSampler(4).NewReservoir(7)
	defer func() {
		if rec := recover(); rec != nil {
			t.Fatalf("Reservoir.Count panicked: %v", rec)
		}
	}()
	r.Count("rel-1")
	r.Count("rel-1")
	if got := r.Count("rel-1"); got != 3 {
		t.Fatalf("expected count 3, got %d", got)
	}
}
