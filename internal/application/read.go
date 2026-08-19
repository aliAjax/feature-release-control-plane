package application

import (
	"context"
	"github.com/example/feature-release-control-plane/internal/configdomain"
	"github.com/example/feature-release-control-plane/internal/releasedomain"
)

func (s *Service) ListVersions(ctx context.Context, id string) ([]configdomain.ConfigVersion, error) {
	return s.configs.ListVersions(ctx, id)
}
func (s *Service) ListReleases(ctx context.Context, page, size int) ([]releasedomain.Release, int, error) {
	return s.releases.List(ctx, page, size)
}

// ListReleasesByState returns releases in the given state for operator
// dashboards and retry queues.
func (s *Service) ListReleasesByState(ctx context.Context, state releasedomain.State) ([]releasedomain.Release, error) {
	all, _, err := s.releases.List(ctx, 1, 200)
	if err != nil {
		return nil, err
	}
	out := []releasedomain.Release{}
	for _, r := range all {
		if r.State == state {
			out = append(out, r)
		}
	}
	return out, nil
}
