package mcp

import (
	"bufio"
	"context"
	"encoding/json"
	"errors"
	"io"
	"log/slog"
	"os"
	"strconv"
	"strings"
	"time"
)

// Run implements framed stdio JSON-RPC with MCP built-ins: initialize, tools/list, tools/call.
func (s *Server) Run(ctx context.Context, r io.Reader, w io.Writer) error {
	br := bufio.NewReader(r)
	peek, err := br.Peek(32)
	if err != nil && !errors.Is(err, io.EOF) {
		return err
	}
	if strings.HasPrefix(strings.ToLower(string(peek)), "content-length:") {
		return s.runFramed(ctx, br, w)
	}
	return s.runNDJSON(ctx, br, w)
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
			if errors.Is(err, io.EOF) || errors.Is(err, context.Canceled) {
				// Graceful shutdown when input stream closes
				return nil
			}
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
			if len(req.Params) > 0 {
				if err := json.Unmarshal(req.Params, &p); err != nil {
					_ = fw.WriteJSON(rpcResponse{JSONRPC: "2.0", ID: req.ID, Error: &rpcError{Code: -32602, Message: "invalid params"}})
					continue
				}
			}
			res := InitializeResult{
				ServerInfo:   ServerInfo{Name: "mcp-k8s-server", Version: "0.1.0-go"},
				Capabilities: map[string]any{"tools": map[string]any{}},
			}
			if err := fw.WriteJSON(rpcResponse{JSONRPC: "2.0", ID: req.ID, Result: res}); err != nil {
				return err
			}
			// trigger background initialization after we have responded
			s.triggerInitialized(ctx)
		case "tools/list":
			res := ToolsListResult{Tools: reg.List()}
			_ = fw.WriteJSON(rpcResponse{JSONRPC: "2.0", ID: req.ID, Result: res})
		case "tools/call":
			// forward to registry
			var p ToolsCallParams
			if err := json.Unmarshal(req.Params, &p); err != nil {
				_ = fw.WriteJSON(rpcResponse{JSONRPC: "2.0", ID: req.ID, Error: &rpcError{Code: -32602, Message: "invalid params"}})
				continue
			}
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
			out, err := reg.Call(callCtx, p.Name, p.Arguments)
			if err != nil {
				_ = fw.WriteJSON(rpcResponse{JSONRPC: "2.0", ID: req.ID, Error: &rpcError{Code: -32000, Message: err.Error()}})
				continue
			}
			if out.IsError {
				msg := "tool execution error"
				if len(out.Content) > 0 {
					msg = out.Content[0].Text
				}
				_ = fw.WriteJSON(rpcResponse{JSONRPC: "2.0", ID: req.ID, Error: &rpcError{Code: -32000, Message: msg}})
				continue
			}
			_ = fw.WriteJSON(rpcResponse{JSONRPC: "2.0", ID: req.ID, Result: out})
		default:
			_ = fw.WriteJSON(rpcResponse{JSONRPC: "2.0", ID: req.ID, Error: &rpcError{Code: -32601, Message: "Method not found"}})
		}
	}
}

// runNDJSON supports newline-delimited JSON for simple CLI testing
func (s *Server) runNDJSON(ctx context.Context, r io.Reader, w io.Writer) error {
	dec := json.NewDecoder(r)
	enc := json.NewEncoder(w)
	enc.SetEscapeHTML(false)

	reg := s.Registry()
	s.installBuiltins(reg)

	for {
		select {
		case <-ctx.Done():
			return nil
		default:
		}
		var req rpcRequest
		if err := dec.Decode(&req); err != nil {
			if errors.Is(err, io.EOF) || errors.Is(err, context.Canceled) {
				return nil
			}
			_ = enc.Encode(rpcResponse{JSONRPC: "2.0", Error: &rpcError{Code: -32700, Message: "parse error"}})
			continue
		}
		if req.JSONRPC != "2.0" {
			_ = enc.Encode(rpcResponse{JSONRPC: "2.0", ID: req.ID, Error: &rpcError{Code: -32600, Message: "invalid request"}})
			continue
		}
		switch req.Method {
		case "initialize":
			var p InitializeParams
			if len(req.Params) > 0 {
				if err := json.Unmarshal(req.Params, &p); err != nil {
					_ = enc.Encode(rpcResponse{JSONRPC: "2.0", ID: req.ID, Error: &rpcError{Code: -32602, Message: "invalid params"}})
					continue
				}
			}
			res := InitializeResult{ServerInfo: ServerInfo{Name: "mcp-k8s-server", Version: "0.1.0-go"}, Capabilities: map[string]any{"tools": map[string]any{}}}
			if err := enc.Encode(rpcResponse{JSONRPC: "2.0", ID: req.ID, Result: res}); err != nil {
				return err
			}
			s.triggerInitialized(ctx)
		case "tools/list":
			res := ToolsListResult{Tools: reg.List()}
			_ = enc.Encode(rpcResponse{JSONRPC: "2.0", ID: req.ID, Result: res})
		case "tools/call":
			var p ToolsCallParams
			if err := json.Unmarshal(req.Params, &p); err != nil {
				_ = enc.Encode(rpcResponse{JSONRPC: "2.0", ID: req.ID, Error: &rpcError{Code: -32602, Message: "invalid params"}})
				continue
			}
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
			out, err := reg.Call(callCtx, p.Name, p.Arguments)
			if err != nil {
				_ = enc.Encode(rpcResponse{JSONRPC: "2.0", ID: req.ID, Error: &rpcError{Code: -32000, Message: err.Error()}})
				continue
			}
			if out.IsError {
				msg := "tool execution error"
				if len(out.Content) > 0 {
					msg = out.Content[0].Text
				}
				_ = enc.Encode(rpcResponse{JSONRPC: "2.0", ID: req.ID, Error: &rpcError{Code: -32000, Message: msg}})
				continue
			}
			_ = enc.Encode(rpcResponse{JSONRPC: "2.0", ID: req.ID, Result: out})
		default:
			_ = enc.Encode(rpcResponse{JSONRPC: "2.0", ID: req.ID, Error: &rpcError{Code: -32601, Message: "Method not found"}})
		}
	}
}

func (s *Server) installBuiltins(reg *Registry) {
	// Tools will be registered by higher layers (k8s) via exported accessor in the future.
	// Keep logger reference to show we are alive
	s.logger.Info("MCP server started", slog.String("version", "0.1.0-go"))

	// Simple echo tool for readiness and smoke testing
	reg.Register(Tool{
		Name:        "echo",
		Description: "Echo back the provided text",
		Handler: func(ctx context.Context, params json.RawMessage) (any, error) {
			var p struct {
				Text string `json:"text"`
			}
			_ = json.Unmarshal(params, &p)
			return p.Text, nil
		},
	})
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
