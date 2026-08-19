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
