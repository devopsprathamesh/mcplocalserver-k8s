package mcp

import (
	"bytes"
	"context"
	"io"
	"log/slog"
	"os"
	"strconv"
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
}

func fmtInt(n int) string { return strconv.Itoa(n) }
