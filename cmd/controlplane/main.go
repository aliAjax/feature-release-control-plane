package main

import (
	"context"
	"log/slog"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/example/feature-release-control-plane/internal/bootstrap"
)

func main() {
	app, err := bootstrap.New()
	if err != nil {
		slog.Error("bootstrap failed", "error", err)
		os.Exit(1)
	}
	go app.RunWorkers()
	go app.RunHTTP()
	stop := make(chan os.Signal, 1)
	signal.Notify(stop, syscall.SIGINT, syscall.SIGTERM)
	<-stop
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	if err := app.Shutdown(ctx); err != nil {
		slog.Error("shutdown failed", "error", err)
		os.Exit(1)
	}
}
