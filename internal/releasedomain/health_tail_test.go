package releasedomain

import (
	"testing"
	"time"
)

func TestTailIsolationAfterCompaction(t *testing.T) {
	w := &HealthWindow{max: 3}
	base := time.Now()
	_ = w.Record(Observation{ReleaseID: "rel-1", TenantID: "tenant1", Success: true, At: base, Latency: time.Millisecond})
	_ = w.Record(Observation{ReleaseID: "rel-1", TenantID: "tenant1", Success: true, At: base.Add(time.Second), Latency: time.Millisecond})
	_ = w.Record(Observation{ReleaseID: "rel-1", TenantID: "tenant1", Success: true, At: base.Add(2 * time.Second), Latency: time.Millisecond})

	tail := w.Tail(2)
	firstAt := tail[0].At
	_ = w.Record(Observation{ReleaseID: "rel-1", TenantID: "tenant1", Success: true, At: base.Add(3 * time.Second), Latency: time.Millisecond})

	if len(tail) != 2 || tail[0].At != firstAt {
		t.Fatalf("tail mutated after compaction: len=%d first=%v", len(tail), tail[0].At)
	}
}
