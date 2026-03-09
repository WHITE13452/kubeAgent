package tools

import (
	"fmt"

	"kubeagent/pkg/k8s"
)

// LogTool gets pod logs directly from the Kubernetes API.
type LogTool struct {
	client *k8s.Client
}

func NewLogTool(client *k8s.Client) *LogTool {
	return &LogTool{client: client}
}

func (l *LogTool) Name() string {
	return "LogTool"
}

func (l *LogTool) Description() string {
	return "用于查看 K8s Pod 的日志，支持指定容器名称"
}

func (l *LogTool) ArgsSchema() string {
	return `{"type":"object","properties":{"podName":{"type":"string","description":"指定的 Pod 名称"},"namespace":{"type":"string","description":"指定的 Kubernetes 命名空间"},"container":{"type":"string","description":"指定的容器名称（可选）"}},"required":["podName","namespace"]}`
}

func (l *LogTool) Execute(params map[string]any) (string, error) {
	podName, ok := params["podName"].(string)
	if !ok || podName == "" {
		return "", fmt.Errorf("podName is required")
	}
	namespace, ok := params["namespace"].(string)
	if !ok || namespace == "" {
		namespace = "default"
	}
	container, _ := params["container"].(string)

	return l.client.GetPodLogs(podName, namespace, container)
}
