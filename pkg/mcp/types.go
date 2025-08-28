package mcp

import (
	"context"
	"encoding/json"
)

// MCP tool model (minimal for parity with TS server)
type Tool struct {
	Name        string `json:"name"`
	Description string `json:"description,omitempty"`
	// JSON schema for params (free-form)
	InputSchema json.RawMessage `json:"inputSchema,omitempty"`
	Handler     ToolHandler     `json:"-"`
}

type ToolHandler func(ctx context.Context, params json.RawMessage) (any, error)

// Initialize
type InitializeParams struct {
	ClientInfo struct {
		Name    string `json:"name"`
		Version string `json:"version,omitempty"`
	} `json:"clientInfo"`
}

type InitializeResult struct {
	ServerInfo   ServerInfo     `json:"serverInfo"`
	Capabilities map[string]any `json:"capabilities"`
}

type ServerInfo struct {
	Name    string `json:"name"`
	Version string `json:"version"`
}

// tools/list
type ToolsListResult struct {
	Tools []Tool `json:"tools"`
}

// tools/call
type ToolsCallParams struct {
	Name      string          `json:"name"`
	Arguments json.RawMessage `json:"arguments,omitempty"`
}

type TextContent struct {
	Type string `json:"type"`
	Text string `json:"text"`
}

type ToolsCallResult struct {
	Content []TextContent `json:"content"`
	IsError bool          `json:"isError,omitempty"`
}
