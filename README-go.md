## mcp-k8s-server-go

Production-grade Go port of the MCP Kubernetes local server.

### Build
```bash
go build -o bin/mcp-server ./cmd/server
```

### Run
```bash
KUBECONFIG=/absolute/path/to/kubeconfig \
K8S_CONTEXT=$(kubectl config current-context) \
K8S_NAMESPACE=default \
MCP_K8S_READONLY=true \
LOG_LEVEL=info \
./bin/mcp-server
```

Wire it from your MCP client by pointing to the built binary.

### Supported tools
- cluster.health, cluster.listContexts, cluster.setContext, ns.listNamespaces
- pods.listPods, pods.get, pods.logs, pods.exec
- resources.get, resources.apply, resources.delete
- secrets.get, secrets.set

### Notes
- JSON-RPC 2.0 over stdio using LSP-style framing (Content-Length headers)
- Uses client-go with RESTMapper for dynamic resources and SSA
- Structured logs via slog to stderr; stdout reserved for protocol

Example initialize response shape:

```json
{"jsonrpc":"2.0","id":1,"result":{"serverInfo":{"name":"mcp-k8s-server","version":"0.1.0-go"},"capabilities":{"tools":{}}}}
```


