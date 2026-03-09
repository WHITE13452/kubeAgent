package tools

import (
	"fmt"

	"kubeagent/pkg/k8s"
)

// EventTool gets K8s warning events for a pod directly from the Kubernetes API.
type EventTool struct {
	client *k8s.Client
}

func NewEventTool(client *k8s.Client) *EventTool {
	return &EventTool{client: client}
}

func (e *EventTool) Name() string {
	return "EventTool"
}

func (e *EventTool) Description() string {
	return "用于查看 K8s Pod 的 Event 事件，可以帮助诊断 Pod 启动失败等问题"
}

func (e *EventTool) ArgsSchema() string {
	return `{"type":"object","properties":{"podName":{"type":"string","description":"指定的 Pod 名称"},"namespace":{"type":"string","description":"指定的 Kubernetes 命名空间"}},"required":["podName","namespace"]}`
}

func (e *EventTool) Execute(params map[string]any) (string, error) {
	podName, ok := params["podName"].(string)
	if !ok || podName == "" {
		return "", fmt.Errorf("podName is required")
	}
	namespace, ok := params["namespace"].(string)
	if !ok || namespace == "" {
		namespace = "default"
	}

	return e.client.GetPodEvents(podName, namespace)
}
