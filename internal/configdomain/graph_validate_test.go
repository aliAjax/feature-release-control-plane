package configdomain

import (
	"errors"
	"testing"
)

func TestValidateDependencyGraphReportsCycle(t *testing.T) {
	g := NewGraph()
	g.Add("a", []Dependency{{Key: "b"}})
	g.Add("b", []Dependency{{Key: "a"}})
	err := g.ValidateDependencyGraph()
	if !errors.Is(err, ErrInvalidValue) {
		t.Fatalf("expected wrapped ErrInvalidValue, got %v", err)
	}
}
