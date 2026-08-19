package releasedomain

import (
	"errors"
	"testing"
	"time"
)

func TestReleaseTransitionIncludesRetrying(t *testing.T) {
	r := Release{ID: "rel-1", Scope: "scope", VersionRefs: []VersionRef{{ConfigID: "c", Version: 1}}, Waves: []Wave{{Number: 1, Percentage: 100, MaxFailureRate: 0.1}}, State: Pending, FencingToken: 0, Revision: 1}
	now := time.Now()
	var err error
	if r, err = r.Transition(Running, 1, now); err != nil {
		t.Fatal(err)
	}
	if r, err = r.Transition(Failed, 2, now); err != nil {
		t.Fatal(err)
	}
	if r, err = r.Transition(Retrying, 3, now); err != nil {
		t.Fatal(err)
	}
	if r, err = r.Transition(Running, 4, now); err != nil {
		t.Fatalf("Retrying -> Running should be allowed, got %v", err)
	}
	if !errors.Is(err, ErrTransition) && r.State != Running {
		t.Fatalf("unexpected state %s", r.State)
	}
}
