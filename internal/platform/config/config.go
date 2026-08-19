package config

import (
	"os"
	"strconv"
	"strings"
)

type Config struct {
	HTTPAddress           string
	AdminToken            string
	SnapshotSecret        string
	LogLevel              string
	SSEHeartbeatSeconds   int
	WorkerIntervalSeconds int
}

func Load() Config {
	return Config{HTTPAddress: env("CONTROLPLANE_HTTP_ADDRESS", ":8080"), AdminToken: env("CONTROLPLANE_ADMIN_TOKEN", "development-token"), SnapshotSecret: env("CONTROLPLANE_SNAPSHOT_SECRET", "development-secret-change-me"), LogLevel: env("CONTROLPLANE_LOG_LEVEL", "info"), SSEHeartbeatSeconds: integer("CONTROLPLANE_SSE_HEARTBEAT_SECONDS", 15), WorkerIntervalSeconds: integer("CONTROLPLANE_WORKER_INTERVAL_SECONDS", 1)}
}
func env(key, fallback string) string {
	if v := strings.TrimSpace(os.Getenv(key)); v != "" {
		return v
	}
	return fallback
}
func integer(key string, fallback int) int {
	if v, err := strconv.Atoi(os.Getenv(key)); err == nil && v > 0 {
		return v
	}
	return fallback
}
