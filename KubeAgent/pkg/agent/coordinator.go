package agent

import (
	"encoding/json"
	"fmt"
	"sync"
	"time"

	"github.com/google/uuid"
)

// BaseCoordinator implements the CoordinatorAgent interface
type BaseCoordinator struct {
	config         *AgentConfig
	agents         map[AgentType]Agent
	agentsMutex    sync.RWMutex
	llmClient      LLMClient
	stateStore     StateStore
	toolRegistry   ToolRegistry
	logger         Logger
	metrics        *AgentMetrics
}

// NewCoordinator creates a new coordinator agent
func NewCoordinator(config *AgentConfig, llmClient LLMClient, stateStore StateStore, logger Logger) *BaseCoordinator {
	if config == nil {
		config = &AgentConfig{
			Name:        "coordinator",
			Type:        AgentTypeCoordinator,
			Description: "Orchestrates multiple specialist agents",
			MaxRetries:  3,
			Timeout:     5 * time.Minute,
		}
	}

	return &BaseCoordinator{
		config:      config,
		agents:      make(map[AgentType]Agent),
		llmClient:   llmClient,
		stateStore:  stateStore,
		logger:      logger,
		metrics: &AgentMetrics{
			AgentType: AgentTypeCoordinator,
		},
	}
}

// Name returns the coordinator's name
func (c *BaseCoordinator) Name() string {
	return c.config.Name
}

// Type returns the coordinator's type
func (c *BaseCoordinator) Type() AgentType {
	return c.config.Type
}

// Config returns the coordinator's configuration
func (c *BaseCoordinator) Config() *AgentConfig {
	return c.config
}

// CanHandle checks if the coordinator can handle a task type
func (c *BaseCoordinator) CanHandle(taskType TaskType) bool {
	// Coordinator can handle all task types by delegating to specialists
	return true
}

// Execute executes a single task (usually delegates to specialist)
func (c *BaseCoordinator) Execute(ctx *AgentContext, task *Task) (*Task, error) {
	c.logger.Info("Coordinator executing task", map[string]interface{}{
		"task_id":   task.ID,
		"task_type": task.Type,
	})

	// Update task status
	task.Status = TaskStatusRunning
	now := time.Now()
	task.StartedAt = &now

	// Save task state
	if c.stateStore != nil {
		if err := c.stateStore.SaveTask(ctx.Context(), task); err != nil {
			c.logger.Warn("Failed to save task state", map[string]interface{}{
				"task_id": task.ID,
				"error":   err.Error(),
			})
		}
	}

	// Find appropriate agent for the task
	agent, err := c.selectAgentForTask(task)
	if err != nil {
		task.Status = TaskStatusFailed
		task.Error = err.Error()
		completedAt := time.Now()
		task.CompletedAt = &completedAt
		return task, err
	}

	// Execute task with selected agent
	result, err := agent.Execute(ctx, task)
	if err != nil {
		c.logger.Error("Agent execution failed", map[string]interface{}{
			"task_id":    task.ID,
			"agent_type": agent.Type(),
			"error":      err.Error(),
		})
		task.Status = TaskStatusFailed
		task.Error = err.Error()
	} else {
		c.logger.Info("Agent execution completed", map[string]interface{}{
			"task_id":    task.ID,
			"agent_type": agent.Type(),
		})
		task.Status = TaskStatusCompleted
	}

	completedAt := time.Now()
	task.CompletedAt = &completedAt

	// Save final task state
	if c.stateStore != nil {
		if err := c.stateStore.SaveTask(ctx.Context(), task); err != nil {
			c.logger.Warn("Failed to save final task state", map[string]interface{}{
				"task_id": task.ID,
				"error":   err.Error(),
			})
		}
	}

	return result, err
}

// RegisterAgent registers a specialist agent
func (c *BaseCoordinator) RegisterAgent(agent Agent) error {
	if agent == nil {
		return fmt.Errorf("cannot register nil agent")
	}

	c.agentsMutex.Lock()
	defer c.agentsMutex.Unlock()

	agentType := agent.Type()
	if agentType == AgentTypeCoordinator {
		return fmt.Errorf("cannot register coordinator as specialist agent")
	}

	if _, exists := c.agents[agentType]; exists {
		return fmt.Errorf("agent of type %s already registered", agentType)
	}

	c.agents[agentType] = agent
	c.logger.Info("Registered agent", map[string]interface{}{
		"agent_type": agentType,
		"agent_name": agent.Name(),
	})

	return nil
}

