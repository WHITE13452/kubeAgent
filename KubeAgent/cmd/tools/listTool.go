package tools

import (
	"kubeagent/cmd/utils"
	"strings"
)

type ListTool struct {
	Name        string `json:"name"`
	Description string `json:"description"`
	ArgsSchema  string `json:"args_schema"`
}

type ListToolParam struct {
	Resource  string `json:"resource"`  // Resource is the type of Kubernetes resource to list, e.g., "pods", "deployments"
	Namespace string `json:"namespace"` // Namespace is the namespace of the resource, e.g., "default", "kube-system"
}

func NewListTool() *ListTool {
	return &ListTool{
		Name:        "ListTool",
		Description: "用于列出指定命名空间下的 Kubernetes 资源，例如列出某个命名空间下的所有 pods",
		ArgsSchema:  `{"type":"object","properties":{"resource":{"type":"string", "description": "指定的 k8s 资源类型，例如 pods, deployments 等等"},"namespace":{"type":"string", "description": "指定的 k8s 命名空间"}}}`,
	}
}

func (l *ListTool) Run(resource, namespace string) string {
	resource = strings.ToLower(resource)
	url := "http://localhost:8080/" + resource + "?namespace=" + namespace
	response, err := utils.GetHTTP(url)
	if err != nil {
		return "Error listing resources: " + err.Error()
	}

	return response
}