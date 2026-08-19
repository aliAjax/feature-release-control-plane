package configdomain

import (
	"errors"
	"testing"
)

func TestDependenciesCheckedReportsCycle(t *testing.T) {
	g := NewGraph()
	g.Add("a", []Dependency{{Key: "b"}})
	g.Add("b", []Dependency{{Key: "a"}})
	_, err := g.DependenciesChecked("a")
	if !errors.Is(err, ErrInvalidValue) {
		t.Fatalf("expected wrapped ErrInvalidValue, got %v", err)
	}
}
