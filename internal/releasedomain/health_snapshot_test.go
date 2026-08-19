package releasedomain

import (
	"testing"
	"time"
)

func TestSnapshotIsolationAfterCompaction(t *testing.T) {
	w := &HealthWindow{max: 3}
	base := time.Now()
	_ = w.Record(Observation{ReleaseID: "rel-1", TenantID: "tenant1", Success: true, At: base, Latency: time.Millisecond})
	_ = w.Record(Observation{ReleaseID: "rel-1", TenantID: "tenant1", Success: true, At: base.Add(time.Second), Latency: time.Millisecond})
	_ = w.Record(Observation{ReleaseID: "rel-1", TenantID: "tenant1", Success: true, At: base.Add(2 * time.Second), Latency: time.Millisecond})

	snap := w.Snapshot("rel-1")
	if len(snap) != 3 {
		t.Fatalf("expected 3 snapshots, got %d", len(snap))
	}
	firstAt := snap[0].At

	// Trigger compaction by adding a fourth observation.
	_ = w.Record(Observation{ReleaseID: "rel-1", TenantID: "tenant1", Success: true, At: base.Add(3 * time.Second), Latency: time.Millisecond})

	if len(snap) != 3 || snap[0].At != firstAt {
		t.Fatalf("snapshot mutated after compaction: len=%d first=%v", len(snap), snap[0].At)
	}
}
