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

// Execute executes a remediation task using the agentic tool-use loop
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

	result, err := r.remediate(ctx, rootCause, errorType, diagnosis)
	if err != nil {
		task.Status = agent.TaskStatusFailed
		task.Error = err.Error()
		completedAt := time.Now()
		task.CompletedAt = &completedAt
		return task, err
	}

	task.Status = agent.TaskStatusCompleted
	task.Output = result
	task.Output["remediation_time"] = time.Since(startTime).String()

	completedAt := time.Now()
	task.CompletedAt = &completedAt

	return task, nil
}

// Analyze implements SpecialistAgent
func (r *RemediatorAgent) Analyze(ctx *agent.AgentContext, input map[string]any) (map[string]any, error) {
	rootCause, _ := input["root_cause"].(string)
	errorType, _ := input["error_type"].(string)
	diagnosis, _ := input["diagnosis"].(map[string]any)

	return r.remediate(ctx, rootCause, errorType, diagnosis)
}

// remediate runs the agentic tool-use loop to generate and apply fixes
func (r *RemediatorAgent) remediate(ctx *agent.AgentContext, rootCause, errorType string, diagnosis map[string]any) (map[string]any, error) {
	systemPrompt := `You are a Kubernetes remediation expert. You have tools to create/delete resources, execute kubectl commands, and ask for human approval.

Your workflow:
1. Analyze the diagnosis and generate a remediation plan
2. For dangerous operations (delete, modify production resources), use the HumanTool to ask for confirmation before proceeding
3. Apply the fix using available tools (CreateTool, DeleteTool, KubeTool)
4. Report what actions were taken

Return your final result in JSON format:
{
  "remediation_type": "patch|config_change|restart|scale",
  "actions_taken": ["action1", "action2"],
  "verification_steps": ["Step 1", "Step 2"],
  "risk_level": "low|medium|high"
}`

	diagnosisJSON, _ := json.Marshal(diagnosis)
	userPrompt := fmt.Sprintf(`Remediate the following Kubernetes issue:

Root Cause: %s
Error Type: %s
Diagnosis Details: %s

Use the available tools to fix the issue. Ask for human confirmation before applying dangerous changes.
Return a summary in JSON format when done.`, rootCause, errorType, string(diagnosisJSON))

	response, err := r.RunToolLoop(ctx, systemPrompt, userPrompt, 0)
	if err != nil {
		return nil, fmt.Errorf("remediation failed: %w", err)
	}

	var result map[string]any
	if err := json.Unmarshal([]byte(response), &result); err != nil {
		// LLM returned non-JSON text - wrap it
		return map[string]any{
			"remediation_type":   "manual",
			"actions_taken":     []string{response},
			"verification_steps": []string{"Verify the fix manually"},
			"risk_level":         "medium",
		}, nil
	}

	return result, nil
}
