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
			return nil, errors.New("Kubernetes client not initialized yet")
		}
		reg.Register(mcp.Tool{Name: "cluster_health", Description: "Get basic cluster health and version", Handler: notReady})
		reg.Register(mcp.Tool{Name: "cluster_list_contexts", Description: "List kubeconfig contexts and current selection", Handler: notReady})
		reg.Register(mcp.Tool{Name: "cluster_set_context", Description: "Set current kube context", Handler: func(ctx context.Context, params json.RawMessage) (any, error) {
			return nil, errors.New("Kubernetes client not initialized yet")
		}})
		reg.Register(mcp.Tool{Name: "ns_list_namespaces", Description: "List namespaces", Handler: notReady})
		return
	}
	// cluster_health
	reg.Register(mcp.Tool{
		Name:        "cluster_health",
		Description: "Get basic cluster health and version",
		Handler: func(ctx context.Context, _ json.RawMessage) (any, error) {
			if err := authz.RateLimit("cluster_health", 10, 5); err != nil {
				return nil, err
			}
			v, err := k.Discovery.ServerVersion()
			if err != nil {
				return nil, err
			}
			out := map[string]any{"status": "ok", "clusterVersion": v.GitVersion, "timestamp": time.Now().UTC().Format(time.RFC3339)}
			return out, nil
		},
	})

	// cluster_list_contexts
	reg.Register(mcp.Tool{
		Name:        "cluster_list_contexts",
		Description: "List kubeconfig contexts and current selection",
		Handler: func(ctx context.Context, _ json.RawMessage) (any, error) {
			_ = authz.RateLimit("cluster_list_contexts", 10, 5)
			current, items, err := k.ListContexts()
			if err != nil {
				return nil, err
			}
			return map[string]any{"current": current, "contexts": items}, nil
		},
	})

	// cluster_set_context
	reg.Register(mcp.Tool{
		Name:        "cluster_set_context",
		Description: "Set current kube context",
		Handler: func(ctx context.Context, params json.RawMessage) (any, error) {
			_ = authz.RateLimit("cluster_set_context", 5, 2)
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

	// ns_list_namespaces
	reg.Register(mcp.Tool{
		Name:        "ns_list_namespaces",
		Description: "List namespaces",
		Handler: func(ctx context.Context, params json.RawMessage) (any, error) {
			_ = authz.RateLimit("ns_list_namespaces", 10, 5)
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
