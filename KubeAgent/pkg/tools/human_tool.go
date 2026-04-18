package tools

import (
	"bufio"
	"fmt"
	"os"
	"strings"
	"time"
)

// HumanTool asks a human for confirmation before performing dangerous operations.
type HumanTool struct {
	timeout time.Duration
}

func NewHumanTool() *HumanTool {
	return &HumanTool{
		timeout: 5 * time.Minute, // Default 5 minute timeout
	}
}

// WithTimeout sets the timeout for HumanTool
func (h *HumanTool) WithTimeout(timeout time.Duration) *HumanTool {
	h.timeout = timeout
	return h
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

	// Check for auto-reject mode (non-interactive)
	if os.Getenv("AUTO_REJECT") == "true" || os.Getenv("CI") == "true" {
		fmt.Printf("\n[HumanTool] Auto-rejected (AUTO_REJECT=true or CI=true): %s\n", prompt)
		return "auto-rejected", nil
	}

	fmt.Printf("\n[Human Approval Required] %s\n", prompt)
	fmt.Printf("请在 %d 分钟内输入 yes/y 确认，其他内容取消: ", int(h.timeout/time.Minute))

	// Set up timeout
	resultChan := make(chan string, 1)
	errChan := make(chan error, 1)
	
	go func() {
		reader := bufio.NewReader(os.Stdin)
		input, err := reader.ReadString('\n')
		if err != nil {
			errChan <- err
			return
		}
		input = strings.TrimSpace(strings.ToLower(input))
		if input == "yes" || input == "y" {
			resultChan <- "approved"
		} else {
			resultChan <- "rejected"
		}
	}()

	select {
	case result := <-resultChan:
		if result == "approved" {
			return "approved", nil
		}
		return "rejected", nil
	case err := <-errChan:
		return "", fmt.Errorf("failed to read user input: %w", err)
	case <-time.After(h.timeout):
		return "", fmt.Errorf("timeout waiting for human approval (waited %v)", h.timeout)
	}
}
