package tools

import (
	"fmt"
	"strings"

	"kubeagent/pkg/k8s"
)

// ListTool lists K8s resources directly from the Kubernetes API.
type ListTool struct {
	client *k8s.Client
}

func NewListTool(client *k8s.Client) *ListTool {
	return &ListTool{client: client}
}

func (l *ListTool) Name() string {
	return "ListTool"
}

func (l *ListTool) Description() string {
	return "用于列出指定命名空间下的 Kubernetes 资源，例如 pods、deployments、services 等"
}

func (l *ListTool) ArgsSchema() string {
	return `{"type":"object","properties":{"resource":{"type":"string","description":"指定的 K8s 资源类型，例如 pods、deployments、services"},"namespace":{"type":"string","description":"指定的 Kubernetes 命名空间"}},"required":["resource","namespace"]}`
}

func (l *ListTool) Execute(params map[string]any) (string, error) {
	resource, ok := params["resource"].(string)
	if !ok || resource == "" {
		return "", fmt.Errorf("resource is required")
	}
	namespace, ok := params["namespace"].(string)
	if !ok || namespace == "" {
		namespace = "default"
	}

	resource = strings.ToLower(resource)
	return l.client.ListResources(resource, namespace)
}
