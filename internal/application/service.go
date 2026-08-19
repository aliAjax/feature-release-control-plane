package application

import (
	"context"
	"crypto/hmac"
	"crypto/sha256"
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"sort"
	"strings"
	"sync"
	"sync/atomic"
	"time"

	"github.com/example/feature-release-control-plane/internal/audit"
	"github.com/example/feature-release-control-plane/internal/configdomain"
	"github.com/example/feature-release-control-plane/internal/platform/httpx"
	"github.com/example/feature-release-control-plane/internal/releasedomain"
	"github.com/example/feature-release-control-plane/internal/repository"
)

type Clock interface{ Now() time.Time }
type systemClock struct{}

func (systemClock) Now() time.Time { return time.Now().UTC() }

type Service struct {
	configs  repository.ConfigRepository
	audits   repository.AuditRepository
	outbox   repository.Outbox
	releases releasedomain.Repository
	secret   []byte
	clock    Clock
	sequence atomic.Int64
	streamMu sync.Mutex
	streams  map[chan Change]struct{}
}

func New(configs repository.ConfigRepository, audits repository.AuditRepository, outbox repository.Outbox, releases releasedomain.Repository, secret string) *Service {
	return &Service{configs: configs, audits: audits, outbox: outbox, releases: releases, secret: []byte(secret), clock: systemClock{}, streams: map[chan Change]struct{}{}}
}

type Change struct {
	Cursor   int64     `json:"cursor"`
	Type     string    `json:"type"`
	Scope    string    `json:"scope"`
	Resource string    `json:"resource"`
	Version  int64     `json:"version"`
	At       time.Time `json:"at"`
}
type CreateConfig struct {
	Scope       configdomain.Scope `json:"scope"`
	Key         string             `json:"key"`
	Description string             `json:"description"`
}

func (s *Service) CreateConfig(ctx context.Context, actor string, in CreateConfig) (configdomain.Config, error) {
	if err := in.Scope.Validate(); err != nil {
		return configdomain.Config{}, err
	}
	c := configdomain.Config{ID: s.id("cfg"), Scope: in.Scope, Key: in.Key, Description: in.Description, CreatedAt: s.clock.Now(), UpdatedAt: s.clock.Now(), Version: 1}
	if err := c.Validate(); err != nil {
		return c, err
	}
	if err := s.configs.Create(ctx, c); err != nil {
		return c, err
	}
	s.audit(ctx, c.Scope.Key(), actor, "config.created", c.ID, c)
	s.emit(ctx, "config.created", c.Scope.Key(), c.ID, c.Version)
	return c, nil
}
func (s *Service) ListConfigs(ctx context.Context, scope configdomain.Scope, page, size int) ([]configdomain.Config, int, error) {
	return s.configs.List(ctx, scope, page, size)
}

type CreateVersion struct {
	Value        configdomain.ConfigValue  `json:"value"`
	Rules        []configdomain.TargetRule `json:"rules"`
	Dependencies []configdomain.Dependency `json:"dependencies"`
	ExpiresAt    *time.Time                `json:"expires_at,omitempty"`
}

