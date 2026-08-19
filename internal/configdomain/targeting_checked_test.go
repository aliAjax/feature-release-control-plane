package configdomain

import (
	"errors"
	"testing"
	"time"
)

func TestEvaluateCheckedRejectsEmptySubject(t *testing.T) {
	_, err := EvaluateChecked([]TargetRule{{ID: "r1", Percentage: 10, HashAttribute: "device_id", Enabled: true}}, Subject{}, time.Now())
	if !errors.Is(err, ErrInvalidValue) {
		t.Fatalf("expected wrapped ErrInvalidValue, got %v", err)
	}
}
