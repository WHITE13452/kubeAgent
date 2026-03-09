package tools

import (
	"encoding/json"
	"fmt"
	"kubeagent/cmd/utils"
	"strings"
)

// CreateTool creates a K8s resource via the ginK8s backend.
// The caller (agent/LLM) is responsible for generating the YAML content.
type CreateTool struct{}

func NewCreateTool() *CreateTool {
	return &CreateTool{}
}

func (c *CreateTool) Name() string {
	return "CreateTool"
}

func (c *CreateTool) Description() string {
	return "用于在 Kubernetes 集群中创建资源（Pod、Service、Deployment 等），需要提供资源 YAML 内容和资源类型"
}

func (c *CreateTool) ArgsSchema() string {
	return `{"type":"object","properties":{"yaml":{"type":"string","description":"要创建的 K8s 资源的 YAML 内容"},"resource":{"type":"string","description":"K8s 资源类型，例如 pod、service、deployment"}},"required":["yaml","resource"]}`
}

func (c *CreateTool) Execute(params map[string]any) (string, error) {
	yaml, ok := params["yaml"].(string)
	if !ok || yaml == "" {
		return "", fmt.Errorf("yaml is required")
	}
	resource, ok := params["resource"].(string)
	if !ok || resource == "" {
		return "", fmt.Errorf("resource is required")
	}

	resource = strings.ToLower(resource)
	requestBody, err := json.Marshal(map[string]string{"yaml": yaml})
	if err != nil {
		return "", fmt.Errorf("failed to marshal request: %w", err)
	}

	resp, err := utils.PostHTTP("http://localhost:8080/"+resource, requestBody)
	if err != nil {
		return "", fmt.Errorf("failed to create %s: %w", resource, err)
	}
	return resp, nil
}
