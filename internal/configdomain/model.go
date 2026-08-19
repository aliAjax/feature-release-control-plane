package configdomain

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"regexp"
	"sort"
	"strings"
	"time"
)

type ValueKind string

const (
	String          ValueKind = "string"
	Number          ValueKind = "number"
	Boolean         ValueKind = "boolean"
	JSON            ValueKind = "json"
	SecretReference ValueKind = "secret_reference"
	Template        ValueKind = "template"
)

type VersionState string

const (
	Draft      VersionState = "draft"
	Reviewing  VersionState = "reviewing"
	Scheduled  VersionState = "scheduled"
	RollingOut VersionState = "rolling_out"
	Paused     VersionState = "paused"
	Published  VersionState = "published"
	RolledBack VersionState = "rolled_back"
	Expired    VersionState = "expired"
)

var ErrInvalidValue = errors.New("invalid configuration value")
var ErrInvalidTransition = errors.New("invalid version state transition")

type Scope struct {
	TenantID    string `json:"tenant_id"`
	Application string `json:"application"`
	Environment string `json:"environment"`
	Namespace   string `json:"namespace"`
}

func (s Scope) Key() string {
	return strings.Join([]string{s.TenantID, s.Application, s.Environment, s.Namespace}, "/")
}
func (s Scope) Validate() error {
	if !validName(s.TenantID) || !validName(s.Application) || !validName(s.Environment) || !validName(s.Namespace) {
		return fmt.Errorf("%w: scope names must be 2-64 lowercase letters, digits, hyphens, or underscores", ErrInvalidValue)
	}
	return nil
}
func validName(s string) bool { return regexp.MustCompile(`^[a-z][a-z0-9_-]{1,63}$`).MatchString(s) }

type ConfigValue struct {
	Kind      ValueKind       `json:"kind"`
	Raw       json.RawMessage `json:"raw"`
	SecretRef string          `json:"secret_ref,omitempty"`
}

func (v ConfigValue) Validate() error {
	switch v.Kind {
	case String:
		var x string
		if json.Unmarshal(v.Raw, &x) != nil {
			return fmt.Errorf("%w: string must be JSON encoded", ErrInvalidValue)
		}
	case Number:
		var x float64
		if json.Unmarshal(v.Raw, &x) != nil {
			return fmt.Errorf("%w: number must be JSON encoded", ErrInvalidValue)
		}
	case Boolean:
		var x bool
		if json.Unmarshal(v.Raw, &x) != nil {
			return fmt.Errorf("%w: boolean must be JSON encoded", ErrInvalidValue)
		}
	case JSON:
		var x any
		if json.Unmarshal(v.Raw, &x) != nil {
			return fmt.Errorf("%w: invalid json", ErrInvalidValue)
		}
	case SecretReference:
		if !validName(v.SecretRef) || len(v.Raw) > 0 {
			return fmt.Errorf("%w: secret references carry no value", ErrInvalidValue)
		}
	case Template:
		var x string
		if json.Unmarshal(v.Raw, &x) != nil || !strings.Contains(x, "{{") {
			return fmt.Errorf("%w: template must include a variable", ErrInvalidValue)
		}
	default:
		return fmt.Errorf("%w: unsupported kind", ErrInvalidValue)
	}
	return nil
}
func (v ConfigValue) Redacted() ConfigValue {
	if v.Kind == SecretReference {
		return v
	}
	return v
}
func (v ConfigValue) Digest() string {
	sum := sha256.Sum256(append(append([]byte(v.Kind+":"), v.Raw...), []byte(v.SecretRef)...))
	return hex.EncodeToString(sum[:])
}

type Config struct {
	ID          string    `json:"id"`
	Scope       Scope     `json:"scope"`
	Key         string    `json:"key"`
	Description string    `json:"description"`
	CreatedAt   time.Time `json:"created_at"`
	UpdatedAt   time.Time `json:"updated_at"`
	Version     int64     `json:"version"`
	Archived    bool      `json:"archived"`
}

