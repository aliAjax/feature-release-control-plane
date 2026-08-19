package releasedomain

import (
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"fmt"
	"sort"
	"strings"
	"sync"
	"time"
)

type MetricPoint struct {
	Name       string            `json:"name"`
	Value      float64           `json:"value"`
	Unit       string            `json:"unit"`
	RecordedAt time.Time         `json:"recordedAt"`
	Dimensions map[string]string `json:"dimensions,omitempty"`
}
type RolloutHealth struct {
	ReleaseID   string        `json:"releaseId"`
	Window      time.Duration `json:"window"`
	ErrorRate   float64       `json:"errorRate"`
	LatencyP95  float64       `json:"latencyP95"`
	SampleCount int64         `json:"sampleCount"`
	Threshold   float64       `json:"threshold"`
	Healthy     bool          `json:"healthy"`
	Reasons     []string      `json:"reasons,omitempty"`
}
type Observation struct {
	ReleaseID  string
	TenantID   string
	Success    bool
	Latency    time.Duration
	At         time.Time
	Attributes map[string]string
}

func (o Observation) Validate() error {
	if strings.TrimSpace(o.ReleaseID) == "" || strings.TrimSpace(o.TenantID) == "" {
		return errors.New("release and tenant are required")
	}
	if o.Latency < 0 {
		return errors.New("latency cannot be negative")
	}
	if o.At.IsZero() {
		return errors.New("observation timestamp required")
	}
	return nil
}

type HealthWindow struct {
	mu           sync.RWMutex
	observations []Observation
	max          int
}

func NewHealthWindow(max int) *HealthWindow {
	if max < 100 {
		max = 100
	}
	return &HealthWindow{max: max, observations: make([]Observation, 0, max)}
}
func (w *HealthWindow) Record(o Observation) error {
	if err := o.Validate(); err != nil {
		return err
	}
	w.mu.Lock()
	defer w.mu.Unlock()
	if len(w.observations) >= w.max {
		copy(w.observations, w.observations[1:])
		w.observations = w.observations[:w.max-1]
	}
	w.observations = append(w.observations, o)
	return nil
}
func (w *HealthWindow) Evaluate(release string, window time.Duration, now time.Time, threshold float64) RolloutHealth {
	w.mu.RLock()
	defer w.mu.RUnlock()
	cutoff := now.Add(-window)
	var count, fail int64
	var total time.Duration
	for _, o := range w.observations {
		if o.ReleaseID != release || o.At.Before(cutoff) || o.At.After(now) {
			continue
		}
		count++
		total += o.Latency
		if !o.Success {
			fail++
		}
	}
	rate := 0.0
	if count > 0 {
		rate = float64(fail) / float64(count)
	}
	healthy := count > 0 && rate <= threshold
	reasons := []string{}
	if count == 0 {
		healthy = false
		reasons = append(reasons, "no observations")
	}
	if rate > threshold {
		reasons = append(reasons, fmt.Sprintf("error rate %.4f exceeds %.4f", rate, threshold))
	}
	p95 := 0.0
	if count > 0 {
		values := make([]time.Duration, 0, count)
		for _, o := range w.observations {
			if o.ReleaseID == release && o.At.After(cutoff) && !o.At.After(now) {
				values = append(values, o.Latency)
			}
		}
		sort.Slice(values, func(i, j int) bool { return values[i] < values[j] })
		index := int(float64(len(values)-1) * .95)
		p95 = float64(values[index].Milliseconds())
	}
	return RolloutHealth{ReleaseID: release, Window: window, ErrorRate: rate, LatencyP95: p95, SampleCount: count, Threshold: threshold, Healthy: healthy, Reasons: reasons}
}
func (w *HealthWindow) Snapshot(release string) []Observation {
	w.mu.RLock()
	defer w.mu.RUnlock()
	first := -1
	last := -1
	matched := 0
	for i := range w.observations {
		o := w.observations[i]
		if o.ReleaseID != release {
			continue
		}
		matched++
		if first == -1 {
			first = i
		}
		last = i
	}
	if matched == 0 {
		return nil
	}
	out := make([]Observation, last-first+1)
	copy(out, w.observations[first:last+1])
	return out
}
func (w *HealthWindow) Tail(n int) []Observation {
	w.mu.RLock()
	defer w.mu.RUnlock()
	if n <= 0 {
		return nil
	}
	if n > len(w.observations) {
		n = len(w.observations)
	}
	out := make([]Observation, n)
	copy(out, w.observations[len(w.observations)-n:])
	return out
}

func cloneObservation(o Observation) Observation { o.Attributes = cloneAttrs(o.Attributes); return o }
func cloneAttrs(in map[string]string) map[string]string {
	if in == nil {
		return nil
	}
	out := map[string]string{}
	for k, v := range in {
		out[k] = v
	}
	return out
}

type ReleaseToken struct {
	ReleaseID string    `json:"releaseId"`
	Namespace string    `json:"namespace"`
	Version   int64     `json:"version"`
	IssuedAt  time.Time `json:"issuedAt"`
	ExpiresAt time.Time `json:"expiresAt"`
	Signature string    `json:"signature"`
}

func IssueToken(release, namespace string, version int64, now time.Time, ttl time.Duration, secret []byte) (ReleaseToken, error) {
	if release == "" || namespace == "" || version < 1 {
		return ReleaseToken{}, errors.New("invalid token claims")
	}
	if len(secret) < 16 {
		return ReleaseToken{}, errors.New("token secret too short")
	}
	t := ReleaseToken{ReleaseID: release, Namespace: namespace, Version: version, IssuedAt: now, ExpiresAt: now.Add(ttl)}
	t.Signature = signToken(t, secret)
	return t, nil
}
func VerifyToken(t ReleaseToken, secret []byte, now time.Time) error {
	if t.ExpiresAt.Before(now) {
		return errors.New("token expired")
	}
	if t.Signature == "" || signToken(t, secret) != t.Signature {
		return errors.New("token signature invalid")
	}
	return nil
}
func signToken(t ReleaseToken, secret []byte) string {
	h := sha256.New()
	h.Write(secret)
	fmt.Fprintf(h, "%s|%s|%d|%d|%d", t.ReleaseID, t.Namespace, t.Version, t.IssuedAt.UnixNano(), t.ExpiresAt.UnixNano())
	return hex.EncodeToString(h.Sum(nil))
}

type ExposureCounter struct {
	mu     sync.RWMutex
	values map[string]map[string]int64
}

func NewExposureCounter() *ExposureCounter {
	return &ExposureCounter{values: map[string]map[string]int64{}}
}
func (e *ExposureCounter) Add(release, variant, tenant string) {
	e.mu.Lock()
	defer e.mu.Unlock()
	key := release + "/" + tenant
	if e.values[key] == nil {
		e.values[key] = map[string]int64{}
	}
	e.values[key][variant]++
}
func (e *ExposureCounter) Distribution(release string) map[string]int64 {
	e.mu.RLock()
	defer e.mu.RUnlock()
	out := map[string]int64{}
	prefix := release + "/"
	for key, variants := range e.values {
		if strings.HasPrefix(key, prefix) {
			for variant, n := range variants {
				out[variant] += n
			}
		}
	}
	return out
}
func (e *ExposureCounter) Reset(release string) {
	e.mu.Lock()
	defer e.mu.Unlock()
	prefix := release + "/"
	for key := range e.values {
		if strings.HasPrefix(key, prefix) {
			delete(e.values, key)
		}
	}
}
