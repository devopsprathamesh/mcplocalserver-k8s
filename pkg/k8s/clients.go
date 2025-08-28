package k8s

import (
	"context"
	"fmt"
	"log/slog"
	"os"
	"path/filepath"
	"strings"

	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/apimachinery/pkg/runtime/serializer/yaml"
	"k8s.io/client-go/discovery"
	"k8s.io/client-go/dynamic"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/rest"
	"k8s.io/client-go/restmapper"
	"k8s.io/client-go/tools/clientcmd"
)

type Clients struct {
	Logger           *slog.Logger
	RestConfig       *rest.Config
	Clientset        *kubernetes.Clientset
	Dynamic          dynamic.Interface
	Discovery        discovery.DiscoveryInterface
	DefaultNamespace string
	// kubeconfig paths (for context switching)
	kubeconfigPaths []string
}

func Load(ctx context.Context, logger *slog.Logger) (*Clients, error) {
	// Load order: KUBECONFIG (supports ':'), in-cluster, default
	var cfg *rest.Config
	var kcPaths []string
	if env := os.Getenv("KUBECONFIG"); env != "" {
		parts := strings.Split(env, string(os.PathListSeparator))
		for _, p := range parts {
			if p != "" {
				kcPaths = append(kcPaths, p)
			}
		}
		rules := &clientcmd.ClientConfigLoadingRules{Precedence: kcPaths}
		overrides := &clientcmd.ConfigOverrides{}
		c := clientcmd.NewNonInteractiveDeferredLoadingClientConfig(rules, overrides)
		var err error
		cfg, err = c.ClientConfig()
		if err != nil {
			return nil, fmt.Errorf("failed to load kubeconfig: %w", err)
		}
	} else {
		var err error
		cfg, err = rest.InClusterConfig()
		if err != nil {
			// fallback to default kubeconfig
			home, _ := os.UserHomeDir()
			path := filepath.Join(home, ".kube", "config")
			if _, statErr := os.Stat(path); statErr == nil {
				rules := &clientcmd.ClientConfigLoadingRules{ExplicitPath: path}
				overrides := &clientcmd.ConfigOverrides{}
				c := clientcmd.NewNonInteractiveDeferredLoadingClientConfig(rules, overrides)
				cfg, err = c.ClientConfig()
				if err != nil {
					return nil, fmt.Errorf("failed to load default kubeconfig: %w", err)
				}
				kcPaths = []string{path}
			} else {
				return nil, fmt.Errorf("no kubeconfig found and not running in-cluster")
			}
		}
	}
	cs, err := kubernetes.NewForConfig(cfg)
	if err != nil {
		return nil, err
	}
	dyn, err := dynamic.NewForConfig(cfg)
	if err != nil {
		return nil, err
	}
	disc, err := discovery.NewDiscoveryClientForConfig(cfg)
	if err != nil {
		return nil, err
	}
	ns := os.Getenv("K8S_NAMESPACE")
	if ns == "" {
		ns = "default"
	}
	return &Clients{Logger: logger, RestConfig: cfg, Clientset: cs, Dynamic: dyn, Discovery: disc, DefaultNamespace: ns, kubeconfigPaths: kcPaths}, nil
}

// SwitchContext attempts to switch kube context by name when kubeconfig is present.
func (c *Clients) SwitchContext(ctx context.Context, contextName string) error {
	if len(c.kubeconfigPaths) == 0 {
		return fmt.Errorf("context switching not available (in-cluster)")
	}
	rules := &clientcmd.ClientConfigLoadingRules{Precedence: c.kubeconfigPaths}
	overrides := &clientcmd.ConfigOverrides{CurrentContext: contextName}
	loader := clientcmd.NewNonInteractiveDeferredLoadingClientConfig(rules, overrides)
	cfg, err := loader.ClientConfig()
	if err != nil {
		return err
	}
	cs, err := kubernetes.NewForConfig(cfg)
	if err != nil {
		return err
	}
	dyn, err := dynamic.NewForConfig(cfg)
	if err != nil {
		return err
	}
	disc, err := discovery.NewDiscoveryClientForConfig(cfg)
	if err != nil {
		return err
	}
	c.RestConfig, c.Clientset, c.Dynamic, c.Discovery = cfg, cs, dyn, disc
	return nil
}

// ListContexts returns current and list of contexts when kubeconfig is present.
func (c *Clients) ListContexts() (current string, contexts []struct{ Name, Cluster, User string }, err error) {
	if len(c.kubeconfigPaths) == 0 {
		return "", nil, fmt.Errorf("contexts unavailable in in-cluster mode")
	}
	rules := &clientcmd.ClientConfigLoadingRules{Precedence: c.kubeconfigPaths}
	raw, err := rules.Load()
	if err != nil {
		return "", nil, err
	}
	current = raw.CurrentContext
	for name, ctx := range raw.Contexts {
		contexts = append(contexts, struct{ Name, Cluster, User string }{Name: name, Cluster: ctx.Cluster, User: ctx.AuthInfo})
	}
	return
}

// PodLogs returns the logs for a pod/container with options.
func (c *Clients) PodLogs(ctx context.Context, namespace, name, container string, tailLines *int64, sinceSeconds *int64, timestamps *bool) (string, error) {
	opts := &corev1.PodLogOptions{Container: container, TailLines: tailLines, SinceSeconds: sinceSeconds, Timestamps: false}
	if timestamps != nil && *timestamps {
		opts.Timestamps = true
	}
	req := c.Clientset.CoreV1().Pods(namespace).GetLogs(name, opts)
	b, err := req.Do(ctx).Raw()
	if err != nil {
		return "", err
	}
	return string(b), nil
}

// Resolve GVK to GVR using discovery and RESTMapper
func (c *Clients) ResolveResource(gvk schema.GroupVersionKind) (schema.GroupVersionResource, error) {
	mapper := restmapper.NewDeferredDiscoveryRESTMapper(memoryCached{c.Discovery})
	m, err := mapper.RESTMapping(schema.GroupKind{Group: gvk.Group, Kind: gvk.Kind}, gvk.Version)
	if err != nil {
		return schema.GroupVersionResource{}, err
	}
	return m.Resource, nil
}

// YAML to Unstructured helpers
var decUnstructured = yaml.NewDecodingSerializer(unstructured.UnstructuredJSONScheme)

func DecodeYAMLToUnstructured(doc []byte) (*unstructured.Unstructured, error) {
	obj := &unstructured.Unstructured{}
	_, _, err := decUnstructured.Decode(doc, nil, obj)
	return obj, err
}

// memoryCached wraps discovery to satisfy RESTMapper cache interface methods used by restmapper.
type memoryCached struct{ discovery.DiscoveryInterface }

func (m memoryCached) Fresh() bool { return true }
func (m memoryCached) Invalidate() {}
