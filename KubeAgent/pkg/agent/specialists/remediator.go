package specialists

import (
	"encoding/json"
	"fmt"
	"time"

	"kubeagent/pkg/agent"
)

// RemediatorAgent specializes in generating and applying fixes
type RemediatorAgent struct {
	*agent.BaseAgent
}

// NewRemediatorAgent creates a new remediator agent
func NewRemediatorAgent(llmClient agent.LLMClient, logger agent.Logger) *RemediatorAgent {
	config := &agent.AgentConfig{
		Name:        "remediator",
		Type:        agent.AgentTypeRemediator,
		Description: "Generates fixes, creates patches, and remediates issues",
		MaxRetries:  3,
		Timeout:     2 * time.Minute,
	}

	return &RemediatorAgent{
		BaseAgent: agent.NewBaseAgent(config, llmClient, logger),
	}
}

// CanHandle checks if the remediator can handle a task type
func (r *RemediatorAgent) CanHandle(taskType agent.TaskType) bool {
	return taskType == agent.TaskTypeRemediate
}

// Execute executes a remediation task
func (r *RemediatorAgent) Execute(ctx *agent.AgentContext, task *agent.Task) (*agent.Task, error) {
	startTime := time.Now()

	task.Status = agent.TaskStatusRunning
	now := time.Now()
	task.StartedAt = &now

	rootCause, _ := task.Input["root_cause"].(string)
	errorType, _ := task.Input["error_type"].(string)
	diagnosis, _ := task.Input["diagnosis"].(map[string]any)

	if rootCause == "" {
		rootCause = task.Description
	}

	// Generate remediation plan via LLM
	remediation, err := r.generateRemediation(ctx, rootCause, errorType, diagnosis)
	if err != nil {
		task.Status = agent.TaskStatusFailed
		task.Error = err.Error()
		completedAt := time.Now()
		task.CompletedAt = &completedAt
		return task, err
	}

	// If plan requires approval, ask human before proceeding
	requiresApproval, _ := remediation["requires_approval"].(bool)
	patch, _ := remediation["patch"].(string)
	approvalStatus := "not_required"

	if requiresApproval {
		prompt := fmt.Sprintf("即将执行修复操作:\n根因: %s\n修复方案:\n%s\n\n是否确认执行?", rootCause, patch)
		decision := r.askHuman(prompt)
		approvalStatus = decision
		if decision == "rejected" {
			task.Status = agent.TaskStatusCancelled
			task.Error = "remediation rejected by user"
			completedAt := time.Now()
			task.CompletedAt = &completedAt
			return task, nil
		}
	}

	// Apply the remediation if approved or not requiring approval
	applyResult := r.applyRemediation(remediation)

	task.Status = agent.TaskStatusCompleted
	task.Output = map[string]any{
		"remediation_type":   remediation["remediation_type"],
		"patch":              patch,
		"verification_steps": remediation["verification_steps"],
		"requires_approval":  requiresApproval,
		"risk_level":         remediation["risk_level"],
		"approval_status":    approvalStatus,
		"apply_result":       applyResult,
		"remediation_time":   time.Since(startTime).String(),
	}

	completedAt := time.Now()
	task.CompletedAt = &completedAt

	return task, nil
}

// Analyze implements SpecialistAgent
func (r *RemediatorAgent) Analyze(ctx *agent.AgentContext, input map[string]any) (map[string]any, error) {
	rootCause, _ := input["root_cause"].(string)
	errorType, _ := input["error_type"].(string)
	diagnosis, _ := input["diagnosis"].(map[string]any)

	return r.generateRemediation(ctx, rootCause, errorType, diagnosis)
}

// generateRemediation calls LLM to produce a structured remediation plan
func (r *RemediatorAgent) generateRemediation(ctx *agent.AgentContext, rootCause, errorType string, diagnosis map[string]any) (map[string]any, error) {
	systemPrompt := `You are a Kubernetes remediation expert. Generate fixes for diagnosed issues.

Your task is to:
1. Generate appropriate remediation actions (patch, configuration change, etc.)
2. Provide verification steps
3. Assess risk level
4. Determine if human approval is required

Return your remediation plan in JSON format:
{
  "remediation_type": "patch|config_change|restart|scale",
  "patch": "YAML patch content or commands",
  "verification_steps": ["Step 1", "Step 2"],
  "requires_approval": true,
  "risk_level": "low|medium|high"
}`

	diagnosisJSON, _ := json.Marshal(diagnosis)
	userPrompt := fmt.Sprintf(`Generate a remediation plan for the following issue:

Root Cause: %s
Error Type: %s
Diagnosis Details: %s

Provide a comprehensive remediation plan in JSON format.`, rootCause, errorType, string(diagnosisJSON))

	response, err := r.CallLLM(ctx, systemPrompt, userPrompt)
	if err != nil {
		return nil, fmt.Errorf("remediation generation failed: %w", err)
	}

	var remediation map[string]any
	if err := json.Unmarshal([]byte(response), &remediation); err != nil {
		return map[string]any{
			"remediation_type":   "manual",
			"patch":              response,
			"verification_steps": []string{"Apply patch and verify pod status"},
			"requires_approval":  true,
			"risk_level":         "medium",
		}, nil
	}

	return remediation, nil
}

// askHuman calls HumanTool if registered; returns "approved", "rejected", or "skipped" if tool not found
func (r *RemediatorAgent) askHuman(prompt string) string {
	for _, tool := range r.GetTools() {
		if tool.Name() == "HumanTool" {
			result, err := tool.Execute(map[string]any{"prompt": prompt})
			if err != nil {
				return "rejected"
			}
			return result
		}
	}
	// No HumanTool registered — default to approved for non-interactive scenarios
	return "approved"
}

// applyRemediation attempts to execute the remediation using registered tools.
// Returns a status message; actual K8s mutations go through CreateTool/DeleteTool.
func (r *RemediatorAgent) applyRemediation(remediation map[string]any) string {
	remediationType, _ := remediation["remediation_type"].(string)
	patch, _ := remediation["patch"].(string)

	switch remediationType {
	case "patch", "config_change":
		// Try KubeTool for kubectl-based patches if registered
		for _, tool := range r.GetTools() {
			if tool.Name() == "KubeTool" {
				result, err := tool.Execute(map[string]any{"command": patch})
				if err != nil {
					return fmt.Sprintf("apply failed: %v", err)
				}
				return result
			}
		}
		return "patch generated (no KubeTool registered for automatic apply)"

	case "restart":
		return "manual restart required: " + patch

	case "scale":
		return "manual scaling required: " + patch

	default:
		return "manual action required: " + patch
	}
}
