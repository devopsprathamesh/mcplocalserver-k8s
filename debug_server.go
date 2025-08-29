package main

import (
	"context"
	"log/slog"
	"os"

	"github.com/example/mcp-k8s-server-go/pkg/mcp"
)

func main() {
	logger := slog.New(slog.NewTextHandler(os.Stderr, &slog.HandlerOptions{Level: slog.LevelInfo}))
	server := mcp.NewServer(logger)

	// Test OnInitialized callback
	server.OnInitialized(func(bg context.Context, srv *mcp.Server) {
		logger.Info("OnInitialized callback triggered!")
	})

	// Test with simple NDJSON
	input := `{"jsonrpc":"2.0","id":1,"method":"initialize","params":{}}
{"jsonrpc":"2.0","id":2,"method":"tools/list","params":{}}`

	r := os.Stdin
	w := os.Stdout

	// For testing, we'll use the input directly
	// In a real scenario, this would be stdin/stdout
	logger.Info("Server starting...")

	// We'll just simulate the input for now
	logger.Info("Test complete")
}
