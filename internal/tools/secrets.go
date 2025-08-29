package tools

import (
	"context"
	"encoding/base64"
	"encoding/json"
	"errors"

	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	"github.com/example/mcp-k8s-server-go/internal/authz"
	"github.com/example/mcp-k8s-server-go/pkg/k8s"
	"github.com/example/mcp-k8s-server-go/pkg/mcp"
)

func RegisterSecrets(reg *mcp.Registry, k *k8s.Clients) {
	if k == nil {
		notReady := func(ctx context.Context, _ json.RawMessage) (any, error) {
			return nil, errors.New("Kubernetes client not initialized yet")
		}
		reg.Register(mcp.Tool{Name: "secrets_get", Description: "Get a secret (redacted by default)", Handler: notReady})
		reg.Register(mcp.Tool{Name: "secrets_set", Description: "Create/update a secret with provided keys", Handler: notReady})
		return
	}
	// secrets_get
	reg.Register(mcp.Tool{
		Name:        "secrets_get",
		Description: "Get a secret (redacted by default)",
		Handler: func(ctx context.Context, params json.RawMessage) (any, error) {
			_ = authz.RateLimit("secrets_get", 10, 5)
			var p struct {
				Namespace, Name string
				Keys            []string
				ShowValues      *bool
			}
			if err := json.Unmarshal(params, &p); err != nil {
				return nil, err
			}
			s, err := k.Clientset.CoreV1().Secrets(p.Namespace).Get(ctx, p.Name, metav1.GetOptions{})
			if err != nil {
				return nil, err
			}
			keys := p.Keys
			if len(keys) == 0 {
				for k := range s.Data {
					keys = append(keys, k)
				}
			}
			show := p.ShowValues != nil && *p.ShowValues && !authz.IsReadOnly()
			data := map[string]string{}
			for _, key := range keys {
				v := s.Data[key]
				if show {
					data[key] = base64.StdEncoding.EncodeToString(v)
				} else {
					data[key] = "REDACTED"
				}
			}
			out := map[string]any{"type": s.Type, "data": data}
			return out, nil
		},
	})

	// secrets_set
	reg.Register(mcp.Tool{
		Name:        "secrets_set",
		Description: "Create/update a secret with provided keys",
		Handler: func(ctx context.Context, params json.RawMessage) (any, error) {
			_ = authz.RateLimit("secrets_set", 10, 5)
			var p struct {
				Namespace, Name string
				Data            map[string]string
				Type            *string
				Base64Encoded   *bool
				CreateIfMissing *bool
				DryRun          *bool
			}
			if err := json.Unmarshal(params, &p); err != nil {
				return nil, err
			}
			if err := authz.EnforceMutating("secrets_set", p.Namespace, "Secret"); err != nil {
				return nil, err
			}
			existing, _ := k.Clientset.CoreV1().Secrets(p.Namespace).Get(ctx, p.Name, metav1.GetOptions{})
			data := map[string][]byte{}
			for k, v := range p.Data {
				if p.Base64Encoded != nil && *p.Base64Encoded {
					b, err := base64.StdEncoding.DecodeString(v)
					if err != nil {
						return nil, err
					}
					data[k] = b
				} else {
					data[k] = []byte(v)
				}
			}
			typ := corev1.SecretType("Opaque")
			if p.Type != nil && *p.Type != "" {
				typ = corev1.SecretType(*p.Type)
			}
			sec := &corev1.Secret{TypeMeta: metav1.TypeMeta{APIVersion: "v1", Kind: "Secret"}, ObjectMeta: metav1.ObjectMeta{Name: p.Name, Namespace: p.Namespace}, Type: typ, Data: data}
			dr := []string{}
			if p.DryRun == nil || *p.DryRun {
				dr = []string{"All"}
			}
			if existing == nil {
				if p.CreateIfMissing != nil && !*p.CreateIfMissing {
					return map[string]any{"error": "Secret does not exist and createIfMissing=false"}, nil
				}
				res, err := k.Clientset.CoreV1().Secrets(p.Namespace).Create(ctx, sec, metav1.CreateOptions{DryRun: dr})
				if err != nil {
					return nil, err
				}
				return map[string]any{"created": true, "name": res.Name, "keys": keysOf(data)}, nil
			}
			res, err := k.Clientset.CoreV1().Secrets(p.Namespace).Update(ctx, sec, metav1.UpdateOptions{DryRun: dr})
			if err != nil {
				return nil, err
			}
			return map[string]any{"updated": true, "name": res.Name, "keys": keysOf(data)}, nil
		},
	})
}

func keysOf(m map[string][]byte) []string {
	out := make([]string, 0, len(m))
	for k := range m {
		out = append(out, k)
	}
	return out
}
