package tools

import (
	"context"
	"fmt"
	"os/exec"
	"strings"
	"time"
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

// KubeToolConfig holds configuration for KubeTool
type KubeToolConfig struct {
	MaxRetries    int           // Maximum retry attempts
	RetryDelay    time.Duration // Delay between retries
	CommandTimeout time.Duration // Timeout for each command
}

var defaultKubeToolConfig = KubeToolConfig{
	MaxRetries:    3,
	RetryDelay:    1 * time.Second,
	CommandTimeout: 30 * time.Second,
}

type KubeTool struct {
	config *KubeToolConfig
}

func NewKubeTool() *KubeTool {
	return &KubeTool{
		config: &defaultKubeToolConfig,
	}
}

// WithConfig allows customizing KubeTool behavior
func (k *KubeTool) WithConfig(config *KubeToolConfig) *KubeTool {
	k.config = config
	return k
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

	// Execute with retry logic
	var lastErr error
	for attempt := 1; attempt <= k.config.MaxRetries; attempt++ {
		output, err := k.executeCommand(parts)
		if err == nil {
			return output, nil
		}

		lastErr = err
		
		// Check if error is retryable
		errMsg := strings.ToLower(err.Error())
		isRetryable := strings.Contains(errMsg, "connection refused") ||
			strings.Contains(errMsg, "timeout") ||
			strings.Contains(errMsg, "temporary failure")
		
		// Don't retry for non-retryable errors
		if !isRetryable {
			// Enhance error message with helpful information
			return "", k.enhanceError(err, command)
		}

		// Check if we have retries left
		if attempt < k.config.MaxRetries {
			time.Sleep(k.config.RetryDelay * time.Duration(attempt))
		}
	}

	return "", k.enhanceError(lastErr, command)
}

func (k *KubeTool) executeCommand(parts []string) (string, error) {
	ctx, cancel := context.WithTimeout(context.Background(), k.config.CommandTimeout)
	defer cancel()
	
	cmd := exec.CommandContext(ctx, parts[0], parts[1:]...)
	output, err := cmd.Output()
	return string(output), err
}

func (k *KubeTool) enhanceError(err error, command string) error {
	errMsg := err.Error()
	
	// Provide more helpful error messages based on the error type
	if strings.Contains(errMsg, "not found") || strings.Contains(errMsg, "doesn't exist") {
		return fmt.Errorf("resource not found. The requested resource may not exist in the cluster or may be in a different namespace. Original error: %v", err)
	}
	
	if strings.Contains(errMsg, "forbidden") || strings.Contains(errMsg, "denied") || strings.Contains(errMsg, "Unauthorized") {
		return fmt.Errorf("permission denied. You don't have access to this resource. Check your RBAC permissions. Original error: %v", err)
	}
	
	if strings.Contains(errMsg, "connection refused") {
		return fmt.Errorf("cannot connect to Kubernetes API server. Please check if the cluster is running and accessible. Original error: %v", err)
	}
	
	if strings.Contains(errMsg, "no such host") || strings.Contains(errMsg, "could not resolve") {
		return fmt.Errorf("cannot resolve Kubernetes API server host. Check your kubeconfig and network connection. Original error: %v", err)
	}
	
	if strings.Contains(errMsg, "metrics not available") || strings.Contains(errMsg, "metrics.k8s.io") {
		return fmt.Errorf("metrics-server not installed. Run 'kubectl apply -f https://github.com/kubernetes-sigs/metrics-server/releases/latest/download/components.yaml' to install. Original error: %v", err)
	}
	
	// Default: return original error with command context
	return fmt.Errorf("kubectl command failed for '%s': %v", command, err)
}
