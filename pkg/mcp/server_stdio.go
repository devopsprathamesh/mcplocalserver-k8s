package mcp

import (
	"context"
	"encoding/json"
	"io"
	"log/slog"
	"os"
	"strconv"
	"time"
)

// Run implements framed stdio JSON-RPC with MCP built-ins: initialize, tools/list, tools/call.
func (s *Server) Run(ctx context.Context, r io.Reader, w io.Writer) error {
	return s.runFramed(ctx, r, w)
}

func (s *Server) runFramed(ctx context.Context, r io.Reader, w io.Writer) error {
	fr := newFramedReader(r)
	fw := newFramedWriter(w)

	reg := s.Registry()
	s.installBuiltins(reg)

	for {
		select {
		case <-ctx.Done():
			return nil
		default:
		}
		msg, err := fr.ReadMessage()
		if err != nil {
			return err
		}
		var req rpcRequest
		if err := json.Unmarshal(msg, &req); err != nil {
			_ = fw.WriteJSON(rpcResponse{JSONRPC: "2.0", Error: &rpcError{Code: -32700, Message: "parse error"}})
			continue
		}
		if req.JSONRPC != "2.0" {
			_ = fw.WriteJSON(rpcResponse{JSONRPC: "2.0", ID: req.ID, Error: &rpcError{Code: -32600, Message: "invalid request"}})
			continue
		}

		// Dispatch
		switch req.Method {
		case "initialize":
			var p InitializeParams
			_ = json.Unmarshal(req.Params, &p)
			res := InitializeResult{
				ServerInfo:   ServerInfo{Name: "mcp-k8s-server", Version: "0.1.0-go"},
				Capabilities: map[string]any{"tools": map[string]any{"listChanged": false}},
			}
			_ = fw.WriteJSON(rpcResponse{JSONRPC: "2.0", ID: req.ID, Result: res})
		case "tools/list":
			res := ToolsListResult{Tools: reg.List()}
			_ = fw.WriteJSON(rpcResponse{JSONRPC: "2.0", ID: req.ID, Result: res})
		case "tools/call":
			// forward to registry
			var p ToolsCallParams
			_ = json.Unmarshal(req.Params, &p)
			// Per-call timeout to mirror TS behavior when applicable
			timeoutMs := 0
			if tm := getEnvInt("MCP_K8S_TIMEOUT_MS", 0); tm > 0 {
				timeoutMs = tm
			}
			callCtx := ctx
			var cancel context.CancelFunc
			if timeoutMs > 0 {
				callCtx, cancel = context.WithTimeout(ctx, time.Duration(timeoutMs)*time.Millisecond)
				defer cancel()
			}
			out, _ := reg.Call(callCtx, p.Name, p.Arguments)
			_ = fw.WriteJSON(rpcResponse{JSONRPC: "2.0", ID: req.ID, Result: out})
		default:
			_ = fw.WriteJSON(rpcResponse{JSONRPC: "2.0", ID: req.ID, Error: &rpcError{Code: -32601, Message: "method not found"}})
		}
	}
}

func (s *Server) installBuiltins(reg *Registry) {
	// Tools will be registered by higher layers (k8s) via exported accessor in the future.
	// Keep logger reference to show we are alive
	s.logger.Info("MCP server started", slog.String("version", "0.1.0-go"))
}

// getEnvInt parses an int env var; returns def on error.
func getEnvInt(key string, def int) int {
	v := os.Getenv(key)
	if v == "" {
		return def
	}
	n, err := strconv.Atoi(v)
	if err != nil {
		return def
	}
	return n
}
