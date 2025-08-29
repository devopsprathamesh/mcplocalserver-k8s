#!/usr/bin/env bash
set -euo pipefail

ROOT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
cd "$ROOT_DIR"

# Build the server
GOFLAGS=${GOFLAGS:-}
CGO_ENABLED=0 go build $GOFLAGS -o bin/mcp-server ./cmd/server

# Send a single NDJSON initialize and exit
printf '{"jsonrpc":"2.0","id":1,"method":"initialize","params":{}}\n' | ./bin/mcp-server 2> /dev/stderr | sed -n '1,5p'
