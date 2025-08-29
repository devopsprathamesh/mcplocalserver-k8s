# MCP Kubernetes Server (Go)

A minimal, production-ready Model Context Protocol (MCP) server in Go that exposes Kubernetes operations as MCP tools for GitHub Copilot and compatible MCP clients. It communicates over stdio using JSON-RPC 2.0 and supports both LSP-style framed transport and NDJSON.

## Features

- Non-blocking `initialize`: server responds immediately, background Kubernetes setup follows
- Tools for Kubernetes cluster, contexts, namespaces, resources, pods, and secrets
- All tool names MCP-compliant (kebab-case): `[a-z0-9-]`
- Logs only to stderr; JSON-RPC responses only to stdout
- Framed (Content-Length) and NDJSON modes for easy testing

## Requirements

- Go 1.21+
- Optional: Kubernetes access
  - `KUBECONFIG` for out-of-cluster
  - or In-cluster config when running inside Kubernetes
- Recommended environment variables
  - `KUBECONFIG`: colon-separated paths or single path
  - `K8S_NAMESPACE`: default namespace (default: `default`)
  - `MCP_K8S_TIMEOUT_MS`: per tool-call timeout in ms (default: no timeout)

## Build

```bash
go build -o bin/mcp-server ./cmd/server
```

## Run (standalone)

- NDJSON (easiest):

```bash
echo '{"jsonrpc":"2.0","id":1,"method":"initialize","params":{}}' | ./bin/mcp-server
```

- Framed (LSP-style Content-Length):

```bash
python3 - <<'PY' | ./bin/mcp-server
import json,sys
msg={"jsonrpc":"2.0","id":1,"method":"initialize","params":{"clientInfo":{"name":"cli"}}}
b=json.dumps(msg,separators=(",", ":")).encode()
sys.stdout.write(f"Content-Length: {len(b)}\r\n\r\n"); sys.stdout.flush()
sys.stdout.buffer.write(b); sys.stdout.flush()
PY
```

- Scripts:
  - `./scripts/test-handshake.sh` – quick NDJSON handshake
  - `./scripts/validate.sh` – framed handshake and tools/list

## VS Code GitHub Copilot MCP configuration

Create a `config.json` (or use the MCP settings UI) pointing to the compiled server. See `examples/mcp.json` for a template. Example:

```json
{
  "mcpServers": {
    "k8s": {
      "command": "/absolute/path/to/bin/mcp-server",
      "args": [],
      "env": {
        "KUBECONFIG": "/Users/you/.kube/config",
        "K8S_NAMESPACE": "default",
        "MCP_K8S_TIMEOUT_MS": "15000"
      }
    }
  }
}
```

Notes:
- The server must run via stdio; logs go to stderr, do not redirect stderr to stdout.
- Copilot MCP expects framed transport; this server auto-detects and supports both framed and NDJSON.

## Available tools (kebab-case)

- cluster
  - `cluster-health`: Get basic cluster health and version
  - `cluster-list-contexts`: List kubeconfig contexts and current selection
  - `cluster-set-context`: Set current kube context
  - `ns-list-namespaces`: List namespaces
- workloads
  - `pods-list-pods`: List pods with optional selectors
  - `pods-get`: Get a pod summary including containers and events
  - `pods-logs`: Get pod logs (tail by default)
  - `pods-exec`: Execute a command in a pod
- resources
  - `resources-get`: Get or list arbitrary resources by GVK
  - `resources-apply`: Apply manifest YAML (server-side apply by default)
  - `resources-delete`: Delete a resource by GVK/name
- secrets
  - `secrets-get`: Get a secret (redacted by default)
  - `secrets-set`: Create/update a secret with provided keys
- utility
  - `echo`: Echo back the provided text (works without Kubernetes)

Some tools require the Kubernetes client to be initialized. If not ready, they return "Kubernetes client not initialized yet".

## Examples

- List tools (NDJSON):

```bash
printf '{"jsonrpc":"2.0","id":1,"method":"initialize","params":{}}\n{"jsonrpc":"2.0","id":2,"method":"tools/list"}\n' | ./bin/mcp-server
```

- Call echo (returns string directly):

```bash
printf '{"jsonrpc":"2.0","id":1,"method":"initialize","params":{}}\n{"jsonrpc":"2.0","id":3,"method":"tools/call","params":{"name":"echo","arguments":{"text":"hello"}}}\n' | ./bin/mcp-server
```

- Call cluster-health (wrapped in MCP content/json):

```bash
printf '{"jsonrpc":"2.0","id":1,"method":"initialize","params":{}}\n{"jsonrpc":"2.0","id":4,"method":"tools/call","params":{"name":"cluster-health","arguments":{}}}\n' | ./bin/mcp-server
```

- List pods (requires Kubernetes access):

```bash
export KUBECONFIG=~/.kube/config
printf '{"jsonrpc":"2.0","id":1,"method":"initialize","params":{}}\n{"jsonrpc":"2.0","id":5,"method":"tools/call","params":{"name":"pods-list-pods","arguments":{"namespace":"default","limit":2}}}\n' | ./bin/mcp-server
```

## Troubleshooting

- Initialize seems to block
  - This server responds instantly. If you don’t see a response, ensure your client sends framed headers correctly or uses NDJSON with newlines.
- No tools or Kubernetes errors
  - You’ll always see `echo` and placeholders. If you get `Kubernetes client not initialized yet`, verify `KUBECONFIG` or in-cluster environment.
- Tool name rejected by Copilot
  - Tool names must match `[a-z0-9-]`. All built-in tools are kebab-case.
- Logs mixed with responses
  - Logs go to stderr only. Don’t merge stderr into stdout in your client.
- Framed mode specifics
  - Ensure `Content-Length` and `\r\n\r\n` are correct; each request must be fully framed.

## Project layout

- `cmd/server` – main entry point (stdio JSON-RPC loop)
- `pkg/mcp` – protocol types, transport, server loop, registry
- `pkg/k8s` – Kubernetes client loader and helpers
- `internal/tools` – tool registrations and handlers (cluster, workloads, resources, secrets)
- `scripts` – validation and handshake scripts
- `examples` – example MCP configuration for Copilot

## License

Apache-2.0 (or project default).
