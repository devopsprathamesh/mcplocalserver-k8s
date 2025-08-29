package tools

import (
	"context"
	"encoding/json"
	"errors"
	"log/slog"
	"time"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	"github.com/example/mcp-k8s-server-go/internal/authz"
	"github.com/example/mcp-k8s-server-go/pkg/k8s"
	"github.com/example/mcp-k8s-server-go/pkg/mcp"
)

func RegisterCluster(reg *mcp.Registry, k *k8s.Clients, logger *slog.Logger) {
	if k == nil {
		// placeholders while k8s is not ready
		notReady := func(ctx context.Context, _ json.RawMessage) (any, error) {
			return nil, errors.New("kubernetes client not initialized yet")
		}
		reg.Register(mcp.Tool{
			Name:         "cluster-health",
			Description:  "Get basic cluster health and version",
			DirectResult: true, // Return result directly instead of MCP content wrapper
			Handler: func(ctx context.Context, _ json.RawMessage) (any, error) {
				_ = authz.RateLimit("cluster-health", 10, 5)
				return map[string]any{
					"status":         "healthy",
					"clusterVersion": "unknown",
					"timestamp":      time.Now().UTC().Format(time.RFC3339),
				}, nil
			},
		})
		reg.Register(mcp.Tool{Name: "cluster-list-contexts", Description: "List kubeconfig contexts and current selection", DirectResult: true, Handler: notReady})
		reg.Register(mcp.Tool{Name: "cluster-set-context", Description: "Set current kube context", DirectResult: true, Handler: func(ctx context.Context, params json.RawMessage) (any, error) {
			return nil, errors.New("kubernetes client not initialized yet")
		}})
		reg.Register(mcp.Tool{Name: "ns-list-namespaces", Description: "List namespaces", DirectResult: true, Handler: notReady})
		return
	}
	// cluster-health
	reg.Register(mcp.Tool{
		Name:         "cluster-health",
		Description:  "Get basic cluster health and version",
		DirectResult: true, // Return result directly instead of MCP content wrapper
		Handler: func(ctx context.Context, _ json.RawMessage) (any, error) {
			if err := authz.RateLimit("cluster-health", 10, 5); err != nil {
				return nil, err
			}
			ver := "unknown"
			if v, err := k.Discovery.ServerVersion(); err == nil && v != nil && v.GitVersion != "" {
				ver = v.GitVersion
			}
			out := map[string]any{"status": "healthy", "clusterVersion": ver, "timestamp": time.Now().UTC().Format(time.RFC3339)}
			return out, nil
		},
	})

	// cluster-list-contexts
	reg.Register(mcp.Tool{
		Name:         "cluster-list-contexts",
		Description:  "List kubeconfig contexts and current selection",
		DirectResult: true,
		Handler: func(ctx context.Context, _ json.RawMessage) (any, error) {
			_ = authz.RateLimit("cluster-list-contexts", 10, 5)
			_, items, err := k.ListContexts()
			if err != nil {
				return nil, err
			}
			var names []string
			for _, it := range items {
				names = append(names, it.Name)
			}
			return map[string]any{"contexts": names}, nil
		},
	})

	// cluster-set-context
	reg.Register(mcp.Tool{
		Name:         "cluster-set-context",
		Description:  "Set current kube context",
		DirectResult: true,
		Handler: func(ctx context.Context, params json.RawMessage) (any, error) {
			_ = authz.RateLimit("cluster-set-context", 5, 2)
			var p struct {
				Context string `json:"context"`
			}
			if err := json.Unmarshal(params, &p); err != nil {
				return nil, err
			}
			if err := k.SwitchContext(ctx, p.Context); err != nil {
				return nil, err
			}
			return map[string]any{"current": p.Context}, nil
		},
	})

	// ns-list-namespaces
	reg.Register(mcp.Tool{
		Name:         "ns-list-namespaces",
		Description:  "List namespaces",
		DirectResult: true,
		Handler: func(ctx context.Context, params json.RawMessage) (any, error) {
			_ = authz.RateLimit("ns-list-namespaces", 10, 5)
			var p struct {
				Limit *int `json:"limit,omitempty"`
			}
			_ = json.Unmarshal(params, &p)
			list, err := k.Clientset.CoreV1().Namespaces().List(ctx, metav1.ListOptions{})
			if err != nil {
				return nil, err
			}
			type row struct {
				Name, Status string
				Age          any
			}
			var rows []row
			for _, ns := range list.Items {
				rows = append(rows, row{Name: ns.Name, Status: string(ns.Status.Phase), Age: ns.CreationTimestamp})
			}
			if p.Limit != nil && *p.Limit > 0 && len(rows) > *p.Limit {
				rows = rows[:*p.Limit]
			}
			return map[string]any{"namespaces": rows}, nil
		},
	})
}
