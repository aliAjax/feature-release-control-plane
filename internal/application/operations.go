package application

import (
	"context"
	"fmt"
	"sort"
	"strings"
	"time"

	"github.com/example/feature-release-control-plane/internal/audit"
	"github.com/example/feature-release-control-plane/internal/configdomain"
	"github.com/example/feature-release-control-plane/internal/platform/httpx"
	"github.com/example/feature-release-control-plane/internal/releasedomain"
)

// AuditQuery is the application-level representation of an audit search. It
// mirrors the storage query without exposing repository types to
// transports or callers embedding the service.
type AuditQuery struct {
	Scope    string
	Actor    string
	Action   string
	Resource string
	From     *time.Time
	To       *time.Time
}

func (q AuditQuery) Validate() error {
	if q.From != nil && q.To != nil && !q.From.Before(*q.To) {
		return fmt.Errorf("%w: audit from must be before to", httpx.ErrBadRequest)
	}
	if len(q.Action) > 128 || len(q.Actor) > 128 || len(q.Resource) > 256 || len(q.Scope) > 512 {
		return fmt.Errorf("%w: audit filter is too long", httpx.ErrBadRequest)
	}
	return nil
}

type AuditPage struct {
	Items      []audit.Record `json:"items"`
	Page       int            `json:"page"`
	PageSize   int            `json:"page_size"`
	Total      int            `json:"total"`
	ChainValid bool           `json:"chain_valid"`
}

// QueryAudit provides a paginated, filterable view while checking the complete
// chain for the selected scope. Verifying only the returned page would miss a
// tampered record immediately before that page.
func (s *Service) QueryAudit(ctx context.Context, q AuditQuery, page, size int) (AuditPage, error) {
	if err := q.Validate(); err != nil {
		return AuditPage{}, err
	}
	page, size = normalizePage(page, size)
	items, total, err := s.audits.ListAuditFiltered(ctx, audit.Query{
		Scope: q.Scope, Actor: q.Actor, Action: q.Action, Resource: q.Resource, From: q.From, To: q.To,
	}, page, size)
	if err != nil {
		return AuditPage{}, err
	}
	chain, err := s.audits.ListAuditAll(ctx)
	if err != nil {
		return AuditPage{}, err
	}
	if q.Scope != "" {
		filtered := chain[:0]
		for _, record := range chain {
			if record.Scope == q.Scope {
				filtered = append(filtered, record)
			}
		}
		chain = filtered
	}
	return AuditPage{Items: items, Page: page, PageSize: size, Total: total, ChainValid: audit.Verify(chain)}, nil
}

func normalizePage(page, size int) (int, int) {
	if page < 1 {
		page = 1
	}
	if size < 1 {
		size = 50
	}
	if size > 500 {
		size = 500
	}
	return page, size
}

type RuleSummary struct {
	RuleID       string `json:"rule_id"`
	Priority     int    `json:"priority"`
	Enabled      bool   `json:"enabled"`
	Experiment   string `json:"experiment,omitempty"`
	MatchReason  string `json:"match_reason"`
	Selected     bool   `json:"selected"`
	Bucket       int    `json:"bucket,omitempty"`
	WindowActive bool   `json:"window_active"`
}

type VersionExplanation struct {
	ConfigID       string                    `json:"config_id"`
	ConfigKey      string                    `json:"config_key"`
	Version        int64                     `json:"version"`
	State          configdomain.VersionState `json:"state"`
	ValueDigest    string                    `json:"value_digest"`
	Evaluation     configdomain.Evaluation   `json:"evaluation"`
	Rules          []RuleSummary             `json:"rules"`
	Dependencies   []configdomain.Dependency `json:"dependencies"`
	CheckedAt      time.Time                 `json:"checked_at"`
	Valid          bool                      `json:"valid"`
	ValidationErrs []string                  `json:"validation_errors,omitempty"`
}

