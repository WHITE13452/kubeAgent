package tools

import (
	"fmt"
	"os/exec"
	"strings"
)

type KubeInput struct {
	Commands string
}

type KubeTool struct {
	Name        string `json:"name"`
	Description string `json:"description"`
	ArgsSchema  KubeInput `json:"args_schema"`
}

func NewKubeTool() *KubeTool {
	return &KubeTool{
		Name:        "KubeTool",
		Description: "用于在 Kubernetes 集群上运行 k8s 相关命令（kubectl、helm）的工具。",
		ArgsSchema:  KubeInput{`description: "要运行的 kubectl/helm 相关命令。" example: "kubectl get pods -n default"`},
	}
}

func (k *KubeTool) Run(commands string) (string, error) {
	parsedCommands := k.parseCommends(commands)
	splitedCommands := k.splitCommands(parsedCommands)
	cmd := exec.Command(splitedCommands[0], splitedCommands[1:]...)

	output, err := cmd.Output()
	if err != nil {
		fmt.Println("Error executing command:", err)
		return "", err
	}

	return fmt.Sprintf("运行结果: %s", output), nil
}

// sometime the input is like "kubectl get pods -n default", we need to parse it
func (k *KubeTool) parseCommends(commands string) string { 
	return strings.TrimSpace(strings.Trim(commands, "\"'"))
}

func (k *KubeTool) splitCommands(commands string) []string { 
	return strings.Split(commands, " ")
}