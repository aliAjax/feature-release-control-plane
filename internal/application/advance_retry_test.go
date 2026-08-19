package application

import (
	"context"
	"testing"
	"time"

	"github.com/example/feature-release-control-plane/internal/releasedomain"
	"github.com/example/feature-release-control-plane/internal/repository"
)

func TestAdvanceReleasesPromotesRetrying(t *testing.T) {
	store := repository.NewMemory()
	releases := releasedomain.NewMemoryRepository()
	svc := New(store, store, store, releases, "0123456789abcdef")
	now := time.Now()
	r := releasedomain.Release{
		ID: "rel-1", Scope: "scope",
		VersionRefs: []releasedomain.VersionRef{{ConfigID: "cfg-1", Version: 1}},
		Waves:       []releasedomain.Wave{{Number: 1, Percentage: 100, MaxFailureRate: 0.1}},
		State:       releasedomain.Retrying, FencingToken: 1, Revision: 1,
		CreatedAt: now, UpdatedAt: now,
	}
	if err := svc.releases.Create(context.Background(), r); err != nil {
		t.Fatal(err)
	}
	if err := svc.AdvanceReleases(context.Background()); err != nil {
		t.Fatal(err)
	}
	got, err := svc.releases.Get(context.Background(), "rel-1")
	if err != nil {
		t.Fatal(err)
	}
	if got.State != releasedomain.Running {
		t.Fatalf("expected Retrying to be promoted to Running, got %s", got.State)
	}
}
