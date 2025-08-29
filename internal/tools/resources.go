package tools

import (
	"context"
	"encoding/json"
	"errors"
	"strings"

	"github.com/example/mcp-k8s-server-go/internal/authz"
	"github.com/example/mcp-k8s-server-go/pkg/k8s"
	"github.com/example/mcp-k8s-server-go/pkg/mcp"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/apimachinery/pkg/types"
)

func RegisterResources(reg *mcp.Registry, k *k8s.Clients) {
	if k == nil {
		notReady := func(ctx context.Context, _ json.RawMessage) (any, error) {
			return nil, errors.New("Kubernetes client not initialized yet")
		}
		reg.Register(mcp.Tool{Name: "resources-get", Description: "Get or list arbitrary resources by GVK", DirectResult: true, Handler: notReady})
		reg.Register(mcp.Tool{Name: "resources-apply", Description: "Apply manifest YAML (server-side apply by default)", DirectResult: true, Handler: notReady})
		reg.Register(mcp.Tool{Name: "resources-delete", Description: "Delete a resource by GVK/name", DirectResult: true, Handler: notReady})
		return
	}
	// resources-get
	reg.Register(mcp.Tool{
		Name:         "resources-get",
		Description:  "Get or list arbitrary resources by GVK",
		DirectResult: true,
		Handler: func(ctx context.Context, params json.RawMessage) (any, error) {
			_ = authz.RateLimit("resources-get", 10, 5)
			var p struct {
				Group         *string
				Version       string
				Kind          string
				Name          *string
				Namespace     *string
				LabelSelector *string
				FieldSelector *string
				Limit         *int
			}
			if err := json.Unmarshal(params, &p); err != nil {
				return nil, err
			}
			gvk := schema.GroupVersionKind{Group: func() string {
				if p.Group == nil {
					return ""
				}
				return *p.Group
			}(), Version: p.Version, Kind: p.Kind}
			gvr, err := k.ResolveResource(gvk)
			if err != nil {
				return nil, err
			}
			ns := ""
			if p.Namespace != nil {
				ns = *p.Namespace
			} else {
				ns = k.DefaultNamespace
			}
			nri := k.Dynamic.Resource(gvr).Namespace(ns)
			if p.Name != nil && *p.Name != "" {
				item, err := nri.Get(ctx, *p.Name, metav1.GetOptions{})
				if err != nil {
					return nil, err
				}
				return map[string]any{"item": item.Object}, nil
			}
			list, err := nri.List(ctx, metav1.ListOptions{LabelSelector: ptrStr(p.LabelSelector), FieldSelector: ptrStr(p.FieldSelector)})
			if err != nil {
				return nil, err
			}
			var summary []map[string]any
			for _, it := range list.Items {
				summary = append(summary, map[string]any{"apiVersion": it.GetAPIVersion(), "kind": it.GetKind(), "name": it.GetName(), "namespace": it.GetNamespace(), "uid": string(it.GetUID()), "creationTimestamp": it.GetCreationTimestamp()})
			}
			if p.Limit != nil && *p.Limit > 0 && len(summary) > *p.Limit {
				summary = summary[:*p.Limit]
			}
			return map[string]any{"items": summary}, nil
		},
	})

	// resources-apply (server-side apply)
	reg.Register(mcp.Tool{
		Name:         "resources-apply",
		Description:  "Apply manifest YAML (server-side apply by default)",
		DirectResult: true,
		Handler: func(ctx context.Context, params json.RawMessage) (any, error) {
			_ = authz.RateLimit("resources-apply", 10, 5)
			var p struct {
				ManifestYAML string  `json:"manifestYAML"`
				FieldManager *string `json:"fieldManager"`
				DryRun       *bool   `json:"dryRun"`
			}
			if err := json.Unmarshal(params, &p); err != nil {
				return nil, err
			}
			docs := splitYAMLDocs(p.ManifestYAML)
			var results []map[string]any
			for _, d := range docs {
				if strings.TrimSpace(d) == "" {
					continue
				}
				obj, err := k8s.DecodeYAMLToUnstructured([]byte(d))
				if err != nil {
					results = append(results, map[string]any{"error": err.Error()})
					continue
				}
				if err := authz.EnforceMutating("resources-apply", obj.GetNamespace(), obj.GetKind()); err != nil {
					results = append(results, map[string]any{"error": err.Error()})
					continue
				}
				gvk := obj.GroupVersionKind()
				gvr, err := k.ResolveResource(gvk)
				if err != nil {
					results = append(results, map[string]any{"error": err.Error()})
					continue
				}
				ns := obj.GetNamespace()
				ri := k.Dynamic.Resource(gvr).Namespace(ns)
				fm := "mcp-k8s-server"
				if p.FieldManager != nil && *p.FieldManager != "" {
					fm = *p.FieldManager
				}
				dr := []string{}
				if p.DryRun == nil || *p.DryRun {
					dr = []string{"All"}
				}
				applied, err := ri.Patch(ctx, obj.GetName(), types.ApplyPatchType, []byte(d), metav1.PatchOptions{FieldManager: fm, Force: ptrBool(true), DryRun: dr})
				if err != nil {
					results = append(results, map[string]any{"error": err.Error()})
					continue
				}
				results = append(results, map[string]any{"kind": applied.GetKind(), "name": applied.GetName(), "namespace": applied.GetNamespace()})
			}
			return map[string]any{"results": results}, nil
		},
	})

	// resources-delete
	reg.Register(mcp.Tool{
		Name:         "resources-delete",
		Description:  "Delete a resource by GVK/name",
		DirectResult: true,
		Handler: func(ctx context.Context, params json.RawMessage) (any, error) {
			_ = authz.RateLimit("resources-delete", 10, 5)
			var p struct {
				Group               *string
				Version, Kind, Name string
				Namespace           *string
				PropagationPolicy   *string
				GracePeriodSeconds  *int64
				DryRun              *bool
			}
			if err := json.Unmarshal(params, &p); err != nil {
				return nil, err
			}
			gvk := schema.GroupVersionKind{Group: ptrStr(p.Group), Version: p.Version, Kind: p.Kind}
			gvr, err := k.ResolveResource(gvk)
			if err != nil {
				return nil, err
			}
			ns := ptrStr(p.Namespace)
			ri := k.Dynamic.Resource(gvr).Namespace(ns)
			if err := authz.EnforceMutating("resources-delete", ns, p.Kind); err != nil {
				return nil, err
			}
			dr := []string{}
			if p.DryRun == nil || *p.DryRun {
				dr = []string{"All"}
			}
			var pp *metav1.DeletionPropagation
			if p.PropagationPolicy != nil {
				s := *p.PropagationPolicy
				switch s {
				case "Foreground":
					v := metav1.DeletePropagationForeground
					pp = &v
				case "Background":
					v := metav1.DeletePropagationBackground
					pp = &v
				case "Orphan":
					v := metav1.DeletePropagationOrphan
					pp = &v
				}
			}
			opts := metav1.DeleteOptions{DryRun: dr, GracePeriodSeconds: p.GracePeriodSeconds, PropagationPolicy: pp}
			if err := ri.Delete(ctx, p.Name, opts); err != nil {
				return nil, err
			}
			return map[string]any{"status": "Success"}, nil
		},
	})
}

func splitYAMLDocs(s string) []string {
	parts := strings.Split(s, "\n---")
	return parts
}

func ptrStr(p *string) string {
	if p == nil {
		return ""
	}
	return *p
}
func ptrBool(v bool) *bool { b := v; return &b }
