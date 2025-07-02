package tools

import (
	"fmt"
	"kubeagent/cmd/utils"
	"strings"
)

type DeleteToolParam struct {
	Resource string `json:"resource"`
	Namespace string `json:"namespace"`
	Name string `json:"name"`
}

type DeleteTool struct {
	Name        string `json:"name"`
	Description string `json:"description"`
	ArgsSchema  string `json:"args_schema"`
}

func NewDeleteTool() *DeleteTool {
	return &DeleteTool{
		Name:        "DeleteTool",
		Description: "用于删除指定命名空间下的 Kubernetes 资源，例如删除某个命名空间下的某个 pod",
		ArgsSchema:  `{"type":"object","properties":{"resource":{"type":"string", "description": "指定的 k8s 资源类型，例如 pod, service等等"}, "name":{"type":"string", "description": "指定的某 k8s 资源实例的名称"}, "namespace":{"type":"string", "description": "指定的 k8s 资源所在命名空间"}}`,
	}
}

func (d *DeleteTool) Run(resource, name, namespace string) string {
	resource = strings.ToLower(resource)
	url := "http://localhost:8080/resource/" + resource + "?namespace=" + namespace + "&name=" + name
	fmt.Println(url)
	response, err := utils.DeleteHTTP(url)
	if err != nil {
		return "Error deleting resources: " + err.Error()
	}

	return response
}