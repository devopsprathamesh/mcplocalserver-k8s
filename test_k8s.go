package main

import (
	"context"
	"log/slog"
	"os"

	"github.com/example/mcp-k8s-server-go/pkg/k8s"
)

func main() {
	logger := slog.New(slog.NewTextHandler(os.Stderr, &slog.HandlerOptions{Level: slog.LevelInfo}))
	kc, err := k8s.Load(context.Background(), logger)
	if err != nil {
		logger.Error("Failed to load Kubernetes client", "error", err)
		os.Exit(1)
	}

	version, err := kc.Discovery.ServerVersion()
	if err != nil {
		logger.Error("Failed to get server version", "error", err)
		os.Exit(1)
	}

	logger.Info("Kubernetes client initialized successfully", "version", version.GitVersion)
}
