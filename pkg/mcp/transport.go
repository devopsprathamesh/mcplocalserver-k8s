package mcp

import (
	"bufio"
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"strconv"
	"strings"
)

// LSP-style header framed transport: `Content-Length: N\r\n\r\n<JSON>`

type framedReader struct {
	r *bufio.Reader
}

func newFramedReader(r io.Reader) *framedReader {
	return &framedReader{r: bufio.NewReader(r)}
}

func (fr *framedReader) ReadMessage() ([]byte, error) {
	// Read headers until empty line
	var contentLen int
	for {
		line, err := fr.r.ReadString('\n')
		if err != nil {
			return nil, err
		}
		line = strings.TrimRight(line, "\r\n")
		if line == "" { // header terminator
			break
		}
		if strings.HasPrefix(strings.ToLower(line), "content-length:") {
			parts := strings.SplitN(line, ":", 2)
			if len(parts) == 2 {
				n, err := strconv.Atoi(strings.TrimSpace(parts[1]))
				if err != nil {
					return nil, fmt.Errorf("invalid content-length: %w", err)
				}
				contentLen = n
			}
		}
	}
	if contentLen <= 0 {
		return nil, io.EOF
	}
	buf := make([]byte, contentLen)
	if _, err := io.ReadFull(fr.r, buf); err != nil {
		return nil, err
	}
	return buf, nil
}

type framedWriter struct {
	w *bufio.Writer
}

func newFramedWriter(w io.Writer) *framedWriter { return &framedWriter{w: bufio.NewWriter(w)} }

func (fw *framedWriter) WriteJSON(v any) error {
	b, err := json.Marshal(v)
	if err != nil {
		return err
	}
	var out bytes.Buffer
	fmt.Fprintf(&out, "Content-Length: %d\r\n\r\n", len(b))
	out.Write(b)
	if _, err = fw.w.Write(out.Bytes()); err != nil {
		return err
	}
	return fw.w.Flush()
}
