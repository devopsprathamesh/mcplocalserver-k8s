package mcp

import (
	"context"
	"encoding/json"
	"fmt"
)

// Registry stores tools and exposes MCP built-ins
type Registry struct {
	tools map[string]*Tool
}

func NewRegistry() *Registry { return &Registry{tools: map[string]*Tool{}} }

func (r *Registry) Register(t Tool) {
	tt := t // copy
	r.tools[t.Name] = &tt
}

func (r *Registry) List() []Tool {
	out := make([]Tool, 0, len(r.tools))
	for _, t := range r.tools {
		out = append(out, Tool{Name: t.Name, Description: t.Description, InputSchema: t.InputSchema})
	}
	return out
}

func (r *Registry) Call(ctx context.Context, name string, args json.RawMessage) (ToolsCallResult, error) {
	t, ok := r.tools[name]
	if !ok {
		return ToolsCallResult{Content: []TextContent{{Type: "text", Text: fmt.Sprintf("tool %s not found", name)}}, IsError: true}, nil
	}
	res, err := t.Handler(ctx, args)
	if err != nil {
		return ToolsCallResult{Content: []TextContent{{Type: "text", Text: err.Error()}}, IsError: true}, nil
	}
	switch v := res.(type) {
	case string:
		return ToolsCallResult{Content: []TextContent{{Type: "text", Text: v}}}, nil
	case []byte:
		return ToolsCallResult{Content: []TextContent{{Type: "text", Text: string(v)}}}, nil
	default:
		b, _ := json.Marshal(v)
		return ToolsCallResult{Content: []TextContent{{Type: "text", Text: string(b)}}}, nil
	}
}