// GetAgent retrieves a registered agent by type
func (c *BaseCoordinator) GetAgent(agentType AgentType) (Agent, error) {
	c.agentsMutex.RLock()
	defer c.agentsMutex.RUnlock()

	agent, exists := c.agents[agentType]
	if !exists {
		return nil, fmt.Errorf("agent of type %s not registered", agentType)
	}

	return agent, nil
}

// Plan creates an execution plan from a request
func (c *BaseCoordinator) Plan(ctx *AgentContext, request *Request) (*ExecutionPlan, error) {
	c.logger.Info("Creating execution plan", map[string]interface{}{
		"request_id": request.ID,
		"user":       request.User,
		"input":      request.Input,
	})

	// Parse user intent using LLM
	intent, err := c.parseIntent(ctx, request)
	if err != nil {
		return nil, fmt.Errorf("failed to parse intent: %w", err)
	}

	// Decompose into tasks
	tasks, err := c.decomposeTasks(ctx, request, intent)
	if err != nil {
		return nil, fmt.Errorf("failed to decompose tasks: %w", err)
	}

	// Determine execution mode
	executionMode := c.determineExecutionMode(tasks)

	// Create execution plan
	plan := &ExecutionPlan{
		ID:            uuid.New().String(),
		RequestID:     request.ID,
		Tasks:         tasks,
		ExecutionMode: executionMode,
		Status:        TaskStatusPending,
		Metadata: map[string]interface{}{
			"intent": intent,
		},
		CreatedAt: time.Now(),
		UpdatedAt: time.Now(),
	}

	// Save plan
	if c.stateStore != nil {
		if err := c.stateStore.SavePlan(ctx.Context(), plan); err != nil {
			c.logger.Warn("Failed to save execution plan", map[string]interface{}{
				"plan_id": plan.ID,
				"error":   err.Error(),
			})
		}
	}

	c.logger.Info("Execution plan created", map[string]interface{}{
		"plan_id":        plan.ID,
		"task_count":     len(tasks),
		"execution_mode": executionMode,
	})

	return plan, nil
}

// ExecutePlan executes an execution plan
func (c *BaseCoordinator) ExecutePlan(ctx *AgentContext, plan *ExecutionPlan) (*Response, error) {
	startTime := time.Now()

	c.logger.Info("Executing plan", map[string]interface{}{
		"plan_id":        plan.ID,
		"execution_mode": plan.ExecutionMode,
		"task_count":     len(plan.Tasks),
	})

	plan.Status = TaskStatusRunning
	ctx.ExecutionPlan = plan

	var result *Response
	var err error

	switch plan.ExecutionMode {
	case ExecutionModeSequential:
		result, err = c.executeSequential(ctx, plan)
	case ExecutionModeParallel:
		result, err = c.executeParallel(ctx, plan)
	case ExecutionModeConditional:
		result, err = c.executeConditional(ctx, plan)
	default:
		err = fmt.Errorf("unsupported execution mode: %s", plan.ExecutionMode)
	}

	if err != nil {
		plan.Status = TaskStatusFailed
		c.logger.Error("Plan execution failed", map[string]interface{}{
			"plan_id": plan.ID,
			"error":   err.Error(),
		})
	} else {
		plan.Status = TaskStatusCompleted
		c.logger.Info("Plan execution completed", map[string]interface{}{
			"plan_id": plan.ID,
			"duration": time.Since(startTime).String(),
		})
	}

	plan.UpdatedAt = time.Now()

	// Update metrics
	c.updateMetrics(time.Since(startTime), err == nil)

	return result, err
}

// parseIntent uses LLM to parse user intent
func (c *BaseCoordinator) parseIntent(ctx *AgentContext, request *Request) (string, error) {
	if request.Intent != "" {
		return request.Intent, nil
	}

	prompt := fmt.Sprintf(`Analyze the following user request and identify the primary intent.

User Request: %s

Classify the intent into one of these categories:
- diagnose: User wants to diagnose a problem
- remediate: User wants to fix a problem
- audit: User wants to check security or compliance
- optimize: User wants to optimize resources or costs
- query: User wants to get information

Respond with only the intent category (one word).`, request.Input)

	messages := []Message{
		{Role: "system", Content: "You are a Kubernetes operations assistant."},
		{Role: "user", Content: prompt},
	}

	response, err := c.llmClient.Complete(ctx.Context(), messages)
	if err != nil {
		return "", fmt.Errorf("LLM call failed: %w", err)
	}

	return response, nil
}

