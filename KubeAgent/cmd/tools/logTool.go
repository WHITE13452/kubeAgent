package tools

import "kubeagent/cmd/utils"

type LogToolParam struct {
	PodName   string `json:"podName"`   
	Namespace string `json:"namespace"` 
	Container string `json:"container"`
}

type LogTool struct {
	Name        string `json:"name"`
	Description string `json:"description"`
	ArgsSchema  string `json:"argsSchema"`
}

func NewLogTool() *LogTool {
	return &LogTool{
		Name:        "LogTool",
		Description: "用于查看K8S 的 Pod 的日志",
		ArgsSchema:  `{"type":"object","properties":{"podName":{"type":"string", "description": "指定的 Pod 名称"},"namespace":{"type":"string", "description": "指定的 Kubernetes 命名空间"},"container":{"type":"string", "description": "指定的容器名称"}}}`,
	}
}

func (l *LogTool) Run(podName, namespace, containerName string) (string, error) {
	url := "http://localhost:8080/pods/logs?namespace=" + namespace + "&podName=" + podName + "&container=" + containerName
	resp, err := utils.GetHTTP(url)
	if err != nil {
		return "", err
	}
	return resp, nil
}