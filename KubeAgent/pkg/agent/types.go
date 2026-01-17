package agent

import (
	"context"
	"time"
)

// AgentType defines the type of agent
type AgentType string

const (
	AgentTypeCoordinator   AgentType = "coordinator"
	AgentTypeDiagnostician AgentType = "diagnostician"
	AgentTypeRemediator    AgentType = "remediator"
	AgentTypeSecurity      AgentType = "security"
	AgentTypeCostOptimizer AgentType = "cost_optimizer"
	AgentTypeKnowledge     AgentType = "knowledge"
)

// TaskStatus represents the status of a task
type TaskStatus string

const (
	TaskStatusPending    TaskStatus = "pending"
	TaskStatusRunning    TaskStatus = "running"
	TaskStatusCompleted  TaskStatus = "completed"
	TaskStatusFailed     TaskStatus = "failed"
	TaskStatusCancelled  TaskStatus = "cancelled"
	TaskStatusSkipped    TaskStatus = "skipped"
)

// TaskType represents different types of tasks
type TaskType string

const (
	TaskTypeDiagnose  TaskType = "diagnose"
	TaskTypeRemediate TaskType = "remediate"
	TaskTypeAudit     TaskType = "audit"
	TaskTypeOptimize  TaskType = "optimize"
	TaskTypeQuery     TaskType = "query"
)

// ExecutionMode defines how subtasks should be executed
type ExecutionMode string

const (
	ExecutionModeSequential ExecutionMode = "sequential"
	ExecutionModeParallel   ExecutionMode = "parallel"
	ExecutionModeConditional ExecutionMode = "conditional"
)

// Request represents a user request to the agent system
type Request struct {
	ID          string                 `json:"id"`
	User        string                 `json:"user"`
	Input       string                 `json:"input"`
	Intent      string                 `json:"intent,omitempty"`
	Context     map[string]interface{} `json:"context,omitempty"`
	Metadata    map[string]string      `json:"metadata,omitempty"`
	CreatedAt   time.Time              `json:"created_at"`
}

// Response represents the final response from the agent system
type Response struct {
	RequestID   string                 `json:"request_id"`
	Status      TaskStatus             `json:"status"`
	Result      string                 `json:"result"`
	Data        map[string]interface{} `json:"data,omitempty"`
	Errors      []string               `json:"errors,omitempty"`
	ExecutedBy  []AgentType            `json:"executed_by"`
	Duration    time.Duration          `json:"duration"`
	CompletedAt time.Time              `json:"completed_at"`
}

// TaskCondition defines conditions for task execution
type TaskCondition struct {
	// OnSuccess specifies task IDs that must succeed for this task to execute
	OnSuccess []string `json:"on_success,omitempty"`
	// OnFailure specifies task IDs that must fail for this task to execute
	OnFailure []string `json:"on_failure,omitempty"`
}

// Task represents a unit of work to be executed by an agent
type Task struct {
	ID            string                 `json:"id"`
	Type          TaskType               `json:"type"`
	Description   string                 `json:"description"`
	Status        TaskStatus             `json:"status"`
	AssignedAgent AgentType              `json:"assigned_agent,omitempty"`
	Input         map[string]interface{} `json:"input"`
	Output        map[string]interface{} `json:"output,omitempty"`
	Error         string                 `json:"error,omitempty"`
	Dependencies  []string               `json:"dependencies,omitempty"`
	Condition     *TaskCondition         `json:"condition,omitempty"`
	CreatedAt     time.Time              `json:"created_at"`
	StartedAt     *time.Time             `json:"started_at,omitempty"`
	CompletedAt   *time.Time             `json:"completed_at,omitempty"`
}

// ExecutionPlan represents a plan for executing multiple tasks
type ExecutionPlan struct {
	ID            string                 `json:"id"`
	RequestID     string                 `json:"request_id"`
	Tasks         []*Task                `json:"tasks"`
	ExecutionMode ExecutionMode          `json:"execution_mode"`
	Status        TaskStatus             `json:"status"`
	Metadata      map[string]interface{} `json:"metadata,omitempty"`
	CreatedAt     time.Time              `json:"created_at"`
	UpdatedAt     time.Time              `json:"updated_at"`
}

// AgentContext contains shared context for agent execution
type AgentContext struct {
	ctx           context.Context
	RequestID     string                 `json:"request_id"`
	UserID        string                 `json:"user_id"`
	TraceID       string                 `json:"trace_id"`
	State         map[string]interface{} `json:"state"`
	ExecutionPlan *ExecutionPlan         `json:"execution_plan,omitempty"`
	CreatedAt     time.Time              `json:"created_at"`
}

// NewAgentContext creates a new agent context
func NewAgentContext(ctx context.Context, requestID, userID, traceID string) *AgentContext {
	return &AgentContext{
		ctx:       ctx,
		RequestID: requestID,
		UserID:    userID,
		TraceID:   traceID,
		State:     make(map[string]interface{}),
		CreatedAt: time.Now(),
	}
}

// Context returns the underlying context.Context
func (ac *AgentContext) Context() context.Context {
	return ac.ctx
}

// SetState sets a value in the state
func (ac *AgentContext) SetState(key string, value interface{}) {
	ac.State[key] = value
}

// GetState retrieves a value from the state
func (ac *AgentContext) GetState(key string) (interface{}, bool) {
	val, ok := ac.State[key]
	return val, ok
}

// AgentConfig holds configuration for an agent
type AgentConfig struct {
	Name        string                 `json:"name"`
	Type        AgentType              `json:"type"`
	Description string                 `json:"description"`
	MaxRetries  int                    `json:"max_retries"`
	Timeout     time.Duration          `json:"timeout"`
	LLMConfig   *LLMConfig             `json:"llm_config,omitempty"`
	Metadata    map[string]interface{} `json:"metadata,omitempty"`
}

// LLMConfig holds LLM configuration
type LLMConfig struct {
	Provider    string  `json:"provider"`
	Model       string  `json:"model"`
	APIKey      string  `json:"api_key"`
	BaseURL     string  `json:"base_url"`
	Temperature float32 `json:"temperature"`
	MaxTokens   int     `json:"max_tokens"`
}

// AgentMetrics tracks agent execution metrics
type AgentMetrics struct {
	AgentType       AgentType     `json:"agent_type"`
	TasksCompleted  int64         `json:"tasks_completed"`
	TasksFailed     int64         `json:"tasks_failed"`
	TotalDuration   time.Duration `json:"total_duration"`
	AverageDuration time.Duration `json:"average_duration"`
	LastExecutedAt  time.Time     `json:"last_executed_at"`
}
