package tools

import (
	"fmt"
	"os/exec"
	"strings"
)

// KubeTool executes read-only kubectl commands on the cluster.
//
// IMPORTANT: this is a diagnosis tool only. Attempts to use mutating
// verbs (patch / apply / edit / delete / create / scale / rollout /
// label / annotate) are rejected with an error that explicitly names
// the correct write tool. This is on purpose — if we only returned
// "not allowed", the LLM tends to retry the same command a few more
// times before giving up, which burns tool-loop iterations. Naming
// the replacement tool in the error body steers the LLM back to
// CreateTool / DeleteTool in one shot.
var allowedKubeCommands = map[string]bool{
	"get":          true,
	"describe":     true,
	"logs":         true,
	"top":          true,
	"explain":      true,
	"version":      true,
	"cluster-info": true,
}

// Known mutating verbs we explicitly redirect to write tools.
// Mapped to a short hint so the LLM sees the right alternative in
// the failure message.
var kubeWriteRedirects = map[string]string{
	"patch":    "use DeleteTool to let the controller recreate the resource, or CreateTool to submit a replacement YAML",
	"apply":    "use CreateTool with the full YAML",
	"create":   "use CreateTool with the full YAML",
	"edit":     "use CreateTool to submit the new YAML (after HumanTool approval)",
	"delete":   "use DeleteTool (after HumanTool approval)",
	"scale":    "use CreateTool to submit a patched Deployment YAML",
	"rollout":  "use DeleteTool on the target Pod to trigger a controller-managed restart",
	"label":    "use CreateTool to submit the updated YAML",
	"annotate": "use CreateTool to submit the updated YAML",
	"replace":  "use CreateTool with the full YAML",
	"set":      "use CreateTool with the full updated YAML",
}

type KubeTool struct{}

func NewKubeTool() *KubeTool {
	return &KubeTool{}
}

func (k *KubeTool) Name() string {
	return "KubeTool"
}

func (k *KubeTool) Description() string {
	// The description is the single most important signal the LLM uses
	// when picking a tool. Keep it: (1) short enough to fit the tool
	// registry, (2) explicit about read-only-ness, (3) listing the
	// allowed verbs so the model does not speculate, (4) naming the
	// correct write tool to use for mutations.
	return "只读的 kubectl 命令执行器。仅允许：get / describe / logs / top / explain / version / cluster-info。" +
		"禁止写操作（patch / apply / edit / delete / create / scale / rollout / label / annotate / replace / set）——" +
		"删除资源请使用 DeleteTool，创建或修改资源请使用 CreateTool（需先通过 HumanTool 获得确认）。"
}

func (k *KubeTool) ArgsSchema() string {
	// The schema's description mirrors the redirect hint so a model
	// that reads the schema but skims the tool description still sees
	// it.
	return `{"type":"object","properties":{"command":{"type":"string","description":"要运行的只读 kubectl 命令，例如 'kubectl get pods -n default'、'kubectl describe pod foo'。写操作（patch/apply/delete/...）不支持，请改用 DeleteTool 或 CreateTool。"}},"required":["command"]}`
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

	subcommand := parts[1]
	if !allowedKubeCommands[subcommand] {
		// If it's a known write verb, name the correct replacement
		// tool so the LLM pivots immediately instead of retrying the
		// same command. This trades a slightly longer error string
		// for significantly fewer wasted tool-loop iterations.
		if hint, ok := kubeWriteRedirects[subcommand]; ok {
			return "", fmt.Errorf(
				"kubectl '%s' is a write operation and is not supported by KubeTool — %s",
				subcommand, hint,
			)
		}
		return "", fmt.Errorf(
			"kubectl subcommand '%s' is not allowed (only read-only commands permitted: get, describe, logs, top, explain, version, cluster-info)",
			subcommand,
		)
	}

	output, err := exec.Command(parts[0], parts[1:]...).Output()
	if err != nil {
		return "", fmt.Errorf("kubectl command failed: %w", err)
	}
	return string(output), nil
}
