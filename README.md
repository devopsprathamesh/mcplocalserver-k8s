## mcp-k8s-server (Go)

A local Model Context Protocol (MCP) server for Kubernetes written in Go. It exposes safe, composable tools for day-to-day cluster introspection and opt-in write operations with strong safety guards.

### Who is this for?
- New to MCP and want to control Kubernetes from Copilot/LLMs
- DevOps/SREs who want read-mostly tooling with opt-in writes
- Developers who need quick pod logs, describe-like info, and dry-run applies

---

## 1) Prerequisites
- Go 1.22+
- A working Kubernetes context (kubectl works on your machine)
- Network access to the cluster API server

Validate kubectl first:
```bash
kubectl get ns
```
If this works, you’re good to proceed.

---

## 2) Build
```bash
go build -o bin/mcp-server ./cmd/server
```

---

## 3) Run
```bash
KUBECONFIG=/absolute/path/to/kubeconfig \
K8S_CONTEXT=$(kubectl config current-context) \
K8S_NAMESPACE=default \
MCP_K8S_READONLY=true \
LOG_LEVEL=info \
./bin/mcp-server
```
This runs the JSON-RPC server over stdio, suitable for MCP clients (like GitHub Copilot MCP) to connect.

---

## 4) Configuration (Environment Variables)
- `KUBECONFIG`: absolute path(s) to kubeconfig; supports ':'-separated multiple files
- `K8S_CONTEXT`: override the current kube context
- `K8S_NAMESPACE`: default namespace (default: `default`)
- `MCP_K8S_READONLY`: when `true`, blocks mutating tools (`apply`, `delete`, `exec`, `secrets.set`)
- `MCP_K8S_NAMESPACE_ALLOWLIST`: comma-separated namespaces allowed for writes
- `MCP_K8S_KIND_ALLOWLIST`: comma-separated resource kinds allowed for writes (e.g., `Deployment,Secret,ConfigMap`)
- `LOG_LEVEL`: `info` (default), `debug`, `warn`, `error`
- `MCP_K8S_TIMEOUT_MS`: request timeout in ms (best-effort; many APIs already handle timeouts)

Tips:
- If `MCP_K8S_READONLY=true`, mutating tools return a clear error.
- If allowlists are set, writes require namespace/kind ∈ allowlists.

---

## 4.1) Quick local test with k3d
```bash
# Create a local Kubernetes cluster
k3d cluster create mcp-k8s --agents 0 --servers 1

# Export kubeconfig and verify access
k3d kubeconfig get mcp-k8s > /tmp/kubeconfig-k3d-mcp-k8s
KUBECONFIG=/tmp/kubeconfig-k3d-mcp-k8s kubectl get ns

# Build and run the MCP server against k3d
go build -o bin/mcp-server ./cmd/server
KUBECONFIG=/tmp/kubeconfig-k3d-mcp-k8s \
LOG_LEVEL=info \
K8S_CONTEXT=$(KUBECONFIG=/tmp/kubeconfig-k3d-mcp-k8s kubectl config current-context) \
K8S_NAMESPACE=default \
MCP_K8S_READONLY=true \
./bin/mcp-server
```

Then, in your MCP client (e.g., Copilot), configure it to point to the built binary (see section 5).

---

## 5) Connect from GitHub Copilot (MCP client)
Create or edit `~/.copilot/mcp.json` (Mac/Linux) or the equivalent on your OS:
```json
{
  "mcpServers": {
    "k8s-local": {
      "command": "/absolute/path/to/mcp-k8s-server/bin/mcp-server",
      "args": [],
      "env": {
        "KUBECONFIG": "/home/user/.kube/config",
        "K8S_CONTEXT": "your-context",
        "K8S_NAMESPACE": "default",
        "MCP_K8S_READONLY": "true",
        "MCP_K8S_NAMESPACE_ALLOWLIST": "default,dev,staging",
        "MCP_K8S_KIND_ALLOWLIST": "Deployment,Secret,ConfigMap"
      }
    }
  }
}
```
Then restart Copilot (or your editor) so it picks up the config.

---

