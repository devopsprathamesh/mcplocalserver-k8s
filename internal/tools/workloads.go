package tools

import (
	"context"
	"encoding/json"
	"strings"

	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes/scheme"
	"k8s.io/client-go/tools/remotecommand"

	"github.com/example/mcp-k8s-server-go/internal/authz"
	"github.com/example/mcp-k8s-server-go/pkg/k8s"
	"github.com/example/mcp-k8s-server-go/pkg/mcp"
)

func RegisterWorkloads(reg *mcp.Registry, k *k8s.Clients) {
	// pods.listPods
	reg.Register(mcp.Tool{
		Name:        "pods.listPods",
		Description: "List pods with optional selectors",
		Handler: func(ctx context.Context, params json.RawMessage) (any, error) {
			_ = authz.RateLimit("pods.listPods", 10, 5)
			var p struct {
				Namespace     string `json:"namespace"`
				LabelSelector string `json:"labelSelector"`
				FieldSelector string `json:"fieldSelector"`
				Limit         *int   `json:"limit"`
			}
			_ = json.Unmarshal(params, &p)
			ns := p.Namespace
			if ns == "" {
				ns = k.DefaultNamespace
			}
			list, err := k.Clientset.CoreV1().Pods(ns).List(ctx, metav1.ListOptions{LabelSelector: p.LabelSelector, FieldSelector: p.FieldSelector})
			if err != nil {
				return nil, err
			}
			type row struct {
				Name, Namespace, Phase, Node string
				Restarts                     int32
				Age                          any
			}
			var rows []row
			for _, pod := range list.Items {
				var restarts int32
				for _, cs := range pod.Status.ContainerStatuses {
					restarts += cs.RestartCount
				}
				rows = append(rows, row{Name: pod.Name, Namespace: pod.Namespace, Phase: string(pod.Status.Phase), Node: pod.Spec.NodeName, Restarts: restarts, Age: pod.CreationTimestamp})
			}
			if p.Limit != nil && *p.Limit > 0 && len(rows) > *p.Limit {
				rows = rows[:*p.Limit]
			}
			return map[string]any{"pods": rows}, nil
		},
	})

	// pods.get
	reg.Register(mcp.Tool{
		Name:        "pods.get",
		Description: "Get a pod summary including containers and events",
		Handler: func(ctx context.Context, params json.RawMessage) (any, error) {
			_ = authz.RateLimit("pods.get", 10, 5)
			var p struct{ Namespace, Name string }
			if err := json.Unmarshal(params, &p); err != nil {
				return nil, err
			}
			pod, err := k.Clientset.CoreV1().Pods(p.Namespace).Get(ctx, p.Name, metav1.GetOptions{})
			if err != nil {
				return nil, err
			}
			var containers []map[string]string
			for _, c := range pod.Spec.Containers {
				containers = append(containers, map[string]string{"name": c.Name, "image": c.Image})
			}
			var initContainers []map[string]string
			for _, c := range pod.Spec.InitContainers {
				initContainers = append(initContainers, map[string]string{"name": c.Name, "image": c.Image})
			}
			// best-effort events via core/v1
			ev, _ := k.Clientset.CoreV1().Events(p.Namespace).List(ctx, metav1.ListOptions{FieldSelector: "involvedObject.name=" + p.Name})
			type evRow struct {
				Type, Reason, Message string
				Age                   any
			}
			var events []evRow
			if ev != nil {
				for _, e := range ev.Items {
					events = append(events, evRow{Type: e.Type, Reason: e.Reason, Message: e.Message, Age: e.LastTimestamp})
				}
				if len(events) > 10 {
					events = events[len(events)-10:]
				}
			}
			out := map[string]any{
				"metadata":       map[string]any{"name": pod.Name, "namespace": pod.Namespace, "uid": string(pod.UID), "creationTimestamp": pod.CreationTimestamp, "labels": pod.Labels},
				"status":         map[string]any{"phase": pod.Status.Phase, "podIP": pod.Status.PodIP, "hostIP": pod.Status.HostIP, "conditions": pod.Status.Conditions},
				"containers":     containers,
				"initContainers": initContainers,
				"events":         events,
			}
			return out, nil
		},
	})

	// pods.logs
	reg.Register(mcp.Tool{
		Name:        "pods.logs",
		Description: "Get pod logs (tail by default)",
		Handler: func(ctx context.Context, params json.RawMessage) (any, error) {
			_ = authz.RateLimit("pods.logs", 10, 5)
			var p struct {
				Namespace, Name, Container string
				TailLines                  *int64
				SinceSeconds               *int64
				Timestamps                 *bool
			}
			if err := json.Unmarshal(params, &p); err != nil {
				return nil, err
			}
			text, err := k.PodLogs(ctx, p.Namespace, p.Name, p.Container, p.TailLines, p.SinceSeconds, p.Timestamps)
			if err != nil {
				return nil, err
			}
			lines := strings.Split(strings.TrimRight(text, "\n"), "\n")
			if len(lines) > 1000 {
				lines = lines[len(lines)-1000:]
			}
			return strings.Join(lines, "\n"), nil
		},
	})

	// pods.exec – approximate: 0 if no error, 1 otherwise
	reg.Register(mcp.Tool{
		Name:        "pods.exec",
		Description: "Execute a command in a pod",
		Handler: func(ctx context.Context, params json.RawMessage) (any, error) {
			_ = authz.RateLimit("pods.exec", 5, 2)
			var p struct {
				Namespace, Name, Container string
				Command                    []string `json:"command"`
				DryRun                     *bool    `json:"dryRun"`
			}
			if err := json.Unmarshal(params, &p); err != nil {
				return nil, err
			}
			if err := authz.EnforceMutating("pods.exec", p.Namespace, "Pod"); err != nil {
				return nil, err
			}
			req := k.Clientset.CoreV1().RESTClient().Post().Resource("pods").Namespace(p.Namespace).Name(p.Name).SubResource("exec").Param("container", p.Container)
			req.VersionedParams(&corev1.PodExecOptions{Container: p.Container, Command: p.Command, Stdin: false, Stdout: false, Stderr: false, TTY: false}, scheme.ParameterCodec)
			executor, err := remotecommand.NewSPDYExecutor(k.RestConfig, "POST", req.URL())
			if err != nil {
				return nil, err
			}
			err = executor.StreamWithContext(ctx, remotecommand.StreamOptions{})
			exit := 0
			if err != nil {
				exit = 1
			}
			return map[string]int{"exitCode": exit}, nil
		},
	})
}
