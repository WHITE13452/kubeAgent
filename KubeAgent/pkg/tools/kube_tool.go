package tools

import (
	"fmt"
	"os/exec"
	"strings"
)

// KubeTool executes read-only kubectl commands on the cluster.
var allowedKubeCommands = map[string]bool{
	"get":          true,
	"describe":     true,
	"logs":         true,
	"top":          true,
	"explain":      true,
	"version":      true,
	"cluster-info": true,
}

type KubeTool struct{}

func NewKubeTool() *KubeTool {
	return &KubeTool{}
}

func (k *KubeTool) Name() string {
	return "KubeTool"
}

func (k *KubeTool) Description() string {
	return "用于在 Kubernetes 集群上运行只读的 kubectl 命令（get、describe、logs、top），不允许写操作"
}

func (k *KubeTool) ArgsSchema() string {
	return `{"type":"object","properties":{"command":{"type":"string","description":"要运行的 kubectl 命令，例如 'kubectl get pods -n default'"}},"required":["command"]}`
}

func (k *KubeTool) Execute(params map[string]any) (string, error) {
	command, ok := params["command"].(string)
	if !ok || command == "" {
		return "", fmt.Errorf("command is required")
	}

	command = strings.TrimSpace(strings.Trim(command, "\"'"))
	parts := strings.Fields(command)
	if len(parts) == 0 {
		return "", fmt.Errorf("empty command")
	}
	if parts[0] != "kubectl" {
		return "", fmt.Errorf("only kubectl commands are allowed, got: %s", parts[0])
	}
	if len(parts) < 2 {
		return "", fmt.Errorf("kubectl command requires a subcommand")
	}
	if !allowedKubeCommands[parts[1]] {
		return "", fmt.Errorf("kubectl subcommand '%s' is not allowed (only read-only commands permitted)", parts[1])
	}

	output, err := exec.Command(parts[0], parts[1:]...).Output()
	if err != nil {
		return "", fmt.Errorf("kubectl command failed: %w", err)
	}
	return string(output), nil
}
