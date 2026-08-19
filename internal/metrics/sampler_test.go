package metrics

import (
	"testing"
	"time"
)

func TestReservoirAddDoesNotPanic(t *testing.T) {
	r := NewSampler(4).NewReservoir(7)
	defer func() {
		if rec := recover(); rec != nil {
			t.Fatalf("Reservoir.Add panicked: %v", rec)
		}
	}()
	r.Add(Observation{ReleaseID: "rel-1", Success: true, Latency: time.Millisecond, At: time.Now()})
	if got := len(r.Items()); got != 1 {
		t.Fatalf("expected 1 item after Add, got %d", got)
	}
}
