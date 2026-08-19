package metrics

import (
	"testing"
	"time"
)

func TestSummariesReturnsMetricsForEachRelease(t *testing.T) {
	obs := []Observation{
		{ReleaseID: "rel-a", Success: true, Latency: time.Millisecond, At: time.Now()},
		{ReleaseID: "rel-b", Success: false, Latency: 2 * time.Millisecond, At: time.Now()},
	}
	out := NewAggregator().Summaries(obs)
	if len(out) != 2 {
		t.Fatalf("expected 2 release summaries, got %d", len(out))
	}
	if out["rel-a"].Samples != 1 || out["rel-b"].Samples != 1 {
		t.Fatalf("unexpected summaries: %+v", out)
	}
}
