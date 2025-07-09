package tools

import (
	"kubeagent/cmd/utils"
)

type EventToolParam struct {
	PodName   string `json:"podName"`
	Namespace string `json:"namespace"`
}

type EventTool struct {
	Name        string `json:"name"`
	Description string `json:"description"`
	ArgsSchema  string `json:"argsSchema"`
}

func NewEventTool() *EventTool {
	return &EventTool{
		Name:        "EventTool",
		Description: "用于查看 k8s pod 的 event 事件",
		ArgsSchema:  `{"type":"object","properties":{"podName":{"type":"string", "description": "指定的 pod 名称"}, "namespace":{"type":"string", "description": "指定的 k8s 命名空间"}}`,
	}
}

func (e *EventTool) Run(podName, nameSpace string) (string, error) {
	url := "http://localhost:8080/pods/events" + "?namespace=" + nameSpace + "&podName=" + podName
	resp, err := utils.GetHTTP(url)
	if err != nil {
		return "", err
	}
	return resp, nil
}
