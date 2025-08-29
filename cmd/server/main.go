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

	// Register placeholders so tools/list is populated even before k8s is ready
	reg := server.Registry()
	tools.RegisterCluster(reg, nil, logger)
	tools.RegisterWorkloads(reg, nil)
	tools.RegisterResources(reg, nil)
	tools.RegisterSecrets(reg, nil)

	// Defer k8s client setup until after MCP initialize response
	server.OnInitialized(func(bg context.Context, srv *mcp.Server) {
		kc, err := k8s.Load(bg, logger)
		if err != nil {
			logger.Warn("k8s not initialized", slog.String("error", err.Error()))
			return
		}
		reg := srv.Registry()
		// Re-register concrete implementations over placeholders
		tools.RegisterCluster(reg, kc, logger)
		tools.RegisterWorkloads(reg, kc)
		tools.RegisterResources(reg, kc)
		tools.RegisterSecrets(reg, kc)
		logger.Info("k8s tools registered")
	})

	if err := server.Run(ctx, os.Stdin, os.Stdout); err != nil {
		if errors.Is(err, context.Canceled) || errors.Is(err, io.EOF) {
			logger.Info("server stopped (stdin closed)")
			return
		}
		logger.Error("server exited with error", "error", err)
		os.Exit(1)
	}
}
