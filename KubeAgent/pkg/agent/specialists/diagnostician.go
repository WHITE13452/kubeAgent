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

	baseAgent := agent.NewBaseAgent(config, llmClient, logger)

	return &DiagnosticianAgent{
		BaseAgent: baseAgent,
	}
}

// CanHandle checks if the diagnostician can handle a task type
func (d *DiagnosticianAgent) CanHandle(taskType agent.TaskType) bool {
	return taskType == agent.TaskTypeDiagnose || taskType == agent.TaskTypeQuery
}

// Execute executes a diagnostic task
func (d *DiagnosticianAgent) Execute(ctx *agent.AgentContext, task *agent.Task) (*agent.Task, error) {
	startTime := time.Now()

	d.BaseAgent.Config().Name = "diagnostician"
	logger := d.BaseAgent.Config()

	// Extract input parameters
	podName, _ := task.Input["pod_name"].(string)
	namespace, _ := task.Input["namespace"].(string)

	if podName == "" {
		// Try to extract from description
		podName, _ = task.Input["podName"].(string)
	}
	if namespace == "" {
		namespace = "default"
	}

	task.Status = agent.TaskStatusRunning
	now := time.Now()
	task.StartedAt = &now

	// Perform diagnosis
	diagnosis, err := d.diagnose(ctx, podName, namespace, task.Description)
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
		"pod_name":         podName,
		"namespace":        namespace,
		"root_cause":       diagnosis["root_cause"],
		"error_type":       diagnosis["error_type"],
		"recommendations":  diagnosis["recommendations"],
		"confidence":       diagnosis["confidence"],
		"diagnosis_time":   time.Since(startTime).String(),
	}

	completedAt := time.Now()
	task.CompletedAt = &completedAt

	_ = logger // Suppress unused variable warning

	return task, nil
}

// Analyze analyzes input and returns insights
func (d *DiagnosticianAgent) Analyze(ctx *agent.AgentContext, input map[string]interface{}) (map[string]interface{}, error) {
	podName, _ := input["pod_name"].(string)
	namespace, _ := input["namespace"].(string)
	description, _ := input["description"].(string)

	return d.diagnose(ctx, podName, namespace, description)
}

// diagnose performs the actual diagnosis logic
func (d *DiagnosticianAgent) diagnose(ctx *agent.AgentContext, podName, namespace, description string) (map[string]interface{}, error) {
	systemPrompt := `You are a Kubernetes diagnostics expert. Analyze pod issues and provide detailed diagnosis.

Your task is to:
1. Identify the root cause of the issue
2. Classify the error type (OOMKilled, CrashLoopBackOff, ImagePullBackOff, etc.)
3. Provide specific recommendations for fixing the issue
4. Estimate confidence level

Return your analysis in JSON format:
{
  "root_cause": "Detailed explanation of the root cause",
  "error_type": "Error classification",
  "key_errors": ["Error 1", "Error 2"],
  "recommendations": ["Recommendation 1", "Recommendation 2"],
  "confidence": 0.95
}`

	userPrompt := fmt.Sprintf(`Diagnose the following Kubernetes pod issue:

Pod Name: %s
Namespace: %s
Issue Description: %s

Provide a comprehensive diagnosis in JSON format.`, podName, namespace, description)

	response, err := d.CallLLM(ctx, systemPrompt, userPrompt)
	if err != nil {
		return nil, fmt.Errorf("diagnosis failed: %w", err)
	}

	// Parse JSON response
	var diagnosis map[string]interface{}
	if err := json.Unmarshal([]byte(response), &diagnosis); err != nil {
		// If JSON parsing fails, return a structured response anyway
		return map[string]interface{}{
			"root_cause":      response,
			"error_type":      "Unknown",
			"recommendations": []string{"Check pod logs and events for more details"},
			"confidence":      0.5,
		}, nil
	}

	return diagnosis, nil
}
