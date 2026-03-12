package specialists

import (
	"encoding/json"
	"fmt"
	"time"

	"kubeagent/pkg/agent"
)

// DiagnosticianAgent specializes in diagnosing pod failures and issues
type DiagnosticianAgent struct {
	*agent.BaseAgent
}

// NewDiagnosticianAgent creates a new diagnostician agent
func NewDiagnosticianAgent(llmClient agent.LLMClient, logger agent.Logger) *DiagnosticianAgent {
	config := &agent.AgentConfig{
		Name:        "diagnostician",
		Type:        agent.AgentTypeDiagnostician,
		Description: "Diagnoses pod failures, analyzes logs, events, and metrics",
		MaxRetries:  3,
		Timeout:     2 * time.Minute,
	}

	return &DiagnosticianAgent{
		BaseAgent: agent.NewBaseAgent(config, llmClient, logger),
	}
}

// CanHandle checks if the diagnostician can handle a task type
func (d *DiagnosticianAgent) CanHandle(taskType agent.TaskType) bool {
	return taskType == agent.TaskTypeDiagnose || taskType == agent.TaskTypeQuery
}

// Execute executes a diagnostic task using the agentic tool-use loop
func (d *DiagnosticianAgent) Execute(ctx *agent.AgentContext, task *agent.Task) (*agent.Task, error) {
	startTime := time.Now()

	podName, _ := task.Input["pod_name"].(string)
	if podName == "" {
		podName, _ = task.Input["podName"].(string)
	}
	namespace, _ := task.Input["namespace"].(string)
	if namespace == "" {
		namespace = "default"
	}

	task.Status = agent.TaskStatusRunning
	now := time.Now()
	task.StartedAt = &now

	diagnosis, err := d.diagnose(ctx, podName, namespace, task.Description)
	if err != nil {
		task.Status = agent.TaskStatusFailed
		task.Error = err.Error()
		completedAt := time.Now()
		task.CompletedAt = &completedAt
		return task, err
	}

	task.Status = agent.TaskStatusCompleted
	task.Output = map[string]any{
		"pod_name":        podName,
		"namespace":       namespace,
		"root_cause":      diagnosis["root_cause"],
		"error_type":      diagnosis["error_type"],
		"recommendations": diagnosis["recommendations"],
		"confidence":      diagnosis["confidence"],
		"diagnosis_time":  time.Since(startTime).String(),
	}

	completedAt := time.Now()
	task.CompletedAt = &completedAt

	return task, nil
}

// Analyze implements SpecialistAgent
func (d *DiagnosticianAgent) Analyze(ctx *agent.AgentContext, input map[string]any) (map[string]any, error) {
	podName, _ := input["pod_name"].(string)
	namespace, _ := input["namespace"].(string)
	description, _ := input["description"].(string)

	return d.diagnose(ctx, podName, namespace, description)
}

// diagnose runs the agentic tool-use loop to collect data and produce a diagnosis
func (d *DiagnosticianAgent) diagnose(ctx *agent.AgentContext, podName, namespace, description string) (map[string]any, error) {
	systemPrompt := `You are a Kubernetes diagnostics expert. You have tools to inspect pods, logs, events, and cluster state.

Use the available tools to collect information about the issue, then provide a diagnosis.

Return your final diagnosis in JSON format:
{
  "root_cause": "Detailed explanation of the root cause",
  "error_type": "Error classification (OOMKilled, CrashLoopBackOff, ImagePullBackOff, etc.)",
  "key_errors": ["Error 1", "Error 2"],
  "recommendations": ["Recommendation 1", "Recommendation 2"],
  "confidence": 0.95
}`

	userPrompt := fmt.Sprintf(`Diagnose the following Kubernetes pod issue:

Pod Name: %s
Namespace: %s
Issue Description: %s

Use the available tools to collect logs, events, and other relevant information, then provide a comprehensive diagnosis in JSON format.`, podName, namespace, description)

	response, err := d.RunToolLoop(ctx, systemPrompt, userPrompt, 0)
	if err != nil {
		return nil, fmt.Errorf("diagnosis failed: %w", err)
	}

	var diagnosis map[string]any
	if err := json.Unmarshal([]byte(response), &diagnosis); err != nil {
		// LLM returned non-JSON text - wrap it as a diagnosis
		return map[string]any{
			"root_cause":      response,
			"error_type":      "Unknown",
			"recommendations": []string{"Check pod logs and events for more details"},
			"confidence":      0.5,
		}, nil
	}

	return diagnosis, nil
}
