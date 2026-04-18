package agent

import (
	"encoding/json"
	"fmt"
	"log"
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

// CallLLMWithTools is a helper method for agents to call LLM with tools (single call, no loop)
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

// DefaultMaxToolIterations is the default max iterations for the tool loop
const DefaultMaxToolIterations = 10

// RunToolLoop runs an agentic tool-use loop:
// 1. Sends the prompt + tool definitions to the LLM
// 2. If the LLM returns tool calls, executes them and feeds results back
// 3. Repeats until the LLM returns a final text response (no more tool calls)
func (b *BaseAgent) RunToolLoop(ctx *AgentContext, systemPrompt, userPrompt string, maxIterations int) (string, error) {
	if maxIterations <= 0 {
		maxIterations = DefaultMaxToolIterations
	}

	// No tools registered - fall back to simple completion
	if len(b.tools) == 0 {
		return b.CallLLM(ctx, systemPrompt, userPrompt)
	}

	// Log the initial prompts
	log.Printf("[AGENT:%s] >>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>", b.config.Name)
	log.Printf("[AGENT:%s] System Prompt (%d chars): %.200s...", b.config.Name, len(systemPrompt), systemPrompt)
	log.Printf("[AGENT:%s] User Prompt (%d chars): %.200s...", b.config.Name, len(userPrompt), userPrompt)
	log.Printf("[AGENT:%s] Available Tools: %v", b.config.Name, func() []string {
		names := make([]string, len(b.tools))
		for i, t := range b.tools {
			names[i] = t.Name()
		}
		return names
	}())
	log.Printf("[AGENT:%s] ==================================================", b.config.Name)

	messages := []Message{
		{Role: "system", Content: systemPrompt},
		{Role: "user", Content: userPrompt},
	}

	for i := 0; i < maxIterations; i++ {
		log.Printf("[AGENT:%s] ---- Tool Loop Iteration %d ----", b.config.Name, i+1)
		
		resp, err := b.llmClient.CompleteWithTools(ctx.Context(), messages, b.tools)
		if err != nil {
			log.Printf("[AGENT:%s] LLM Error: %v", b.config.Name, err)
			log.Printf("[AGENT:%s] <<<<<<<<<<<<<<<<<<<<<<<<<<<<<<<<<<<<<<<<<<<<<<", b.config.Name)
			return "", fmt.Errorf("LLM call failed: %w", err)
		}

		// No tool calls - return the final text response
		if len(resp.ToolCalls) == 0 {
			log.Printf("[AGENT:%s] No more tool calls, final response:", b.config.Name)
			content := resp.Content
			if len(content) > 1000 {
				content = content[:1000] + "..."
			}
			log.Printf("[AGENT:%s] Final Response: %s", b.config.Name, content)
			log.Printf("[AGENT:%s] <<<<<<<<<<<<<<<<<<<<<<<<<<<<<<<<<<<<<<<<<<<<<<", b.config.Name)
			return resp.Content, nil
		}

		// Add assistant message with tool calls to conversation history
		messages = append(messages, Message{
			Role:      "assistant",
			Content:   resp.Content,
			ToolCalls: resp.ToolCalls,
		})

		// Execute each tool and add results to conversation
		for _, tc := range resp.ToolCalls {
			argsJSON, _ := json.Marshal(tc.Arguments)
			log.Printf("[TOOL:%s] >>> Executing Tool >>>", tc.Name)
			log.Printf("[TOOL:%s] ToolCall ID: %s", tc.Name, tc.ID)
			log.Printf("[TOOL:%s] Arguments: %s", tc.Name, argsJSON)
			
			result, toolErr := b.executeTool(tc.Name, tc.Arguments)
			
			toolMsg := Message{
				Role:       "tool",
				Content:    result,
				ToolCallID: tc.ID,
			}
			if toolErr != nil {
				toolMsg.Content = fmt.Sprintf("Error: %v", toolErr)
				toolMsg.IsError = true
				log.Printf("[TOOL:%s] ERROR: %v", tc.Name, toolErr)
			} else {
				// Log result preview
				resultPreview := result
				if len(resultPreview) > 500 {
					resultPreview = resultPreview[:500] + "..."
				}
				log.Printf("[TOOL:%s] Result (%d chars): %s", tc.Name, len(result), resultPreview)
			}
			log.Printf("[TOOL:%s] <<< Tool Complete <<<", tc.Name)
			
			messages = append(messages, toolMsg)
		}

		b.logger.Info("Tool loop iteration completed", map[string]interface{}{
			"agent_type": b.config.Type,
			"iteration":  i + 1,
			"tool_calls": len(resp.ToolCalls),
		})
	}

	log.Printf("[AGENT:%s] Max iterations reached", b.config.Name)
	log.Printf("[AGENT:%s] <<<<<<<<<<<<<<<<<<<<<<<<<<<<<<<<<<<<<<<<<<<<<<", b.config.Name)
	return "", fmt.Errorf("tool loop: max iterations (%d) reached", maxIterations)
}

// executeTool finds and executes a tool by name
func (b *BaseAgent) executeTool(name string, args map[string]interface{}) (string, error) {
	for _, tool := range b.tools {
		if tool.Name() == name {
			return tool.Execute(args)
		}
	}
	return "", fmt.Errorf("tool %s not found", name)
}
