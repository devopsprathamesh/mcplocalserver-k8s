package main

import (
	"context"
	"errors"
	"io"
	"log/slog"
	"os"
	"os/signal"
	"syscall"

	"github.com/example/mcp-k8s-server-go/internal/tools"
	"github.com/example/mcp-k8s-server-go/pkg/k8s"
	"github.com/example/mcp-k8s-server-go/pkg/mcp"
)

func main() {
	logger := slog.New(slog.NewTextHandler(os.Stderr, &slog.HandlerOptions{Level: slog.LevelInfo}))
	ctx, stop := signal.NotifyContext(context.Background(), syscall.SIGINT, syscall.SIGTERM)
	defer stop()

	server := mcp.NewServer(logger)
	// Initialize k8s and register tools at startup to avoid mcp<->tools cycles
	if kc, err := k8s.Load(ctx, logger); err == nil {
		reg := server.Registry()
		tools.RegisterCluster(reg, kc, logger)
		tools.RegisterWorkloads(reg, kc)
		tools.RegisterResources(reg, kc)
		tools.RegisterSecrets(reg, kc)
	} else {
		logger.Warn("k8s not initialized", slog.String("error", err.Error()))
	}

	if err := server.Run(ctx, os.Stdin, os.Stdout); err != nil {
		if errors.Is(err, context.Canceled) || errors.Is(err, io.EOF) {
			logger.Info("server stopped (stdin closed)")
			return
		}
		logger.Error("server exited with error", "error", err)
		os.Exit(1)
	}
}