func (s *Service) CreateVersion(ctx context.Context, actor, id string, in CreateVersion) (configdomain.ConfigVersion, error) {
	c, err := s.configs.Get(ctx, id)
	if err != nil {
		return configdomain.ConfigVersion{}, err
	}
	if err := in.Value.Validate(); err != nil {
		return configdomain.ConfigVersion{}, err
	}
	versions, _ := s.configs.ListVersions(ctx, id)
	number := int64(1)
	for _, v := range versions {
		if v.Number >= number {
			number = v.Number + 1
		}
	}
	v := configdomain.ConfigVersion{ConfigID: id, Number: number, State: configdomain.Draft, Value: in.Value, Rules: configdomain.SortRules(in.Rules), Dependencies: in.Dependencies, ExpiresAt: in.ExpiresAt, CreatedBy: actor, CreatedAt: s.clock.Now(), UpdatedAt: s.clock.Now(), Revision: 1}
	if err := v.Validate(); err != nil {
		return v, err
	}
	if err := s.validateDependencies(ctx, c, v); err != nil {
		return v, err
	}
	if err := s.configs.PutVersion(ctx, v); err != nil {
		return v, err
	}
	s.audit(ctx, c.Scope.Key(), actor, "version.created", fmt.Sprintf("%s/%d", id, number), v)
	s.emit(ctx, "version.created", c.Scope.Key(), id, v.Number)
	return v, nil
}
func (s *Service) validateDependencies(ctx context.Context, c configdomain.Config, v configdomain.ConfigVersion) error {
	g := configdomain.NewGraph()
	versions, err := s.configs.ListVersions(ctx, c.ID)
	if err != nil {
		return err
	}
	for _, old := range versions {
		g.Add(fmt.Sprintf("%s@%d", old.ConfigID, old.Number), old.Dependencies)
	}
	g.Add(fmt.Sprintf("%s@%d", v.ConfigID, v.Number), v.Dependencies)
	if g.HasCycle() {
		return fmt.Errorf("%w: configuration dependency cycle", httpx.ErrBadRequest)
	}
	for _, d := range v.Dependencies {
		if d.Required && d.Key == c.Key {
			return fmt.Errorf("%w: config cannot depend on itself", httpx.ErrBadRequest)
		}
	}
	return nil
}
func (s *Service) TransitionVersion(ctx context.Context, actor, id string, num int64, to configdomain.VersionState, expected int64) (configdomain.ConfigVersion, error) {
	c, err := s.configs.Get(ctx, id)
	if err != nil {
		return configdomain.ConfigVersion{}, err
	}
	v, err := s.configs.GetVersion(ctx, id, num)
	if err != nil {
		return v, err
	}
	if expected != v.Revision {
		return v, fmt.Errorf("%w: version revision mismatch", httpx.ErrPrecondition)
	}
	next, err := v.Transition(to, s.clock.Now())
	if err != nil {
		return v, err
	}
	if err := s.configs.UpdateVersion(ctx, next, v.Revision); err != nil {
		return v, err
	}
	s.audit(ctx, c.Scope.Key(), actor, "version.transition."+string(to), fmt.Sprintf("%s/%d", id, num), next)
	s.emit(ctx, "version.transition", c.Scope.Key(), id, num)
	return next, nil
}
func (s *Service) ValidateVersion(ctx context.Context, id string, num int64) (ValidationReport, error) {
	c, err := s.configs.Get(ctx, id)
	if err != nil {
		return ValidationReport{}, err
	}
	v, err := s.configs.GetVersion(ctx, id, num)
	if err != nil {
		return ValidationReport{}, err
	}
	report := ValidationReport{Valid: true, ConfigID: id, Version: num, CheckedAt: s.clock.Now()}
	if err := v.Validate(); err != nil {
		report.Valid = false
		report.Errors = append(report.Errors, err.Error())
	}
	if err := s.validateDependencies(ctx, c, v); err != nil {
		report.Valid = false
		report.Errors = append(report.Errors, err.Error())
	}
	for _, rule := range v.Rules {
		if err := rule.Validate(); err != nil {
			report.Valid = false
			report.Errors = append(report.Errors, err.Error())
		}
	}
	return report, nil
}

type ValidationReport struct {
	Valid     bool      `json:"valid"`
	ConfigID  string    `json:"config_id"`
	Version   int64     `json:"version"`
	Errors    []string  `json:"errors"`
	CheckedAt time.Time `json:"checked_at"`
}
type Difference struct {
	Path   string `json:"path"`
	Before any    `json:"before"`
	After  any    `json:"after"`
	Kind   string `json:"kind"`
}

func (s *Service) Diff(ctx context.Context, id string, from, to int64) ([]Difference, error) {
	a, err := s.configs.GetVersion(ctx, id, from)
	if err != nil {
		return nil, err
	}
	b, err := s.configs.GetVersion(ctx, id, to)
	if err != nil {
		return nil, err
	}
	out := []Difference{}
	if a.Value.Digest() != b.Value.Digest() {
		out = append(out, Difference{Path: "/value", Before: a.Value.Redacted(), After: b.Value.Redacted(), Kind: "changed"})
	}
	if !sameRules(a.Rules, b.Rules) {
		out = append(out, Difference{Path: "/rules", Before: a.Rules, After: b.Rules, Kind: "changed"})
	}
	if a.State != b.State {
		out = append(out, Difference{Path: "/state", Before: a.State, After: b.State, Kind: "changed"})
	}
	return out, nil
}
func sameRules(a, b []configdomain.TargetRule) bool {
	x, _ := json.Marshal(a)
	y, _ := json.Marshal(b)
	return string(x) == string(y)
}

