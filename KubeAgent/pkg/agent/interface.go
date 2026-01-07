package agent

import (
	"context"
)

// Agent is the core interface that all agents must implement
type Agent interface {
	// Name returns the agent's name
	Name() string

	// Type returns the agent's type
	Type() AgentType

	// Execute executes a task and returns the result
	Execute(ctx *AgentContext, task *Task) (*Task, error)

	// CanHandle checks if the agent can handle a specific task type
	CanHandle(taskType TaskType) bool

	// Config returns the agent's configuration
	Config() *AgentConfig
}

// CoordinatorAgent orchestrates multiple specialist agents
type CoordinatorAgent interface {
	Agent

	// Plan creates an execution plan from a request
	Plan(ctx *AgentContext, request *Request) (*ExecutionPlan, error)

	// ExecutePlan executes an execution plan
	ExecutePlan(ctx *AgentContext, plan *ExecutionPlan) (*Response, error)

	// RegisterAgent registers a specialist agent
	RegisterAgent(agent Agent) error

	// GetAgent retrieves a registered agent by type
	GetAgent(agentType AgentType) (Agent, error)
}

// SpecialistAgent is a specialized agent for specific tasks
type SpecialistAgent interface {
	Agent

	// Analyze analyzes input and returns insights
	Analyze(ctx *AgentContext, input map[string]interface{}) (map[string]interface{}, error)
}

// Tool represents a tool that agents can use
type Tool interface {
	// Name returns the tool's name
	Name() string

	// Description returns the tool's description
	Description() string

	// ArgsSchema returns the JSON schema for the tool's arguments
	ArgsSchema() string

	// Execute executes the tool with given parameters
	Execute(params map[string]interface{}) (string, error)
}

// ToolRegistry manages available tools
type ToolRegistry interface {
	// RegisterTool registers a new tool
	RegisterTool(tool Tool) error

	// GetTool retrieves a tool by name
	GetTool(name string) (Tool, bool)

	// ListTools returns all registered tools
	ListTools() []Tool

	// GetToolsForAgent returns tools available for a specific agent type
	GetToolsForAgent(agentType AgentType) []Tool
}

// StateStore manages agent execution state
type StateStore interface {
	// SaveContext saves the agent context
	SaveContext(ctx context.Context, agentCtx *AgentContext) error

	// LoadContext loads the agent context
	LoadContext(ctx context.Context, requestID string) (*AgentContext, error)

	// SaveTask saves a task
	SaveTask(ctx context.Context, task *Task) error

	// LoadTask loads a task
	LoadTask(ctx context.Context, taskID string) (*Task, error)

	// SavePlan saves an execution plan
	SavePlan(ctx context.Context, plan *ExecutionPlan) error

	// LoadPlan loads an execution plan
	LoadPlan(ctx context.Context, planID string) (*ExecutionPlan, error)

	// UpdateTaskStatus updates a task's status
	UpdateTaskStatus(ctx context.Context, taskID string, status TaskStatus) error
}

// LLMClient represents a client for interacting with LLM
type LLMClient interface {
	// Complete sends a prompt and returns the completion
	Complete(ctx context.Context, messages []Message) (string, error)

	// CompleteWithTools sends a prompt with available tools
	CompleteWithTools(ctx context.Context, messages []Message, tools []Tool) (*LLMResponse, error)
}

// Message represents a message in LLM conversation
type Message struct {
	Role    string `json:"role"`
	Content string `json:"content"`
}

// LLMResponse represents a response from LLM
type LLMResponse struct {
	Content    string     `json:"content"`
	ToolCalls  []ToolCall `json:"tool_calls,omitempty"`
	FinishReason string   `json:"finish_reason"`
}

// ToolCall represents a tool call from LLM
type ToolCall struct {
	ID       string                 `json:"id"`
	Name     string                 `json:"name"`
	Arguments map[string]interface{} `json:"arguments"`
}

// Logger provides logging capabilities
type Logger interface {
	Debug(msg string, fields map[string]interface{})
	Info(msg string, fields map[string]interface{})
	Warn(msg string, fields map[string]interface{})
	Error(msg string, fields map[string]interface{})
}