## 6) Quickstart: Ask Copilot to use this server
- "Use `k8s-local` to run `cluster.health`"
- "List pods in namespace `dev` with label `app=myapi`"
- "Get last 200 lines of logs for pod `api-abc` in `staging`"
- "(When readOnly=false, dryRun=true) Apply this manifest ..."

---

## 7) Safety Model
- **Read-only mode**: set `MCP_K8S_READONLY=true` to block mutating operations: `resources.apply`, `resources.delete`, `secrets.set`, `pods.exec`.
- **Namespace allowlist**: `MCP_K8S_NAMESPACE_ALLOWLIST=default,dev` restricts writes to those namespaces.
- **Kind allowlist**: `MCP_K8S_KIND_ALLOWLIST=Deployment,Secret` restricts writes to those kinds.
- **Dry-run first**: Mutating tools default to `dryRun=true` when applicable. Explicitly set `dryRun=false` to persist.
- **Secrets hygiene**: Values are never logged. `secrets.get` redacts values unless `showValues=true` and `MCP_K8S_READONLY` is not `true`.
- **403/RBAC handling**: If your account lacks RBAC permissions, tools will return clear errors.

---

## 8) Tools Reference (concise)
All tools return compact JSON (as text). Use selectors and limits to keep outputs small.

- **cluster.health** → `{ status, clusterVersion, timestamp }`
- **cluster.listContexts** → `{ current, contexts[] }`
- **cluster.setContext({ context })** → `{ current }`
- **ns.listNamespaces({ limit? })** → `{ namespaces[] }`

- **pods.listPods({ namespace?, labelSelector?, fieldSelector?, limit? })** → `{ pods[] }`
- **pods.get({ namespace, name })** → `{ metadata, status, containers[], initContainers[], events[] }`
- **pods.logs({ namespace, name, container?, tailLines?, sinceSeconds?, timestamps? })** → multiline text
- **pods.exec({ namespace, name, container?, command[] }) [mutating]** → `{ exitCode }`

- **resources.get({ group?, version, kind, name?, namespace?, labelSelector?, fieldSelector?, limit? })** → `{ item | items[] }`
- **resources.apply({ manifestYAML, fieldManager?, dryRun? }) [mutating]** → `{ results[] }`
- **resources.delete({ group?, version, kind, name, namespace?, propagationPolicy?, gracePeriodSeconds?, dryRun? }) [mutating]** → `{ status }`

- **secrets.get({ namespace, name, keys?, showValues? })** → `{ type, data }` (values redacted unless allowed)
- **secrets.set({ namespace, name, data, type?, base64Encoded?, createIfMissing?, dryRun? }) [mutating]** → `{ created|updated, name, keys }`

Notes:
- `group`+`version` is the API group/version. For core resources, `group` is empty and `version` like `v1`.
- For CRDs, pass the CRD's `apiVersion` split into `group` and `version`, and the CRD `kind`.

---

## 9) Examples
### Apply a sample deployment (dry-run)
```bash
cat examples/sample-deployment.yaml | pbcopy
```
Ask Copilot:
- "Use `k8s-local` → `resources.apply` with the manifest I provide (dryRun=true)"

### List pods in your default namespace
- "Use `k8s-local` → `pods.listPods({ labelSelector: "app=sample" })`"

### Get logs
- "Use `k8s-local` → `pods.logs({ namespace: "default", name: "sample-xxxxx" })`"

---

## 10) How it works (high level)
- The server implements JSON-RPC 2.0 over stdio using LSP-style framing (Content-Length headers)
- Kubernetes access is via client-go with:
  - Core APIs for pods/secrets/namespaces
  - Dynamic client + RESTMapper for generic GVK operations (including CRDs)
- Safety is enforced in `internal/authz` (read-only toggle, namespace/kind allowlists, basic rate limiting)

---

## 11) Local Testing with k3d (step-by-step)

Prereqs:
- k3d installed (e.g., `brew install k3d` on macOS)
- kubectl installed and in PATH