type EvaluateRequest struct {
	Scope     configdomain.Scope   `json:"scope"`
	Subject   configdomain.Subject `json:"subject"`
	KnownETag string               `json:"known_etag,omitempty"`
}
type EvaluatedConfig struct {
	Key     string                   `json:"key"`
	Value   configdomain.ConfigValue `json:"value"`
	Version int64                    `json:"version"`
	Rule    configdomain.Evaluation  `json:"rule"`
	Proof   string                   `json:"proof"`
}
type EvaluateResponse struct {
	Configs     []EvaluatedConfig `json:"configs"`
	ETag        string            `json:"etag"`
	NotModified bool              `json:"not_modified"`
	EvaluatedAt time.Time         `json:"evaluated_at"`
}

func (s *Service) Evaluate(ctx context.Context, in EvaluateRequest) (EvaluateResponse, error) {
	if err := in.Scope.Validate(); err != nil {
		return EvaluateResponse{}, err
	}
	if in.Subject.TenantID != "" && in.Subject.TenantID != in.Scope.TenantID {
		return EvaluateResponse{}, fmt.Errorf("%w: subject tenant does not match scope", httpx.ErrForbidden)
	}
	versions, err := s.configs.Published(ctx, in.Scope)
	if err != nil {
		return EvaluateResponse{}, err
	}
	out := EvaluateResponse{Configs: []EvaluatedConfig{}, EvaluatedAt: s.clock.Now()}
	seed := strings.Builder{}
	for _, v := range versions {
		if v.ExpiresAt != nil && !s.clock.Now().Before(*v.ExpiresAt) {
			continue
		}
		e := configdomain.Evaluate(v.Rules, in.Subject, s.clock.Now())
		if len(v.Rules) > 0 && !e.Matched {
			continue
		}
		c, _ := s.configs.Get(ctx, v.ConfigID)
		item := EvaluatedConfig{Key: c.Key, Value: v.Value.Redacted(), Version: v.Number, Rule: e}
		item.Proof = s.sign(c.Scope.Key() + "/" + c.Key + fmt.Sprintf("/%d", v.Number))
		out.Configs = append(out.Configs, item)
		seed.WriteString(item.Key + fmt.Sprintf(":%d;", item.Version))
	}
	sort.Slice(out.Configs, func(i, j int) bool { return out.Configs[i].Key < out.Configs[j].Key })
	out.ETag = `\"` + s.sign(seed.String()) + `\"`
	out.NotModified = in.KnownETag != "" && in.KnownETag == out.ETag
	return out, nil
}
func (s *Service) sign(data string) string {
	h := hmac.New(sha256.New, s.secret)
	_, _ = h.Write([]byte(data))
	return base64.RawURLEncoding.EncodeToString(h.Sum(nil))
}
func (s *Service) VerifyProof(data, proof string) bool {
	return hmac.Equal([]byte(s.sign(data)), []byte(proof))
}
func (s *Service) audit(ctx context.Context, scope, actor, action, resource string, data any) {
	b, _ := json.Marshal(data)
	_ = s.audits.Append(ctx, audit.Record{ID: s.id("aud"), Scope: scope, Actor: actor, Action: action, Resource: resource, Metadata: b, At: s.clock.Now()})
}
func (s *Service) emit(ctx context.Context, topic, scope, resource string, version int64) {
	e := Change{Cursor: s.sequence.Add(1), Type: topic, Scope: scope, Resource: resource, Version: version, At: s.clock.Now()}
	b, _ := json.Marshal(e)
	_ = s.outbox.Add(ctx, repository.OutboxEvent{ID: fmt.Sprintf("evt-%d", e.Cursor), Topic: topic, Key: resource, Payload: b, CreatedAt: e.At})
	s.streamMu.Lock()
	defer s.streamMu.Unlock()
	for ch := range s.streams {
		select {
		case ch <- e:
		default:
		}
	}
}
func (s *Service) Subscribe() (<-chan Change, func()) {
	ch := make(chan Change, 64)
	s.streamMu.Lock()
	s.streams[ch] = struct{}{}
	s.streamMu.Unlock()
	return ch, func() { s.streamMu.Lock(); delete(s.streams, ch); close(ch); s.streamMu.Unlock() }
}
func (s *Service) PendingEvents(ctx context.Context, limit int) ([]repository.OutboxEvent, error) {
	return s.outbox.Pending(ctx, limit)
}
func (s *Service) MarkEventDelivered(ctx context.Context, id string) error {
	return s.outbox.MarkDelivered(ctx, id)
}
func (s *Service) CreateRelease(ctx context.Context, actor string, r releasedomain.Release) (releasedomain.Release, error) {
	if err := r.Validate(); err != nil {
		return r, err
	}
	r.ID = s.id("rel")
	r.State = releasedomain.Pending
	r.CreatedAt = s.clock.Now()
	r.UpdatedAt = r.CreatedAt
	r.Revision = 1
	for _, ref := range r.VersionRefs {
		v, err := s.configs.GetVersion(ctx, ref.ConfigID, ref.Version)
		if err != nil {
			return r, err
		}
		if v.State != configdomain.Scheduled && v.State != configdomain.Reviewing {
			return r, fmt.Errorf("%w: version %s/%d must be scheduled or reviewing", httpx.ErrPrecondition, ref.ConfigID, ref.Version)
		}
	}
	if err := s.releases.Create(ctx, r); err != nil {
		return r, err
	}
	s.audit(ctx, r.Scope, actor, "release.created", r.ID, r)
	return r, nil
}
func (s *Service) TransitionRelease(ctx context.Context, actor, id string, to releasedomain.State, reason string, token int64) (releasedomain.Release, error) {
	r, err := s.releases.Get(ctx, id)
	if err != nil {
		return r, err
	}
	next, err := r.Transition(to, token, s.clock.Now())
	if err != nil {
		return r, err
	}
	if to == releasedomain.Paused {
		next.PausedReason = reason
	}
	if to == releasedomain.RolledBack {
		next.RollbackReason = reason
	}
	if err := s.releases.Update(ctx, next, r.Revision); err != nil {
		return r, err
	}
	s.audit(ctx, next.Scope, actor, "release.transition."+string(to), id, next)
	return next, nil
}
func (s *Service) AdvanceReleases(ctx context.Context) error {
	releases, _, err := s.releases.List(ctx, 1, 200)
	if err != nil {
		return err
	}
	for _, r := range releases {
		switch r.State {
		case releasedomain.Pending:
			next, err := s.TransitionRelease(ctx, "scheduler", r.ID, releasedomain.Running, "", r.FencingToken+1)
			if err != nil {
				continue
			}
			for _, ref := range next.VersionRefs {
				v, e := s.configs.GetVersion(ctx, ref.ConfigID, ref.Version)
				if e == nil && v.State == configdomain.Scheduled {
					_, _ = s.TransitionVersion(ctx, "scheduler", ref.ConfigID, ref.Version, configdomain.RollingOut, v.Revision)
				}
			}
		case releasedomain.Running:
			if r.CurrentWave+1 < len(r.Waves) {
				r.CurrentWave++
				r.Revision++
				r.UpdatedAt = s.clock.Now()
				_ = s.releases.Update(ctx, r, r.Revision-1)
				continue
			}
			next, e := s.TransitionRelease(ctx, "scheduler", r.ID, releasedomain.Succeeded, "", r.FencingToken+1)
			if e != nil {
				continue
			}
			for _, ref := range next.VersionRefs {
				v, e := s.configs.GetVersion(ctx, ref.ConfigID, ref.Version)
				if e == nil && v.State == configdomain.RollingOut {
					_, _ = s.TransitionVersion(ctx, "scheduler", ref.ConfigID, ref.Version, configdomain.Published, v.Revision)
				}
			}
		}
	}
	return nil
}
func (s *Service) id(prefix string) string { return fmt.Sprintf("%s_%x", prefix, s.sequence.Add(1)) }

var ErrUnimplemented = errors.New("not implemented")

// RetryRelease moves a failed release back into the retrying state. The caller
// must hold the latest fencing token; a stale retry is rejected by Transition.
func (s *Service) RetryRelease(ctx context.Context, actor, id string) (releasedomain.Release, error) {
	r, err := s.releases.Get(ctx, id)
	if err != nil {
		return r, err
	}
	if r.State != releasedomain.Failed {
		return r, fmt.Errorf("%w: only failed releases can be retried", httpx.ErrPrecondition)
	}
	next, err := r.Transition(releasedomain.Running, r.FencingToken+1, s.clock.Now())
	if err != nil {
		return r, err
	}
	if err := s.releases.Update(ctx, next, r.Revision); err != nil {
		return r, err
	}
	return next, nil
}