// ExplainVersion evaluates one exact immutable version. This is useful for
// rollout previews because /evaluate only exposes published
// versions.
func (s *Service) ExplainVersion(ctx context.Context, id string, number int64, subject configdomain.Subject) (VersionExplanation, error) {
	if number < 1 {
		return VersionExplanation{}, fmt.Errorf("%w: version must be positive", httpx.ErrBadRequest)
	}
	c, err := s.configs.Get(ctx, id)
	if err != nil {
		return VersionExplanation{}, err
	}
	v, err := s.configs.GetVersion(ctx, id, number)
	if err != nil {
		return VersionExplanation{}, err
	}
	if subject.TenantID != "" && subject.TenantID != c.Scope.TenantID {
		return VersionExplanation{}, fmt.Errorf("%w: subject tenant does not match scope", httpx.ErrForbidden)
	}
	now := s.clock.Now()
	evaluation := configdomain.Evaluate(v.Rules, subject, now)
	out := VersionExplanation{
		ConfigID: id, ConfigKey: c.Key, Version: number, State: v.State,
		ValueDigest: v.Value.Digest(), Evaluation: evaluation,
		Rules:        make([]RuleSummary, 0, len(evaluation.Explanations)),
		Dependencies: append([]configdomain.Dependency(nil), v.Dependencies...), CheckedAt: now, Valid: true,
	}
	if err := v.Validate(); err != nil {
		out.Valid = false
		out.ValidationErrs = append(out.ValidationErrs, err.Error())
	}
	if err := s.validateDependencies(ctx, c, v); err != nil {
		out.Valid = false
		out.ValidationErrs = append(out.ValidationErrs, err.Error())
	}
	for _, explanation := range evaluation.Explanations {
		rule, ok := findRule(v.Rules, explanation.RuleID)
		if !ok {
			continue
		}
		out.Rules = append(out.Rules, RuleSummary{
			RuleID: rule.ID, Priority: rule.Priority, Enabled: rule.Enabled,
			Experiment: rule.Experiment, MatchReason: explanation.Reason,
			Selected: explanation.Matched, Bucket: explanation.Bucket,
			WindowActive: explanation.Reason != "window_not_started" && explanation.Reason != "window_ended",
		})
	}
	return out, nil
}

func findRule(rules []configdomain.TargetRule, id string) (configdomain.TargetRule, bool) {
	for _, rule := range rules {
		if rule.ID == id {
			return rule, true
		}
	}
	return configdomain.TargetRule{}, false
}

type BatchEvaluateRequest struct {
	Scope    configdomain.Scope     `json:"scope"`
	Subjects []configdomain.Subject `json:"subjects"`
}

type BatchEvaluateItem struct {
	Index   int                  `json:"index"`
	Subject configdomain.Subject `json:"subject"`
	Result  *EvaluateResponse    `json:"result,omitempty"`
	Error   string               `json:"error,omitempty"`
}

type BatchEvaluateResponse struct {
	Items       []BatchEvaluateItem `json:"items"`
	EvaluatedAt time.Time           `json:"evaluated_at"`
}

// EvaluateBatch keeps the per-subject result isolated. One malformed subject
// therefore does not hide successful evaluations for the rest of a request.
func (s *Service) EvaluateBatch(ctx context.Context, in BatchEvaluateRequest) (BatchEvaluateResponse, error) {
	if err := in.Scope.Validate(); err != nil {
		return BatchEvaluateResponse{}, err
	}
	if len(in.Subjects) == 0 || len(in.Subjects) > 100 {
		return BatchEvaluateResponse{}, fmt.Errorf("%w: subjects must contain 1 to 100 entries", httpx.ErrBadRequest)
	}
	out := BatchEvaluateResponse{Items: make([]BatchEvaluateItem, 0, len(in.Subjects)), EvaluatedAt: s.clock.Now()}
	for index, subject := range in.Subjects {
		item := BatchEvaluateItem{Index: index, Subject: subject}
		result, err := s.Evaluate(ctx, EvaluateRequest{Scope: in.Scope, Subject: subject})
		if err != nil {
			item.Error = err.Error()
		} else {
			item.Result = &result
		}
		out.Items = append(out.Items, item)
	}
	return out, nil
}

