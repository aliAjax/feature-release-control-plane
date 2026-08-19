package main

import (
	"context"
	"log/slog"
	"os"
	"os/signal"
	"syscall"

	"github.com/example/feature-release-control-plane/internal/bootstrap"
)

func main() {
	app, err := bootstrap.New()
	if err != nil {
		slog.Error("bootstrap failed", "error", err)
		os.Exit(1)
	}
	go app.RunWorkers()
	stop := make(chan os.Signal, 1)
	signal.Notify(stop, syscall.SIGINT, syscall.SIGTERM)
	<-stop
	if err := app.Shutdown(context.Background()); err != nil {
		slog.Error("shutdown failed", "error", err)
	}
}
