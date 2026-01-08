package agent

import (
	"context"
	"sync"
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

func TestDependencyValidation(t *testing.T) {
	logger := NewNoOpLogger()
	stateStore := NewMemoryStateStore()
	llmClient := &MockLLMClient{}
	coordinator := NewCoordinator(nil, llmClient, stateStore, logger)

	// Test 1: Valid dependencies (no cycles)
	tasks1 := []*Task{
		{ID: "task1", Dependencies: []string{}},
		{ID: "task2", Dependencies: []string{"task1"}},
		{ID: "task3", Dependencies: []string{"task1", "task2"}},
	}
	err := coordinator.validateDependencies(tasks1)
	if err != nil {
		t.Errorf("Valid dependencies should not return error: %v", err)
	}

	// Test 2: Circular dependency
	tasks2 := []*Task{
		{ID: "task1", Dependencies: []string{"task2"}},
		{ID: "task2", Dependencies: []string{"task1"}},
	}
	err = coordinator.validateDependencies(tasks2)
	if err == nil {
		t.Error("Circular dependency should return error")
	}

	// Test 3: Invalid dependency (non-existent task)
	tasks3 := []*Task{
		{ID: "task1", Dependencies: []string{"nonexistent"}},
	}
	err = coordinator.validateDependencies(tasks3)
	if err == nil {
		t.Error("Invalid dependency should return error")
	}

	// Test 4: Complex circular dependency
	tasks4 := []*Task{
		{ID: "task1", Dependencies: []string{"task2"}},
		{ID: "task2", Dependencies: []string{"task3"}},
		{ID: "task3", Dependencies: []string{"task1"}},
	}
	err = coordinator.validateDependencies(tasks4)
	if err == nil {
		t.Error("Complex circular dependency should return error")
	}
}

func TestDependencyBasedExecution(t *testing.T) {
	logger := NewNoOpLogger()
	stateStore := NewMemoryStateStore()
	llmClient := &MockLLMClient{}
	coordinator := NewCoordinator(nil, llmClient, stateStore, logger)

	// Track execution order
	executionOrder := []string{}
	var mu sync.Mutex

	// Register mock agent that tracks execution order
	mockAgent := &MockSpecialistAgent{
		name:      "mock-agent",
		agentType: AgentTypeDiagnostician,
		canHandleFunc: func(taskType TaskType) bool {
			return true
		},
		executeFunc: func(ctx *AgentContext, task *Task) (*Task, error) {
			mu.Lock()
			executionOrder = append(executionOrder, task.ID)
			mu.Unlock()

			// Simulate some work
			time.Sleep(10 * time.Millisecond)

			task.Status = TaskStatusCompleted
			task.Output = map[string]interface{}{
				"result": "Task " + task.ID + " completed",
			}
			return task, nil
		},
	}
	coordinator.RegisterAgent(mockAgent)

	// Create test context
	ctx := NewAgentContext(
		context.Background(),
		"test-req-dep-001",
		"test-user",
		"test-trace-dep-001",
	)

	// Create test plan with dependencies
	// task1 has no dependencies
	// task2 depends on task1
	// task3 depends on task1
	// task4 depends on task2 and task3
	plan := &ExecutionPlan{
		ID:        "test-plan-001",
		RequestID: "test-req-dep-001",
		Tasks: []*Task{
			{
				ID:            "task1",
				Type:          TaskTypeDiagnose,
				Description:   "Task 1",
				Status:        TaskStatusPending,
				AssignedAgent: AgentTypeDiagnostician,
				Input:         map[string]interface{}{},
				Dependencies:  []string{},
				CreatedAt:     time.Now(),
			},
			{
				ID:            "task2",
				Type:          TaskTypeDiagnose,
				Description:   "Task 2",
				Status:        TaskStatusPending,
				AssignedAgent: AgentTypeDiagnostician,
				Input:         map[string]interface{}{},
				Dependencies:  []string{"task1"},
				CreatedAt:     time.Now(),
			},
			{
				ID:            "task3",
				Type:          TaskTypeDiagnose,
				Description:   "Task 3",
				Status:        TaskStatusPending,
				AssignedAgent: AgentTypeDiagnostician,
				Input:         map[string]interface{}{},
				Dependencies:  []string{"task1"},
				CreatedAt:     time.Now(),
			},
			{
				ID:            "task4",
				Type:          TaskTypeDiagnose,
				Description:   "Task 4",
				Status:        TaskStatusPending,
				AssignedAgent: AgentTypeDiagnostician,
				Input:         map[string]interface{}{},
				Dependencies:  []string{"task2", "task3"},
				CreatedAt:     time.Now(),
			},
		},
		ExecutionMode: ExecutionModeSequential, // Will be overridden by dependency execution
		Status:        TaskStatusPending,
		CreatedAt:     time.Now(),
		UpdatedAt:     time.Now(),
	}

	// Execute plan
	response, err := coordinator.executeDependencyBased(ctx, plan)
	if err != nil {
		t.Fatalf("Dependency-based execution failed: %v", err)
	}

	// Verify all tasks completed
	if len(executionOrder) != 4 {
		t.Errorf("Expected 4 tasks to execute, got %d", len(executionOrder))
	}

	// Verify execution order respects dependencies
	task1Idx := -1
	task2Idx := -1
	task3Idx := -1
	task4Idx := -1

	for i, taskID := range executionOrder {
		switch taskID {
		case "task1":
			task1Idx = i
		case "task2":
			task2Idx = i
		case "task3":
			task3Idx = i
		case "task4":
			task4Idx = i
		}
	}

	// task1 should execute before task2 and task3
	if task1Idx >= task2Idx || task1Idx >= task3Idx {
		t.Errorf("task1 should execute before task2 and task3. Order: %v", executionOrder)
	}

	// task2 and task3 should execute before task4
	if task2Idx >= task4Idx || task3Idx >= task4Idx {
		t.Errorf("task2 and task3 should execute before task4. Order: %v", executionOrder)
	}

	// Verify response
	if response == nil {
		t.Fatal("Expected response, got nil")
	}

	if len(response.Errors) > 0 {
		t.Errorf("Expected no errors, got: %v", response.Errors)
	}
}
