package tools

import (
	"fmt"
	"kubeagent/cmd/utils"
)

// LogTool gets pod logs from the ginK8s backend
type LogTool struct{}

func NewLogTool() *LogTool {
	return &LogTool{}
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

	url := "http://localhost:8080/pods/logs?namespace=" + namespace + "&podName=" + podName + "&container=" + container
	resp, err := utils.GetHTTP(url)
	if err != nil {
		return "", fmt.Errorf("failed to get pod logs: %w", err)
	}
	return resp, nil
}
