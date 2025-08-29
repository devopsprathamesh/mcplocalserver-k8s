package mcp

import (
	"context"
	"encoding/json"
	"log/slog"
	"sync"
)

// JSON-RPC 2.0 request/response structures
type rpcRequest struct {
	JSONRPC string          `json:"jsonrpc"`
	ID      json.RawMessage `json:"id,omitempty"`
	Method  string          `json:"method"`
	Params  json.RawMessage `json:"params,omitempty"`
}

type rpcResponse struct {
	JSONRPC string          `json:"jsonrpc"`
	ID      json.RawMessage `json:"id,omitempty"`
	Result  any             `json:"result,omitempty"`
	Error   *rpcError       `json:"error,omitempty"`
}

type rpcError struct {
	Code    int    `json:"code"`
	Message string `json:"message"`
}

// Server implements a minimal MCP-compatible JSON-RPC stdio server.
type Server struct {
	logger *slog.Logger
	mu     sync.RWMutex
	// handlers keyed by method name
	handlers map[string]Handler
	reg      *Registry
	// onInitialized is called once, after a successful initialize response is sent
	onInitialized func(ctx context.Context, s *Server)
	initOnce      sync.Once
}

type Handler func(ctx context.Context, params json.RawMessage) (any, *rpcError)

func NewServer(logger *slog.Logger) *Server {
	return &Server{
		logger:   logger,
		handlers: make(map[string]Handler),
		reg:      NewRegistry(),
	}
}

// Register registers a JSON-RPC method handler.
func (s *Server) Register(method string, h Handler) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.handlers[method] = h
}

// Registry returns the tool registry so callers can register tools.
func (s *Server) Registry() *Registry { return s.reg }

// OnInitialized registers a callback that will be run once in the background
// immediately after the server replies to the first initialize request.
func (s *Server) OnInitialized(f func(ctx context.Context, srv *Server)) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.onInitialized = f
}

func (s *Server) triggerInitialized(ctx context.Context) {
	s.initOnce.Do(func() {
		s.mu.RLock()
		cb := s.onInitialized
		s.mu.RUnlock()
		if cb != nil {
			go cb(ctx, s)
		}
	})
}
