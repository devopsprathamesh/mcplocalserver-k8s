#!/usr/bin/env bash
set -euo pipefail

ROOT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
cd "$ROOT_DIR"

echo "[validate] Building server..."
go build -o bin/mcp-server ./cmd/server

echo "[validate] Sending initialize and tools/list over framed stdio..."
python3 - <<'PY' | ./bin/mcp-server 2> /tmp/mcp-stderr.log | sed -n '1,200p'
import json, sys, time
def send(o):
    b=json.dumps(o,separators=(",", ":")).encode()
    sys.stdout.write(f"Content-Length: {len(b)}\r\n\r\n"); sys.stdout.flush()
    sys.stdout.buffer.write(b); sys.stdout.flush()
send({"jsonrpc":"2.0","id":1,"method":"initialize","params":{"clientInfo":{"name":"validate"}}})
time.sleep(0.1)
send({"jsonrpc":"2.0","id":2,"method":"tools/list"})
PY

echo
echo "[validate] stderr (server logs):"
sed -n '1,200p' /tmp/mcp-stderr.log || true

echo "[validate] Done."


