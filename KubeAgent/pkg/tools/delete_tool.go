package tools

import (
	"fmt"
	"strings"

	"kubeagent/pkg/k8s"
)

// DeleteTool deletes a K8s resource directly via the Kubernetes API.
// Dangerous: must only be called after HumanTool returns "approved".
type DeleteTool struct {
	client *k8s.Client
}

func NewDeleteTool(client *k8s.Client) *DeleteTool {
	return &DeleteTool{client: client}
}

func (d *DeleteTool) Name() string {
	return "DeleteTool"
}

func (d *DeleteTool) Description() string {
	return "用于删除 Kubernetes 集群中的指定资源，这是危险操作，必须先通过 HumanTool 获得用户确认"
}

func (d *DeleteTool) ArgsSchema() string {
	return `{"type":"object","properties":{"resource":{"type":"string","description":"K8s 资源类型，例如 pod、service"},"name":{"type":"string","description":"资源实例的名称"},"namespace":{"type":"string","description":"资源所在命名空间"}},"required":["resource","name","namespace"]}`
}

func (d *DeleteTool) Execute(params map[string]any) (string, error) {
	resource, ok := params["resource"].(string)
	if !ok || resource == "" {
		return "", fmt.Errorf("resource is required")
	}
	name, ok := params["name"].(string)
	if !ok || name == "" {
		return "", fmt.Errorf("name is required")
	}
	namespace, ok := params["namespace"].(string)
	if !ok || namespace == "" {
		namespace = "default"
	}

	resource = strings.ToLower(resource)
	return d.client.DeleteResource(resource, name, namespace)
}
