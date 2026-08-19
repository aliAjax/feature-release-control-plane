package application

import (
	"context"
	"testing"
	"time"

	"github.com/example/feature-release-control-plane/internal/releasedomain"
)

func TestListReleasesByStateIncludesRetrying(t *testing.T) {
	svc := New(nil, nil, nil, releasedomain.NewMemoryRepository(), "0123456789abcdef")
	now := time.Now()
	_ = svc.releases.Create(context.Background(), releasedomain.Release{
		ID: "rel-1", Scope: "scope",
		VersionRefs: []releasedomain.VersionRef{{ConfigID: "c", Version: 1}},
		Waves:       []releasedomain.Wave{{Number: 1, Percentage: 100, MaxFailureRate: 0.1}},
		State:       releasedomain.Retrying, Revision: 1, CreatedAt: now, UpdatedAt: now,
	})
	items, err := svc.ListReleasesByState(context.Background(), releasedomain.Retrying)
	if err != nil {
		t.Fatal(err)
	}
	if len(items) != 1 {
		t.Fatalf("expected retrying release to be listed, got %d", len(items))
	}
}
