package mcp

import (
	"encoding/json"
	"time"
)

func mustJSON(v any) string { b, _ := json.Marshal(v); return string(b) }
func mcpNow() string        { return time.Now().UTC().Format(time.RFC3339) }