// decomposeTasks breaks down the request into individual tasks
func (c *BaseCoordinator) decomposeTasks(ctx *AgentContext, request *Request, intent string) ([]*Task, error) {
	prompt := fmt.Sprintf(`Break down the following user request into specific tasks.

User Request: %s
Intent: %s

Available Agent Types:
- diagnostician: Diagnose pod failures, analyze logs, events, metrics
- remediator: Generate fixes, create patches, remediate issues
- security: Audit RBAC, scan images, check compliance
- cost_optimizer: Analyze resource usage, recommend optimizations
- knowledge: Search documentation, find best practices

Return a JSON array of tasks with this structure:
[
  {
    "type": "diagnose|remediate|audit|optimize|query",
    "description": "Clear description of the task",
    "assigned_agent": "agent_type",
    "input": {
      "key": "value"
    }
  }
]

Respond with only the JSON array.`, request.Input, intent)

	messages := []Message{
		{Role: "system", Content: "You are a task decomposition expert for Kubernetes operations."},
		{Role: "user", Content: prompt},
	}

	response, err := c.llmClient.Complete(ctx.Context(), messages)
	if err != nil {
		return nil, fmt.Errorf("LLM call failed: %w", err)
	}

	// Parse JSON response
	var tasksData []struct {
		Type          string                 `json:"type"`
		Description   string                 `json:"description"`
		AssignedAgent string                 `json:"assigned_agent"`
		Input         map[string]interface{} `json:"input"`
	}

	if err := json.Unmarshal([]byte(response), &tasksData); err != nil {
		// If JSON parsing fails, create a simple single task
		c.logger.Warn("Failed to parse LLM response as JSON, creating simple task", map[string]interface{}{
			"error": err.Error(),
		})

		return []*Task{
			{
				ID:          uuid.New().String(),
				Type:        TaskType(intent),
				Description: request.Input,
				Status:      TaskStatusPending,
				Input:       request.Context,
				CreatedAt:   time.Now(),
			},
		}, nil
	}

	// Convert to Task objects
	tasks := make([]*Task, 0, len(tasksData))
	for _, td := range tasksData {
		task := &Task{
			ID:            uuid.New().String(),
			Type:          TaskType(td.Type),
			Description:   td.Description,
			Status:        TaskStatusPending,
			AssignedAgent: AgentType(td.AssignedAgent),
			Input:         td.Input,
			CreatedAt:     time.Now(),
		}
		tasks = append(tasks, task)
	}

	return tasks, nil
}

// determineExecutionMode determines how tasks should be executed
func (c *BaseCoordinator) determineExecutionMode(tasks []*Task) ExecutionMode {
	// Simple heuristic: if tasks have dependencies, use sequential
	// Otherwise use parallel if there are multiple tasks
	hasDependencies := false
	for _, task := range tasks {
		if len(task.Dependencies) > 0 {
			hasDependencies = true
			break
		}
	}

	if hasDependencies {
		return ExecutionModeSequential
	}

	if len(tasks) > 1 {
		return ExecutionModeParallel
	}

	return ExecutionModeSequential
}

// selectAgentForTask selects the appropriate agent for a task
func (c *BaseCoordinator) selectAgentForTask(task *Task) (Agent, error) {
	// If agent is already assigned, use it
	if task.AssignedAgent != "" {
		agent, err := c.GetAgent(task.AssignedAgent)
		if err == nil {
			return agent, nil
		}
		// Fall through to automatic selection if assigned agent not found
	}

	// Select agent based on task type
	c.agentsMutex.RLock()
	defer c.agentsMutex.RUnlock()

	for _, agent := range c.agents {
		if agent.CanHandle(task.Type) {
			return agent, nil
		}
	}

	return nil, fmt.Errorf("no agent available to handle task type: %s", task.Type)
}

