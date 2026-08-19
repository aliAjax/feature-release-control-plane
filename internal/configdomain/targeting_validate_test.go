package configdomain

import (
	"errors"
	"testing"
)

func TestValidateRulesReportsInvalidRule(t *testing.T) {
	err := ValidateRules([]TargetRule{{ID: "", Percentage: 0}})
	if !errors.Is(err, ErrInvalidValue) {
		t.Fatalf("expected wrapped ErrInvalidValue, got %v", err)
	}
}
