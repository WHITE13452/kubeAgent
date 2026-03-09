package tools

import (
	"fmt"

	"kubeagent/pkg/k8s"
)

// CreateTool creates a K8s resource directly via the Kubernetes API.
// The caller (agent/LLM) is responsible for generating the YAML content.
type CreateTool struct {
	client *k8s.Client
}

func NewCreateTool(client *k8s.Client) *CreateTool {
	return &CreateTool{client: client}
}

func (c *CreateTool) Name() string {
	return "CreateTool"
}

func (c *CreateTool) Description() string {
	return "用于在 Kubernetes 集群中创建资源（Pod、Service、Deployment 等），需要提供资源 YAML 内容"
}

func (c *CreateTool) ArgsSchema() string {
	return `{"type":"object","properties":{"yaml":{"type":"string","description":"要创建的 K8s 资源的 YAML 内容"}},"required":["yaml"]}`
}

func (c *CreateTool) Execute(params map[string]any) (string, error) {
	yamlContent, ok := params["yaml"].(string)
	if !ok || yamlContent == "" {
		return "", fmt.Errorf("yaml is required")
	}

	return c.client.CreateResource(yamlContent)
}
