package agent

import (
	"context"
	"testing"
	"time"
)

func TestCoordinatorBasics(t *testing.T) {
	// Setup
	logger := NewNoOpLogger()
	stateStore := NewMemoryStateStore()
	llmClient := &MockLLMClient{
		CompleteFunc: func(ctx context.Context, messages []Message) (string, error) {
			// Mock LLM response based on prompt content
			lastMsg := messages[len(messages)-1].Content
			if contains(lastMsg, "intent") {
				return "diagnose", nil
			}
			if contains(lastMsg, "Break down") {
				return `[{"type": "diagnose", "description": "Test diagnosis", "assigned_agent": "diagnostician", "input": {}}]`, nil
			}
			return "Test completed successfully", nil
		},
	}

	coordinator := NewCoordinator(nil, llmClient, stateStore, logger)

	// Test 1: Coordinator creation
	if coordinator.Name() != "coordinator" {
		t.Errorf("Expected coordinator name, got %s", coordinator.Name())
	}

	if coordinator.Type() != AgentTypeCoordinator {
		t.Errorf("Expected coordinator type, got %s", coordinator.Type())
	}

	// Test 2: Agent registration
	mockAgent := &MockSpecialistAgent{
		name:      "test-agent",
		agentType: AgentTypeDiagnostician,
		canHandleFunc: func(taskType TaskType) bool {
			return taskType == TaskTypeDiagnose
		},
	}

	err := coordinator.RegisterAgent(mockAgent)
	if err != nil {
		t.Errorf("Failed to register agent: %v", err)
	}

	// Test 3: Get registered agent
	agent, err := coordinator.GetAgent(AgentTypeDiagnostician)
	if err != nil {
		t.Errorf("Failed to get agent: %v", err)
	}

	if agent.Name() != "test-agent" {
		t.Errorf("Expected test-agent, got %s", agent.Name())
	}

	// Test 4: Duplicate registration should fail
	err = coordinator.RegisterAgent(mockAgent)
	if err == nil {
		t.Error("Expected error when registering duplicate agent")
	}
}

func TestCoordinatorExecution(t *testing.T) {
	logger := NewNoOpLogger()
	stateStore := NewMemoryStateStore()
	llmClient := &MockLLMClient{}

	coordinator := NewCoordinator(nil, llmClient, stateStore, logger)

	// Register mock agent
	mockAgent := &MockSpecialistAgent{
		name:      "mock-diagnostician",
		agentType: AgentTypeDiagnostician,
		canHandleFunc: func(taskType TaskType) bool {
			return taskType == TaskTypeDiagnose
		},
		executeFunc: func(ctx *AgentContext, task *Task) (*Task, error) {
			task.Status = TaskStatusCompleted
			task.Output = map[string]interface{}{
				"result": "Mock diagnosis completed",
			}
			return task, nil
		},
	}

	coordinator.RegisterAgent(mockAgent)

	// Create test context
	ctx := NewAgentContext(
		context.Background(),
		"test-req-001",
		"test-user",
		"test-trace-001",
	)

	// Create test task
	task := &Task{
		ID:            "test-task-001",
		Type:          TaskTypeDiagnose,
		Description:   "Test diagnosis task",
		Status:        TaskStatusPending,
		AssignedAgent: AgentTypeDiagnostician,
		Input:         map[string]interface{}{},
		CreatedAt:     time.Now(),
	}

	// Execute task
	result, err := coordinator.Execute(ctx, task)
	if err != nil {
		t.Errorf("Task execution failed: %v", err)
	}

	if result.Status != TaskStatusCompleted {
		t.Errorf("Expected task status completed, got %s", result.Status)
	}

	if result.Output == nil {
		t.Error("Expected task output, got nil")
	}
}

func TestStateSaving(t *testing.T) {
	stateStore := NewMemoryStateStore()

	// Test context saving/loading
	ctx := NewAgentContext(
		context.Background(),
		"test-req-001",
		"test-user",
		"test-trace-001",
	)

	err := stateStore.SaveContext(context.Background(), ctx)
	if err != nil {
		t.Errorf("Failed to save context: %v", err)
	}

	loadedCtx, err := stateStore.LoadContext(context.Background(), "test-req-001")
	if err != nil {
		t.Errorf("Failed to load context: %v", err)
	}

	if loadedCtx.RequestID != ctx.RequestID {
		t.Errorf("Expected request ID %s, got %s", ctx.RequestID, loadedCtx.RequestID)
	}

	// Test task saving/loading
	task := &Task{
		ID:          "test-task-001",
		Type:        TaskTypeDiagnose,
		Description: "Test task",
		Status:      TaskStatusPending,
		CreatedAt:   time.Now(),
	}

	err = stateStore.SaveTask(context.Background(), task)
	if err != nil {
		t.Errorf("Failed to save task: %v", err)
	}

	loadedTask, err := stateStore.LoadTask(context.Background(), "test-task-001")
	if err != nil {
		t.Errorf("Failed to load task: %v", err)
	}

	if loadedTask.ID != task.ID {
		t.Errorf("Expected task ID %s, got %s", task.ID, loadedTask.ID)
	}
}

// Helper functions and mock types

func contains(s, substr string) bool {
	return len(s) >= len(substr) && (s == substr || findSubstring(s, substr))
}

func findSubstring(s, substr string) bool {
	for i := 0; i <= len(s)-len(substr); i++ {
		if s[i:i+len(substr)] == substr {
			return true
		}
	}
	return false
}

// MockSpecialistAgent is a mock specialist agent for testing
type MockSpecialistAgent struct {
	name          string
	agentType     AgentType
	canHandleFunc func(TaskType) bool
	executeFunc   func(*AgentContext, *Task) (*Task, error)
}

func (m *MockSpecialistAgent) Name() string {
	return m.name
}

func (m *MockSpecialistAgent) Type() AgentType {
	return m.agentType
}

func (m *MockSpecialistAgent) Execute(ctx *AgentContext, task *Task) (*Task, error) {
	if m.executeFunc != nil {
		return m.executeFunc(ctx, task)
	}
	task.Status = TaskStatusCompleted
	return task, nil
}

func (m *MockSpecialistAgent) CanHandle(taskType TaskType) bool {
	if m.canHandleFunc != nil {
		return m.canHandleFunc(taskType)
	}
	return false
}

func (m *MockSpecialistAgent) Config() *AgentConfig {
	return &AgentConfig{
		Name: m.name,
		Type: m.agentType,
	}
}

func (m *MockSpecialistAgent) Analyze(ctx *AgentContext, input map[string]interface{}) (map[string]interface{}, error) {
	return map[string]interface{}{
		"result": "Mock analysis",
	}, nil
}