func (c Config) Validate() error {
	if err := c.Scope.Validate(); err != nil {
		return err
	}
	if !validName(c.Key) {
		return fmt.Errorf("%w: invalid configuration key", ErrInvalidValue)
	}
	if len(c.Description) > 1024 {
		return fmt.Errorf("%w: description exceeds 1024 chars", ErrInvalidValue)
	}
	return nil
}

type ConfigVersion struct {
	ConfigID     string       `json:"config_id"`
	Number       int64        `json:"number"`
	State        VersionState `json:"state"`
	Value        ConfigValue  `json:"value"`
	Rules        []TargetRule `json:"rules"`
	Dependencies []Dependency `json:"dependencies"`
	ExpiresAt    *time.Time   `json:"expires_at,omitempty"`
	CreatedBy    string       `json:"created_by"`
	CreatedAt    time.Time    `json:"created_at"`
	UpdatedAt    time.Time    `json:"updated_at"`
	Revision     int64        `json:"revision"`
	RollbackOf   int64        `json:"rollback_of,omitempty"`
}

func (v ConfigVersion) Validate() error {
	if v.Number < 1 {
		return fmt.Errorf("%w: version must be positive", ErrInvalidValue)
	}
	if err := v.Value.Validate(); err != nil {
		return err
	}
	if !isState(v.State) {
		return fmt.Errorf("%w: unknown state", ErrInvalidValue)
	}
	for _, r := range v.Rules {
		if err := r.Validate(); err != nil {
			return err
		}
	}
	return nil
}
func isState(s VersionState) bool {
	switch s {
	case Draft, Reviewing, Scheduled, RollingOut, Paused, Published, RolledBack, Expired:
		return true
	}
	return false
}
func (v ConfigVersion) CanTransition(to VersionState) bool {
	allowed := map[VersionState]map[VersionState]bool{Draft: {Reviewing: true, Expired: true}, Reviewing: {Draft: true, Scheduled: true, Expired: true}, Scheduled: {RollingOut: true, Paused: true, Expired: true}, RollingOut: {Paused: true, Published: true, RolledBack: true}, Paused: {RollingOut: true, RolledBack: true, Expired: true}, Published: {RolledBack: true, Expired: true}, RolledBack: {}, Expired: {}}
	return allowed[v.State][to]
}
func (v ConfigVersion) Transition(to VersionState, now time.Time) (ConfigVersion, error) {
	if !v.CanTransition(to) {
		return v, fmt.Errorf("%w: %s -> %s", ErrInvalidTransition, v.State, to)
	}
	v.State = to
	v.UpdatedAt = now
	v.Revision++
	return v, nil
}

type Dependency struct {
	Key      string `json:"key"`
	Required bool   `json:"required"`
}
type TargetRule struct {
	ID                 string            `json:"id"`
	Priority           int               `json:"priority"`
	Percentage         int               `json:"percentage"`
	HashAttribute      string            `json:"hash_attribute"`
	RequiredLabels     map[string]string `json:"required_labels,omitempty"`
	RequiredAttributes map[string]string `json:"required_attributes,omitempty"`
	StartsAt           *time.Time        `json:"starts_at,omitempty"`
	EndsAt             *time.Time        `json:"ends_at,omitempty"`
	Experiment         string            `json:"experiment,omitempty"`
	Enabled            bool              `json:"enabled"`
}

func (r TargetRule) Validate() error {
	if r.ID == "" || r.Percentage < 0 || r.Percentage > 100 {
		return fmt.Errorf("%w: rule id and percentage are required", ErrInvalidValue)
	}
	if r.Percentage > 0 && !validName(r.HashAttribute) {
		return fmt.Errorf("%w: invalid hash attribute", ErrInvalidValue)
	}
	if r.StartsAt != nil && r.EndsAt != nil && !r.StartsAt.Before(*r.EndsAt) {
		return fmt.Errorf("%w: invalid rule window", ErrInvalidValue)
	}
	return nil
}
func SortRules(in []TargetRule) []TargetRule {
	out := append([]TargetRule(nil), in...)
	sort.SliceStable(out, func(i, j int) bool { return out[i].Priority > out[j].Priority })
	return out
}
