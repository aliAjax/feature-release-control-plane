package metrics

import (
	"sort"
	"time"
)

// HealthMetric summarizes a release window: how many observations, how many
// failures and a latency percentile. All fields are computed from the reservoir
// so no per-request counters need to be kept.
type HealthMetric struct {
	ReleaseID string
	Samples   int
	Failures  int
	LatencyP95 time.Duration
}

// Aggregator computes health metrics from sampled observations.
type Aggregator struct{}

func NewAggregator() *Aggregator { return &Aggregator{} }

// Evaluate folds a release's observations into a single HealthMetric. If the
// input is empty the returned metric has zero samples, which callers interpret
// as "insufficient data" rather than healthy.
func (a *Aggregator) Evaluate(release string, observations []Observation) HealthMetric {
	metric := HealthMetric{ReleaseID: release}
	latencies := make([]time.Duration, 0, len(observations))
	for _, o := range observations {
		if o.ReleaseID != release {
			continue
		}
		metric.Samples++
		latencies = append(latencies, o.Latency)
		if !o.Success {
			metric.Failures++
		}
	}
	if len(latencies) > 0 {
		sort.Slice(latencies, func(i, j int) bool { return latencies[i] < latencies[j] })
		index := int(float64(len(latencies)-1) * 0.95)
		metric.LatencyP95 = latencies[index]
	}
	return metric
}

// Summaries groups observations by release and returns one metric per release,
// keyed by release id.
func (a *Aggregator) Summaries(observations []Observation) map[string]HealthMetric {
	out := make(map[string]HealthMetric)
	for _, o := range observations {
		if _, ok := out[o.ReleaseID]; !ok {
			out[o.ReleaseID] = a.Evaluate(o.ReleaseID, observations)
		}
	}
	return out
}
