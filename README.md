## mcp-k8s-server

A local Model Context Protocol (MCP) server for Kubernetes written in TypeScript/Node.js. It exposes safe, composable tools for day-to-day cluster introspection and opt-in write operations with strong safety guards.

### Who is this for?
- New to MCP and want to control Kubernetes from Copilot/LLMs
- DevOps/SREs who want read-mostly tooling with opt-in writes
- Developers who need quick pod logs, describe-like info, and dry-run applies

---

## 1) Prerequisites
- Node.js 18+
- A working Kubernetes context (kubectl works on your machine)
- Network access to the cluster API server

Validate kubectl first:
```bash
kubectl get ns
```
If this works, you’re good to proceed.

---

## 2) Install
```bash
cd mcp-k8s-server
npm i
```

---

## 3) Run (development)
```bash
npm run dev
```
This runs the JSON-RPC server over stdio, suitable for MCP clients (like GitHub Copilot MCP) to connect.

---

## 4) Build & Run (production)
```bash
npm run build
npm start
```
The compiled server entry is `dist/server.js`.

---

## 5) Configuration (Environment Variables)
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

## 6) Connect from GitHub Copilot (MCP client)
Create or edit `~/.copilot/mcp.json` (Mac/Linux) or the equivalent on your OS:
```json
{
  "mcpServers": {
    "k8s-local": {
      "command": "node",
      "args": ["/absolute/path/to/mcp-k8s-server/dist/server.js"],
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

## 7) Quickstart: Ask Copilot to use this server
- "Use `k8s-local` to run `cluster.health`"
- "List pods in namespace `dev` with label `app=myapi`"
- "Get last 200 lines of logs for pod `api-abc` in `staging`"
- "(When readOnly=false, dryRun=true) Apply this manifest ..."

---

## 8) Safety Model
- **Read-only mode**: set `MCP_K8S_READONLY=true` to block mutating operations: `resources.apply`, `resources.delete`, `secrets.set`, `pods.exec`.
- **Namespace allowlist**: `MCP_K8S_NAMESPACE_ALLOWLIST=default,dev` restricts writes to those namespaces.
- **Kind allowlist**: `MCP_K8S_KIND_ALLOWLIST=Deployment,Secret` restricts writes to those kinds.
- **Dry-run first**: Mutating tools default to `dryRun=true` when applicable. Explicitly set `dryRun=false` to persist.
- **Secrets hygiene**: Values are never logged. `secrets.get` redacts values unless `showValues=true` and `MCP_K8S_READONLY` is not `true`.
- **403/RBAC handling**: If your account lacks RBAC permissions, tools will return clear errors.

---

## 9) Tools Reference (concise)
All tools return compact JSON (as text). Use selectors and limits to keep outputs small.

- **cluster.health** → `{ status, clusterVersion, serverAddress, timestamp }`
- **cluster.listContexts** → `{ current, contexts[] }`
- **cluster.setContext({ context })** → `{ current }`
- **ns.listNamespaces({ limit? })** → `{ namespaces[] }`

- **pods.listPods({ namespace?, labelSelector?, fieldSelector?, limit? })** → `{ pods[] }`
- **pods.get({ namespace, name })** → `{ metadata, status, containers[], initContainers[], events[] }`
- **pods.logs({ namespace, name, container?, tailLines?, sinceSeconds?, timestamps? })** → multiline text
- **pods.exec({ namespace, name, container?, command[], timeoutSeconds? }) [mutating]** → `{ exitCode }`

- **resources.get({ group?, version, kind, name?, namespace?, labelSelector?, fieldSelector?, limit? })** → `{ item | items[] }`
- **resources.apply({ manifestYAML, serverSideApply?, fieldManager?, dryRun? }) [mutating]** → `{ results[] }`
- **resources.delete({ group?, version, kind, name, namespace?, propagationPolicy?, gracePeriodSeconds?, dryRun? }) [mutating]** → `{ status }`

- **secrets.get({ namespace, name, keys?, showValues? })** → `{ type, data }` (values redacted unless allowed)
- **secrets.set({ namespace, name, data, type?, base64Encoded?, createIfMissing?, dryRun? }) [mutating]** → `{ created|updated, name, keys }`

Notes:
- `group`+`version` is the API group/version. For core resources, `group` is empty and `version` like `v1`.
- For CRDs, pass the CRD's `apiVersion` split into `group` and `version`, and the CRD `kind`.

---

## 10) Examples
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

## 11) Troubleshooting
- **RBAC 403/Forbidden**: Your kube user lacks permissions. Fix your role/rolebinding/clusterrole.
- **Wrong context**: Set `K8S_CONTEXT` env or call `cluster.setContext`.
- **Large responses**: Use `limit`, `fieldSelector`, or `labelSelector`.
- **Timeouts/Network**: Ensure API server is reachable; adjust `MCP_K8S_TIMEOUT_MS` if needed.
- **Read-only blocks**: If a write tool is blocked, check `MCP_K8S_READONLY` and allowlists.

---

## 12) Project Structure
```
mcp-k8s-server/
├─ src/
│  ├─ server.ts
│  ├─ k8s.ts
│  ├─ schemas.ts
│  ├─ authz.ts
│  └─ tools/
│     ├─ cluster.ts
│     ├─ workloads.ts
│     ├─ resources.ts
│     └─ secrets.ts
├─ examples/
│  ├─ sample-deployment.yaml
│  └─ mcp.json
├─ package.json
├─ tsconfig.json
└─ README.md
```

---

## 13) Scripts
```json
{
  "dev": "nodemon --watch src --exec ts-node src/server.ts",
  "build": "tsc -p tsconfig.json",
  "start": "node dist/server.js",
  "lint": "eslint .",
  "format": "prettier -w ."
}
```

---

## 14) How it works (high level)
- The server uses `@modelcontextprotocol/sdk` to expose tools over stdio.
- Kubernetes access is via `@kubernetes/client-node` with `KubeConfig` and helper clients:
  - Core APIs for pods/secrets/namespaces
  - `KubernetesObjectApi` for generic GVK operations (including CRDs)
- Safety is enforced in `authz.ts` (read-only toggle, namespace/kind allowlists, basic rate limiting).
- All inputs are validated with `zod` (see `schemas.ts`).

---

## 15) Next steps / Stretch goals
- Port-forward start/stop tool
- Watch/streaming tools
- Generic diff/apply helper using server-side apply dry-run managed fields
- Docker image and optional metrics endpoint

Happy shipping!

---

## 16) Local Testing with k3d (step-by-step)

This guide helps you spin up or reuse a local Kubernetes cluster via k3d and point the MCP server at it.

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
4. Run the MCP server (production mode is simplest for ESM):
```bash
cd mcp-k8s-server
npm run build
KUBECONFIG=/tmp/kubeconfig-k3d-mycluster \
LOG_LEVEL=info \
K8S_CONTEXT=$(KUBECONFIG=/tmp/kubeconfig-k3d-mycluster kubectl config current-context) \
K8S_NAMESPACE=default \
MCP_K8S_READONLY=true \
npm start
```

Optional (development mode via ts-node):
```bash
# If you see ESM module resolution errors with ts-node, prefer the production mode above.
KUBECONFIG=/tmp/kubeconfig-k3d-mycluster \
LOG_LEVEL=info \
K8S_CONTEXT=$(KUBECONFIG=/tmp/kubeconfig-k3d-mycluster kubectl config current-context) \
K8S_NAMESPACE=default \
MCP_K8S_READONLY=true \
npm run dev
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
3. Run the MCP server (production mode recommended):
```bash
cd mcp-k8s-server
npm run build
KUBECONFIG=/tmp/kubeconfig-k3d-mcp-k8s \
LOG_LEVEL=info \
K8S_CONTEXT=$(KUBECONFIG=/tmp/kubeconfig-k3d-mcp-k8s kubectl config current-context) \
K8S_NAMESPACE=default \
MCP_K8S_READONLY=true \
npm start
```

### C) Test with your MCP client (e.g., GitHub Copilot)
In Copilot chat, try:
- "Use `k8s-local` to run `cluster.health`"
- "Use `k8s-local` to run `ns.listNamespaces`"
- "Use `k8s-local` to run `pods.listPods` with `{ \"namespace\": \"default\" }`"

### D) Troubleshooting (k3d specific)
- If `npm run dev` crashes with `ERR_MODULE_NOT_FOUND` for `*.js` in `src/*`:
  - Prefer `npm run build && npm start`, or
  - Adjust the dev script to use `ts-node --esm` and ensure ESM-compatible imports.
- If kubectl can’t reach the cluster, re-export kubeconfig and verify `K8S_CONTEXT`.
- If mutating tools are blocked, check `MCP_K8S_READONLY` and allowlists.