type HealthAssessment struct {
	ReleaseID      string              `json:"release_id"`
	State          releasedomain.State `json:"state"`
	CurrentWave    int                 `json:"current_wave"`
	Samples        int                 `json:"samples"`
	Failures       int                 `json:"failures"`
	FailureRate    float64             `json:"failure_rate"`
	AllowedRate    float64             `json:"allowed_rate"`
	ShouldPause    bool                `json:"should_pause"`
	Recommendation string              `json:"recommendation"`
	ObservedAt     time.Time           `json:"observed_at"`
}

func validateHealthGate(g releasedomain.HealthGate) error {
	if g.Samples < 1 {
		return fmt.Errorf("%w: health samples must be positive", httpx.ErrBadRequest)
	}
	if g.Failures < 0 || g.Failures > g.Samples {
		return fmt.Errorf("%w: health failures must be between zero and samples", httpx.ErrBadRequest)
	}
	return nil
}

func (s *Service) AssessReleaseHealth(ctx context.Context, id string, gate releasedomain.HealthGate) (HealthAssessment, error) {
	if err := validateHealthGate(gate); err != nil {
		return HealthAssessment{}, err
	}
	r, err := s.releases.Get(ctx, id)
	if err != nil {
		return HealthAssessment{}, err
	}
	allowed := 0.0
	if r.CurrentWave >= 0 && r.CurrentWave < len(r.Waves) {
		allowed = r.Waves[r.CurrentWave].MaxFailureRate
	}
	shouldPause := r.ShouldPause(gate)
	recommendation := "continue"
	if shouldPause {
		recommendation = "pause_release_and_investigate"
	} else if r.State != releasedomain.Running {
		recommendation = "no_action_release_not_running"
	}
	return HealthAssessment{
		ReleaseID: id, State: r.State, CurrentWave: r.CurrentWave,
		Samples: gate.Samples, Failures: gate.Failures, FailureRate: gate.FailureRate(),
		AllowedRate: allowed, ShouldPause: shouldPause && r.State == releasedomain.Running,
		Recommendation: recommendation, ObservedAt: s.clock.Now(),
	}, nil
}

// ApplyHealthGate atomically applies the operational recommendation when the
// caller supplies a fresh fencing token. A non-running release is only
// assessed, never transitioned implicitly.
func (s *Service) ApplyHealthGate(ctx context.Context, actor, id string, gate releasedomain.HealthGate, token int64, reason string) (HealthAssessment, releasedomain.Release, error) {
	assessment, err := s.AssessReleaseHealth(ctx, id, gate)
	if err != nil {
		return assessment, releasedomain.Release{}, err
	}
	r, err := s.releases.Get(ctx, id)
	if err != nil {
		return assessment, r, err
	}
	if !assessment.ShouldPause {
		return assessment, r, nil
	}
	if strings.TrimSpace(reason) == "" {
		reason = fmt.Sprintf("health gate failure rate %.4f exceeded %.4f", assessment.FailureRate, assessment.AllowedRate)
	}
	next, err := s.TransitionRelease(ctx, actor, id, releasedomain.Paused, reason, token)
	if err != nil {
		return assessment, r, err
	}
	return assessment, next, nil
}

// AggregateHealth combines independently collected samples for operators
// that report one gate per worker or availability zone.
func AggregateHealth(samples []releasedomain.HealthGate) (releasedomain.HealthGate, error) {
	if len(samples) == 0 {
		return releasedomain.HealthGate{}, fmt.Errorf("%w: at least one health sample is required", httpx.ErrBadRequest)
	}
	var total releasedomain.HealthGate
	for _, sample := range samples {
		if err := validateHealthGate(sample); err != nil {
			return releasedomain.HealthGate{}, err
		}
		total.Samples += sample.Samples
		total.Failures += sample.Failures
		if sample.At.After(total.At) {
			total.At = sample.At
		}
	}
	if total.At.IsZero() {
		total.At = time.Now().UTC()
	}
	return total, nil
}

// SortAuditByTime is used by export jobs after merging records from multiple
// scopes. Stable ordering keeps records with equal timestamps deterministic.
func SortAuditByTime(records []audit.Record) []audit.Record {
	out := append([]audit.Record(nil), records...)
	sort.SliceStable(out, func(i, j int) bool {
		if out[i].At.Equal(out[j].At) {
			return out[i].ID < out[j].ID
		}
		return out[i].At.Before(out[j].At)
	})
	return out
}
