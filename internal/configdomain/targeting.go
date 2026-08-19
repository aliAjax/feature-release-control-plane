package configdomain

import (
	"crypto/sha256"
	"encoding/binary"
	"fmt"
	"strings"
	"time"
)

type Subject struct {
	TenantID   string            `json:"tenant_id"`
	Labels     map[string]string `json:"labels"`
	Attributes map[string]string `json:"attributes"`
	DeviceID   string            `json:"device_id,omitempty"`
}
type RuleExplanation struct {
	RuleID   string `json:"rule_id"`
	Matched  bool   `json:"matched"`
	Reason   string `json:"reason"`
	Bucket   int    `json:"bucket,omitempty"`
	Priority int    `json:"priority"`
}
type Evaluation struct {
	Matched      bool              `json:"matched"`
	RuleID       string            `json:"rule_id,omitempty"`
	Explanations []RuleExplanation `json:"explanations"`
}

func Evaluate(rules []TargetRule, subject Subject, at time.Time) Evaluation {
	result := Evaluation{}
	experiments := map[string]bool{}
	for _, rule := range SortRules(rules) {
		x := RuleExplanation{RuleID: rule.ID, Priority: rule.Priority}
		if !rule.Enabled {
			x.Reason = "disabled"
			result.Explanations = append(result.Explanations, x)
			continue
		}
		if rule.StartsAt != nil && at.Before(*rule.StartsAt) {
			x.Reason = "window_not_started"
			result.Explanations = append(result.Explanations, x)
			continue
		}
		if rule.EndsAt != nil && !at.Before(*rule.EndsAt) {
			x.Reason = "window_ended"
			result.Explanations = append(result.Explanations, x)
			continue
		}
		if rule.Experiment != "" && experiments[rule.Experiment] {
			x.Reason = "mutually_exclusive_experiment_already_selected"
			result.Explanations = append(result.Explanations, x)
			continue
		}
		if !contains(subject.Labels, rule.RequiredLabels) {
			x.Reason = "label_mismatch"
			result.Explanations = append(result.Explanations, x)
			continue
		}
		if !contains(subject.Attributes, rule.RequiredAttributes) {
			x.Reason = "attribute_mismatch"
			result.Explanations = append(result.Explanations, x)
			continue
		}
		identity := subject.Attributes[rule.HashAttribute]
		if identity == "" {
			identity = subject.DeviceID
		}
		if identity == "" {
			identity = subject.TenantID
		}
		bucket := stableBucket(rule.ID, identity)
		x.Bucket = bucket
		if bucket >= rule.Percentage {
			x.Reason = fmt.Sprintf("bucket_%d_outside_%d", bucket, rule.Percentage)
			result.Explanations = append(result.Explanations, x)
			continue
		}
		x.Matched = true
		x.Reason = "matched"
		result.Explanations = append(result.Explanations, x)
		result.Matched = true
		result.RuleID = rule.ID
		if rule.Experiment != "" {
			experiments[rule.Experiment] = true
		}
		return result
	}
	return result
}
func stableBucket(ruleID, identity string) int {
	sum := sha256.Sum256([]byte(strings.Join([]string{ruleID, identity}, ":")))
	return int(binary.BigEndian.Uint32(sum[:4]) % 100)
}
func contains(have, want map[string]string) bool {
	for k, v := range want {
		if have[k] != v {
			return false
		}
	}
	return true
}

// EvaluateChecked evaluates rules and reports a malformed subject as a wrapped
// error. Callers that need a hard failure instead of a silent no-match use it.
func EvaluateChecked(rules []TargetRule, subject Subject, at time.Time) (Evaluation, error) {
	if strings.TrimSpace(subject.TenantID) == "" {
		return Evaluation{}, fmt.Errorf("%w: subject tenant is required", ErrInvalidValue)
	}
	return Evaluate(rules, subject, at), nil
}

// ValidateRules validates every rule and wraps the first invalid rule so
// callers can detect it with errors.Is without inspecting the message.
func ValidateRules(rules []TargetRule) error {
	for _, r := range rules {
		if err := r.Validate(); err != nil {
			return fmt.Errorf("%w: %v", ErrInvalidValue, err)
		}
	}
	return nil
}
