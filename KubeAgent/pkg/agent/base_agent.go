package agent

import (
	"fmt"
	"time"
)

// BaseAgent provides common functionality for all agents
type BaseAgent struct {
	config       *AgentConfig
	tools        []Tool
	llmClient    LLMClient
	logger       Logger
	metrics      *AgentMetrics
}

// NewBaseAgent creates a new base agent
func NewBaseAgent(config *AgentConfig, llmClient LLMClient, logger Logger) *BaseAgent {
	return &BaseAgent{
		config:    config,
		tools:     make([]Tool, 0),
		llmClient: llmClient,
		logger:    logger,
		metrics: &AgentMetrics{
			AgentType: config.Type,
		},
	}
}

// Name returns the agent's name
func (b *BaseAgent) Name() string {
	return b.config.Name
}

// Type returns the agent's type
func (b *BaseAgent) Type() AgentType {
	return b.config.Type
}

// Config returns the agent's configuration
func (b *BaseAgent) Config() *AgentConfig {
	return b.config
}

// AddTool adds a tool to the agent's toolset
func (b *BaseAgent) AddTool(tool Tool) {
	b.tools = append(b.tools, tool)
	b.logger.Info("Added tool to agent", map[string]interface{}{
		"agent_type": b.config.Type,
		"tool_name":  tool.Name(),
	})
}

// GetTools returns all tools available to this agent
func (b *BaseAgent) GetTools() []Tool {
	return b.tools
}

// Execute executes a task (to be overridden by specific agents)
func (b *BaseAgent) Execute(ctx *AgentContext, task *Task) (*Task, error) {
	startTime := time.Now()
	defer func() {
		duration := time.Since(startTime)
		b.updateMetrics(duration, task.Status == TaskStatusCompleted)
	}()

	b.logger.Info("BaseAgent executing task", map[string]interface{}{
		"agent_type": b.config.Type,
		"task_id":    task.ID,
		"task_type":  task.Type,
	})

	// This is a placeholder - specific agents should override this
	task.Status = TaskStatusCompleted
	task.Output = map[string]interface{}{
		"message": fmt.Sprintf("Task executed by %s agent", b.config.Type),
	}

	return task, nil
}

// CanHandle checks if the agent can handle a task type (to be overridden)
func (b *BaseAgent) CanHandle(taskType TaskType) bool {
	// Default implementation - should be overridden by specific agents
	return false
}

// GetMetrics returns the agent's metrics
func (b *BaseAgent) GetMetrics() *AgentMetrics {
	return b.metrics
}

// updateMetrics updates agent execution metrics
func (b *BaseAgent) updateMetrics(duration time.Duration, success bool) {
	if success {
		b.metrics.TasksCompleted++
	} else {
		b.metrics.TasksFailed++
	}

	b.metrics.TotalDuration += duration
	totalTasks := b.metrics.TasksCompleted + b.metrics.TasksFailed
	if totalTasks > 0 {
		b.metrics.AverageDuration = b.metrics.TotalDuration / time.Duration(totalTasks)
	}
	b.metrics.LastExecutedAt = time.Now()
}

// CallLLM is a helper method for agents to call LLM
func (b *BaseAgent) CallLLM(ctx *AgentContext, systemPrompt, userPrompt string) (string, error) {
	messages := []Message{
		{Role: "system", Content: systemPrompt},
		{Role: "user", Content: userPrompt},
	}

	response, err := b.llmClient.Complete(ctx.Context(), messages)
	if err != nil {
		b.logger.Error("LLM call failed", map[string]interface{}{
			"agent_type": b.config.Type,
			"error":      err.Error(),
		})
		return "", err
	}

	return response, nil
}

// CallLLMWithTools is a helper method for agents to call LLM with tools
func (b *BaseAgent) CallLLMWithTools(ctx *AgentContext, systemPrompt, userPrompt string) (*LLMResponse, error) {
	messages := []Message{
		{Role: "system", Content: systemPrompt},
		{Role: "user", Content: userPrompt},
	}

	response, err := b.llmClient.CompleteWithTools(ctx.Context(), messages, b.tools)
	if err != nil {
		b.logger.Error("LLM call with tools failed", map[string]interface{}{
			"agent_type": b.config.Type,
			"error":      err.Error(),
		})
		return nil, err
	}

	return response, nil
}
