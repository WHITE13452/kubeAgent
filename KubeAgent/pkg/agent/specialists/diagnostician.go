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

// Execute executes a diagnostic task
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

	// Collect K8s data using registered tools before sending to LLM
	logs := d.collectToolOutput("LogTool", map[string]any{
		"podName":   podName,
		"namespace": namespace,
	})
	events := d.collectToolOutput("EventTool", map[string]any{
		"podName":   podName,
		"namespace": namespace,
	})

	diagnosis, err := d.diagnose(ctx, podName, namespace, task.Description, logs, events)
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

// Analyze implements SpecialistAgent; collects tool data then calls LLM
func (d *DiagnosticianAgent) Analyze(ctx *agent.AgentContext, input map[string]any) (map[string]any, error) {
	podName, _ := input["pod_name"].(string)
	namespace, _ := input["namespace"].(string)
	description, _ := input["description"].(string)

	logs := d.collectToolOutput("LogTool", map[string]any{
		"podName":   podName,
		"namespace": namespace,
	})
	events := d.collectToolOutput("EventTool", map[string]any{
		"podName":   podName,
		"namespace": namespace,
	})

	return d.diagnose(ctx, podName, namespace, description, logs, events)
}

// collectToolOutput calls a named tool and returns its output, or an error message string if unavailable
func (d *DiagnosticianAgent) collectToolOutput(toolName string, params map[string]any) string {
	for _, tool := range d.GetTools() {
		if tool.Name() == toolName {
			result, err := tool.Execute(params)
			if err != nil {
				return fmt.Sprintf("[%s error: %v]", toolName, err)
			}
			return result
		}
	}
	return ""
}

// diagnose calls LLM with pod metadata and collected tool data to produce a structured diagnosis
func (d *DiagnosticianAgent) diagnose(ctx *agent.AgentContext, podName, namespace, description, logs, events string) (map[string]any, error) {
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

Pod Logs:
%s

Pod Events:
%s

Provide a comprehensive diagnosis in JSON format.`, podName, namespace, description, logs, events)

	response, err := d.CallLLM(ctx, systemPrompt, userPrompt)
	if err != nil {
		return nil, fmt.Errorf("diagnosis failed: %w", err)
	}

	var diagnosis map[string]any
	if err := json.Unmarshal([]byte(response), &diagnosis); err != nil {
		return map[string]any{
			"root_cause":      response,
			"error_type":      "Unknown",
			"recommendations": []string{"Check pod logs and events for more details"},
			"confidence":      0.5,
		}, nil
	}

	return diagnosis, nil
}
