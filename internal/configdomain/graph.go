package configdomain

import (
	"fmt"
	"sort"
)

type Graph struct{ Edges map[string][]string }

func NewGraph() Graph { return Graph{Edges: map[string][]string{}} }
func (g Graph) Add(key string, deps []Dependency) {
	out := make([]string, 0, len(deps))
	for _, d := range deps {
		out = append(out, d.Key)
	}
	sort.Strings(out)
	g.Edges[key] = out
}
func (g Graph) Dependencies(key string) []string {
	seen := map[string]bool{}
	result := []string{}
	var visit func(string)
	visit = func(k string) {
		for _, next := range g.Edges[k] {
			if !seen[next] {
				seen[next] = true
				result = append(result, next)
				visit(next)
			}
		}
	}
	visit(key)
	sort.Strings(result)
	return result
}
func (g Graph) HasCycle() bool {
	state := map[string]uint8{}
	var visit func(string) bool
	visit = func(k string) bool {
		if state[k] == 1 {
			return true
		}
		if state[k] == 2 {
			return false
		}
		state[k] = 1
		for _, n := range g.Edges[k] {
			if visit(n) {
				return true
			}
		}
		state[k] = 2
		return false
	}
	for k := range g.Edges {
		if visit(k) {
			return true
		}
	}
	return false
}

// DependenciesChecked returns transitive dependencies, reporting a cycle as a
// wrapped error instead of silently returning the partial traversal.
func (g Graph) DependenciesChecked(key string) ([]string, error) {
	if g.HasCycle() {
		return nil, fmt.Errorf("%w: dependency graph contains a cycle", ErrInvalidValue)
	}
	return g.Dependencies(key), nil
}

// ValidateDependencyGraph reports a cycle as a wrapped error, which is the
// strict variant used before publishing a dependency set.
func (g Graph) ValidateDependencyGraph() error {
	if g.HasCycle() {
		return fmt.Errorf("%w: dependency graph contains a cycle", ErrInvalidValue)
	}
	return nil
}