### A) Use an existing k3d cluster
1. List clusters:
```bash
k3d cluster list
```
2. Export kubeconfig for your cluster (replace `mycluster`):
```bash
k3d kubeconfig get mycluster > /tmp/kubeconfig-k3d-mycluster
```
3. Verify access:
```bash
KUBECONFIG=/tmp/kubeconfig-k3d-mycluster kubectl get ns
```
4. Run the MCP server:
```bash
GOFLAGS="-buildvcs=false" go build -o bin/mcp-server ./cmd/server
KUBECONFIG=/tmp/kubeconfig-k3d-mycluster \
LOG_LEVEL=info \
K8S_CONTEXT=$(KUBECONFIG=/tmp/kubeconfig-k3d-mycluster kubectl config current-context) \
K8S_NAMESPACE=default \
MCP_K8S_READONLY=true \
./bin/mcp-server
```

### B) Create a fresh k3d cluster
1. Create cluster:
```bash
k3d cluster create mcp-k8s --agents 0 --servers 1
```
2. Export kubeconfig and verify:
```bash
k3d kubeconfig get mcp-k8s > /tmp/kubeconfig-k3d-mcp-k8s
KUBECONFIG=/tmp/kubeconfig-k3d-mcp-k8s kubectl get ns
```
3. Run the MCP server:
```bash
GOFLAGS="-buildvcs=false" go build -o bin/mcp-server ./cmd/server
KUBECONFIG=/tmp/kubeconfig-k3d-mcp-k8s \
LOG_LEVEL=info \
K8S_CONTEXT=$(KUBECONFIG=/tmp/kubeconfig-k3d-mcp-k8s kubectl config current-context) \
K8S_NAMESPACE=default \
MCP_K8S_READONLY=true \
./bin/mcp-server
```

### C) Test with your MCP client (e.g., GitHub Copilot)
In Copilot chat, try:
- "Use `k8s-local` to run `cluster.health`"
- "Use `k8s-local` to run `ns.listNamespaces`"
- "Use `k8s-local` to run `pods.listPods` with `{ \"namespace\": \"default\" }`"

## 5.1) Local validation (no MCP client)
You can validate the server responds correctly by sending framed JSON-RPC requests over stdio.

```bash
# Build
go build -o bin/mcp-server ./cmd/server

# Send initialize and tools/list using a small Python helper
python3 - <<'PY' | ./bin/mcp-server 2> /tmp/mcp-stderr.log | sed -n '1,200p'
import json, sys

def send(obj):
    b = json.dumps(obj,separators=(",", ":")).encode("utf-8")
    sys.stdout.write(f"Content-Length: {len(b)}\r\n\r\n")
    sys.stdout.flush()
    sys.stdout.buffer.write(b)
    sys.stdout.flush()

send({"jsonrpc":"2.0","id":1,"method":"initialize","params":{"clientInfo":{"name":"test"}}})
send({"jsonrpc":"2.0","id":2,"method":"tools/list"})
PY
```
This should print two framed responses for initialize and tools/list.

### Tool call examples
You can also exercise tool calls directly via `tools/call`:

- cluster.health
```bash
python3 -c 'import sys,json
def send(o):
  b=json.dumps(o,separators=(",",":")).encode()
  sys.stdout.write(f"Content-Length: {len(b)}\r\n\r\n"); sys.stdout.flush()
  sys.stdout.buffer.write(b); sys.stdout.flush()
send({"jsonrpc":"2.0","id":3,"method":"tools/call","params":{"name":"cluster.health"}})' \
| ./bin/mcp-server 2>/tmp/mcp-stderr.log | sed -n "1,200p"
```

- ns.listNamespaces (limit 5)
```bash
python3 -c 'import sys,json
def send(o):
  b=json.dumps(o,separators=(",",":")).encode()
  sys.stdout.write(f"Content-Length: {len(b)}\r\n\r\n"); sys.stdout.flush()
  sys.stdout.buffer.write(b); sys.stdout.flush()
send({"jsonrpc":"2.0","id":4,"method":"tools/call","params":{"name":"ns.listNamespaces","arguments":{"limit":5}}})' \
| ./bin/mcp-server 2>/tmp/mcp-stderr.log | sed -n "1,200p"
```
