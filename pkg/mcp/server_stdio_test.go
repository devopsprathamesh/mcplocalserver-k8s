package mcp

import (
	"bytes"
	"context"
	"encoding/json"
	"io"
	"log/slog"
	"os"
	"strconv"
	"strings"
	"testing"
)

func TestInitializeAndList(t *testing.T) {
	logger := slog.New(slog.NewTextHandler(os.Stderr, &slog.HandlerOptions{Level: slog.LevelError}))
	srv := NewServer(logger)
	// Prepare framed request: initialize then tools/list
	req1 := []byte(`{"jsonrpc":"2.0","id":1,"method":"initialize","params":{"clientInfo":{"name":"test"}}}`)
	req2 := []byte(`{"jsonrpc":"2.0","id":2,"method":"tools/list"}`)
	var in bytes.Buffer
	in.WriteString("Content-Length: ")
	in.WriteString((func(n int) string { return fmtInt(n) })(len(req1)))
	in.WriteString("\r\n\r\n")
	in.Write(req1)
	in.WriteString("Content-Length: ")
	in.WriteString((func(n int) string { return fmtInt(n) })(len(req2)))
	in.WriteString("\r\n\r\n")
	in.Write(req2)

	var out bytes.Buffer
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	err := srv.Run(ctx, &in, &out)
	if err != nil && err != io.EOF {
		t.Fatalf("run error: %v", err)
	}
	if out.Len() == 0 {
		t.Fatalf("no output")
	}
	// Parse two framed responses and validate structure
	resp1, rest := readFrame(out.Bytes())
	if resp1 == nil {
		t.Fatalf("missing resp1")
	}
	var r1 map[string]any
	_ = json.Unmarshal(resp1, &r1)
	if r1["jsonrpc"] != "2.0" || r1["result"] == nil {
		t.Fatalf("bad initialize resp: %v", r1)
	}
	resp2, _ := readFrame(rest)
	if resp2 == nil {
		t.Fatalf("missing resp2")
	}
	var r2 map[string]any
	_ = json.Unmarshal(resp2, &r2)
	if r2["result"] == nil {
		t.Fatalf("bad tools/list resp: %v", r2)
	}
}

func fmtInt(n int) string { return strconv.Itoa(n) }

func readFrame(b []byte) ([]byte, []byte) {
	s := string(b)
	const hdr = "Content-Length: "
	i := strings.Index(s, hdr)
	if i != 0 {
		return nil, nil
	}
	j := strings.Index(s, "\r\n\r\n")
	if j < 0 {
		return nil, nil
	}
	lnStr := strings.TrimSpace(s[len(hdr):j])
	n, _ := strconv.Atoi(lnStr)
	start := j + 4
	if start+n > len(b) {
		return nil, nil
	}
	return b[start : start+n], b[start+n:]
}
