package releasedomain

import (
	"errors"
	"fmt"
	"time"
)

type State string

const (
	Pending    State = "pending"
	Running    State = "running"
	Paused     State = "paused"
	Succeeded  State = "succeeded"
	Failed     State = "failed"
	RolledBack State = "rolled_back"
	Cancelled  State = "cancelled"
	Retrying   State = "retrying"
)

var ErrTransition = errors.New("invalid release transition")

type Wave struct {
	Number          int           `json:"number"`
	Percentage      int           `json:"percentage"`
	MaxFailureRate  float64       `json:"max_failure_rate"`
	MinimumDuration time.Duration `json:"minimum_duration"`
}

func (w Wave) Validate() error {
	if w.Number < 1 || w.Percentage < 1 || w.Percentage > 100 || w.MaxFailureRate < 0 || w.MaxFailureRate > 1 {
		return errors.New("invalid release wave")
	}
	return nil
}

type Release struct {
	ID             string       `json:"id"`
	Scope          string       `json:"scope"`
	VersionRefs    []VersionRef `json:"version_refs"`
	Waves          []Wave       `json:"waves"`
	State          State        `json:"state"`
	CurrentWave    int          `json:"current_wave"`
	FencingToken   int64        `json:"fencing_token"`
	IdempotencyKey string       `json:"idempotency_key"`
	CreatedAt      time.Time    `json:"created_at"`
	UpdatedAt      time.Time    `json:"updated_at"`
	PausedReason   string       `json:"paused_reason,omitempty"`
	RollbackReason string       `json:"rollback_reason,omitempty"`
	Revision       int64        `json:"revision"`
}
type VersionRef struct {
	ConfigID string `json:"config_id"`
	Version  int64  `json:"version"`
}

func (r Release) Validate() error {
	if r.ID == "" || r.Scope == "" || len(r.VersionRefs) == 0 || len(r.Waves) == 0 {
		return errors.New("release requires id, scope, versions, and waves")
	}
	total := 0
	for _, w := range r.Waves {
		if err := w.Validate(); err != nil {
			return err
		}
		total += w.Percentage
	}
	if total != 100 {
		return fmt.Errorf("release waves must total 100, got %d", total)
	}
	return nil
}
func (r Release) Can(to State) bool {
	allowed := map[State]map[State]bool{Pending: {Running: true, Cancelled: true}, Running: {Paused: true, Succeeded: true, Failed: true, RolledBack: true}, Paused: {Running: true, Cancelled: true, RolledBack: true}, Succeeded: {RolledBack: true}, Failed: {Retrying: true, RolledBack: true}, RolledBack: {}, Cancelled: {}, Retrying: {Running: true, Cancelled: true}}
	return allowed[r.State][to]
}
func (r Release) Transition(to State, token int64, now time.Time) (Release, error) {
	if token <= r.FencingToken {
		return r, fmt.Errorf("%w: stale fencing token", ErrTransition)
	}
	if !r.Can(to) {
		return r, fmt.Errorf("%w: %s -> %s", ErrTransition, r.State, to)
	}
	r.State = to
	r.FencingToken = token
	r.Revision++
	r.UpdatedAt = now
	return r, nil
}
func (r Release) WaveForPercent(percent int) int {
	total := 0
	for i, w := range r.Waves {
		total += w.Percentage
		if percent < total {
			return i
		}
	}
	return len(r.Waves) - 1
}

type HealthGate struct {
	Samples  int       `json:"samples"`
	Failures int       `json:"failures"`
	At       time.Time `json:"at"`
}

func (h HealthGate) FailureRate() float64 {
	if h.Samples == 0 {
		return 0
	}
	return float64(h.Failures) / float64(h.Samples)
}
func (r Release) ShouldPause(h HealthGate) bool {
	if r.CurrentWave >= len(r.Waves) {
		return false
	}
	return h.FailureRate() > r.Waves[r.CurrentWave].MaxFailureRate
}
