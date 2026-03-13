package tools

import (
	"bufio"
	"fmt"
	"os"
	"strings"
)

// HumanTool asks a human for confirmation before performing dangerous operations.
type HumanTool struct{}

func NewHumanTool() *HumanTool {
	return &HumanTool{}
}

func (h *HumanTool) Name() string {
	return "HumanTool"
}

func (h *HumanTool) Description() string {
	return "当需要执行不可逆的危险操作（如删除资源）时，先向用户寻求确认。用户输入 yes/y 表示确认，其他输入表示拒绝。"
}

func (h *HumanTool) ArgsSchema() string {
	return `{"type":"object","properties":{"prompt":{"type":"string","description":"向用户展示的确认信息，描述即将执行的操作"}},"required":["prompt"]}`
}

func (h *HumanTool) Execute(params map[string]any) (string, error) {
	prompt, ok := params["prompt"].(string)
	if !ok || prompt == "" {
		return "", fmt.Errorf("prompt is required")
	}

	// Check for auto-approve mode (useful for testing)
	if os.Getenv("AUTO_APPROVE") == "true" {
		fmt.Printf("\n[HumanTool] Auto-approved (AUTO_APPROVE=true): %s\n", prompt)
		return "auto-approved", nil
	}

	fmt.Printf("\n[Human Approval Required] %s\n请输入 yes/y 确认，其他内容取消: ", prompt)

	reader := bufio.NewReader(os.Stdin)
	input, err := reader.ReadString('\n')
	if err != nil {
		return "", fmt.Errorf("failed to read user input: %w", err)
	}

	input = strings.TrimSpace(strings.ToLower(input))
	if input == "yes" || input == "y" {
		return "approved", nil
	}
	return "rejected", nil
}