// executeSequential executes tasks sequentially
func (c *BaseCoordinator) executeSequential(ctx *AgentContext, plan *ExecutionPlan) (*Response, error) {
	executedAgents := make([]AgentType, 0)
	errors := make([]string, 0)
	results := make(map[string]interface{})

	for _, task := range plan.Tasks {
		result, err := c.Execute(ctx, task)
		if err != nil {
			errors = append(errors, fmt.Sprintf("Task %s failed: %s", task.ID, err.Error()))
			// Continue with other tasks even if one fails
		} else {
			executedAgents = append(executedAgents, task.AssignedAgent)
			if result.Output != nil {
				results[task.ID] = result.Output
			}
		}
	}

	// Generate final response
	finalResult := c.generateFinalResponse(ctx, plan, results)

	response := &Response{
		RequestID:   plan.RequestID,
		Status:      plan.Status,
		Result:      finalResult,
		Data:        results,
		Errors:      errors,
		ExecutedBy:  executedAgents,
		CompletedAt: time.Now(),
	}

	return response, nil
}

// executeParallel executes tasks in parallel
func (c *BaseCoordinator) executeParallel(ctx *AgentContext, plan *ExecutionPlan) (*Response, error) {
	var wg sync.WaitGroup
	var mu sync.Mutex

	executedAgents := make([]AgentType, 0)
	errors := make([]string, 0)
	results := make(map[string]interface{})

	for _, task := range plan.Tasks {
		wg.Add(1)
		go func(t *Task) {
			defer wg.Done()

			result, err := c.Execute(ctx, t)

			mu.Lock()
			defer mu.Unlock()

			if err != nil {
				errors = append(errors, fmt.Sprintf("Task %s failed: %s", t.ID, err.Error()))
			} else {
				executedAgents = append(executedAgents, t.AssignedAgent)
				if result.Output != nil {
					results[t.ID] = result.Output
				}
			}
		}(task)
	}

	wg.Wait()

	// Generate final response
	finalResult := c.generateFinalResponse(ctx, plan, results)

	response := &Response{
		RequestID:   plan.RequestID,
		Status:      plan.Status,
		Result:      finalResult,
		Data:        results,
		Errors:      errors,
		ExecutedBy:  executedAgents,
		CompletedAt: time.Now(),
	}

	return response, nil
}

// executeConditional executes tasks with conditional logic
func (c *BaseCoordinator) executeConditional(ctx *AgentContext, plan *ExecutionPlan) (*Response, error) {
	// For now, just use sequential execution
	// TODO: Implement conditional logic based on task results
	return c.executeSequential(ctx, plan)
}

// generateFinalResponse generates the final response using LLM
func (c *BaseCoordinator) generateFinalResponse(ctx *AgentContext, plan *ExecutionPlan, results map[string]interface{}) string {
	// Format results for LLM
	resultsJSON, _ := json.MarshalIndent(results, "", "  ")

	prompt := fmt.Sprintf(`Summarize the following task execution results for the user.

Original Request: %s

Task Results:
%s

Provide a clear, concise summary of what was done and any important findings.`,
		plan.RequestID, string(resultsJSON))

	messages := []Message{
		{Role: "system", Content: "You are a helpful Kubernetes assistant. Summarize task results clearly."},
		{Role: "user", Content: prompt},
	}

	response, err := c.llmClient.Complete(ctx.Context(), messages)
	if err != nil {
		c.logger.Warn("Failed to generate final response with LLM", map[string]interface{}{
			"error": err.Error(),
		})
		return fmt.Sprintf("Executed %d tasks. Check detailed results in the data field.", len(plan.Tasks))
	}

	return response
}

// updateMetrics updates agent execution metrics
func (c *BaseCoordinator) updateMetrics(duration time.Duration, success bool) {
	if success {
		c.metrics.TasksCompleted++
	} else {
		c.metrics.TasksFailed++
	}

	c.metrics.TotalDuration += duration
	totalTasks := c.metrics.TasksCompleted + c.metrics.TasksFailed
	if totalTasks > 0 {
		c.metrics.AverageDuration = c.metrics.TotalDuration / time.Duration(totalTasks)
	}
	c.metrics.LastExecutedAt = time.Now()
}

// GetMetrics returns the coordinator's metrics
func (c *BaseCoordinator) GetMetrics() *AgentMetrics {
	return c.metrics
}
