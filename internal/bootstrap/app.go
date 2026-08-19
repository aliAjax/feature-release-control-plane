package bootstrap

import (
	"context"
	"github.com/example/feature-release-control-plane/internal/application"
	"github.com/example/feature-release-control-plane/internal/platform/config"
	"github.com/example/feature-release-control-plane/internal/releasedomain"
	"github.com/example/feature-release-control-plane/internal/repository"
	"github.com/example/feature-release-control-plane/internal/transport/httpapi"
	"log/slog"
	"os"
	"time"
)

type App struct {
	server  *httpapi.Server
	service *application.Service
	done    chan struct{}
}

func New() (*App, error) {
	cfg := config.Load()
	logger := slog.New(slog.NewJSONHandler(os.Stdout, &slog.HandlerOptions{Level: parseLevel(cfg.LogLevel)}))
	slog.SetDefault(logger)
	store := repository.NewMemory()
	releases := releasedomain.NewMemoryRepository()
	service := application.New(store, store, store, releases, cfg.SnapshotSecret)
	return &App{server: httpapi.New(cfg.HTTPAddress, service, cfg.SSEHeartbeatSeconds), service: service, done: make(chan struct{})}, nil
}
func (a *App) RunHTTP() {
	if err := a.server.ListenAndServe(); err != nil {
		slog.Error("http server stopped", "error", err)
	}
}
func (a *App) RunWorkers() {
	ticker := time.NewTicker(time.Second)
	defer ticker.Stop()
	for {
		select {
		case <-a.done:
			return
		case <-ticker.C:
			_ = a.service.AdvanceReleases(context.Background())
			events, err := a.service.PendingEvents(context.Background(), 100)
			if err != nil {
				continue
			}
			for _, event := range events {
				_ = a.service.MarkEventDelivered(context.Background(), event.ID)
			}
		}
	}
}
func (a *App) Shutdown(ctx context.Context) error { close(a.done); return a.server.Shutdown(ctx) }
func parseLevel(s string) slog.Level {
	switch s {
	case "debug":
		return slog.LevelDebug
	case "warn":
		return slog.LevelWarn
	case "error":
		return slog.LevelError
	default:
		return slog.LevelInfo
	}
}
