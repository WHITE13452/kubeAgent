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

	baseAgent := agent.NewBaseAgent(config, llmClient, logger)

	return &RemediatorAgent{
		BaseAgent: baseAgent,
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

	// Extract diagnosis info
	diagnosis, _ := task.Input["diagnosis"].(map[string]interface{})
	rootCause, _ := task.Input["root_cause"].(string)
	errorType, _ := task.Input["error_type"].(string)

	if diagnosis == nil && rootCause == "" {
		// Try to extract from description
		rootCause = task.Description
	}

	// Generate remediation plan
	remediation, err := r.generateRemediation(ctx, rootCause, errorType, diagnosis)
	if err != nil {
		task.Status = agent.TaskStatusFailed
		task.Error = err.Error()
		completedAt := time.Now()
		task.CompletedAt = &completedAt
		return task, err
	}

	// Update task with results
	task.Status = agent.TaskStatusCompleted
	task.Output = map[string]interface{}{
		"remediation_type":  remediation["remediation_type"],
		"patch":             remediation["patch"],
		"verification_steps": remediation["verification_steps"],
		"requires_approval": remediation["requires_approval"],
		"risk_level":        remediation["risk_level"],
		"remediation_time":  time.Since(startTime).String(),
	}

	completedAt := time.Now()
	task.CompletedAt = &completedAt

	return task, nil
}

// Analyze analyzes input and returns remediation insights
func (r *RemediatorAgent) Analyze(ctx *agent.AgentContext, input map[string]interface{}) (map[string]interface{}, error) {
	rootCause, _ := input["root_cause"].(string)
	errorType, _ := input["error_type"].(string)
	diagnosis, _ := input["diagnosis"].(map[string]interface{})

	return r.generateRemediation(ctx, rootCause, errorType, diagnosis)
}

// generateRemediation generates a remediation plan
func (r *RemediatorAgent) generateRemediation(ctx *agent.AgentContext, rootCause, errorType string, diagnosis map[string]interface{}) (map[string]interface{}, error) {
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
  "requires_approval": true/false,
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

	// Parse JSON response
	var remediation map[string]interface{}
	if err := json.Unmarshal([]byte(response), &remediation); err != nil {
		// If JSON parsing fails, return a basic response
		return map[string]interface{}{
			"remediation_type":   "manual",
			"patch":              response,
			"verification_steps": []string{"Apply patch and verify pod status"},
			"requires_approval":  true,
			"risk_level":         "medium",
		}, nil
	}

	return remediation, nil
}
