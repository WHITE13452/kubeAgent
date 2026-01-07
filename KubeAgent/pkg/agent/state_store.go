package agent

import (
	"context"
	"fmt"
	"sync"
)

// MemoryStateStore is an in-memory implementation of StateStore
type MemoryStateStore struct {
	contexts map[string]*AgentContext
	tasks    map[string]*Task
	plans    map[string]*ExecutionPlan
	mu       sync.RWMutex
}

// NewMemoryStateStore creates a new in-memory state store
func NewMemoryStateStore() *MemoryStateStore {
	return &MemoryStateStore{
		contexts: make(map[string]*AgentContext),
		tasks:    make(map[string]*Task),
		plans:    make(map[string]*ExecutionPlan),
	}
}

// SaveContext saves the agent context
func (m *MemoryStateStore) SaveContext(ctx context.Context, agentCtx *AgentContext) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	m.contexts[agentCtx.RequestID] = agentCtx
	return nil
}

// LoadContext loads the agent context
func (m *MemoryStateStore) LoadContext(ctx context.Context, requestID string) (*AgentContext, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()

	agentCtx, exists := m.contexts[requestID]
	if !exists {
		return nil, fmt.Errorf("context not found for request ID: %s", requestID)
	}

	return agentCtx, nil
}

// SaveTask saves a task
func (m *MemoryStateStore) SaveTask(ctx context.Context, task *Task) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	m.tasks[task.ID] = task
	return nil
}

// LoadTask loads a task
func (m *MemoryStateStore) LoadTask(ctx context.Context, taskID string) (*Task, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()

	task, exists := m.tasks[taskID]
	if !exists {
		return nil, fmt.Errorf("task not found: %s", taskID)
	}

	return task, nil
}

// SavePlan saves an execution plan
func (m *MemoryStateStore) SavePlan(ctx context.Context, plan *ExecutionPlan) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	m.plans[plan.ID] = plan
	return nil
}

// LoadPlan loads an execution plan
func (m *MemoryStateStore) LoadPlan(ctx context.Context, planID string) (*ExecutionPlan, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()

	plan, exists := m.plans[planID]
	if !exists {
		return nil, fmt.Errorf("plan not found: %s", planID)
	}

	return plan, nil
}

// UpdateTaskStatus updates a task's status
func (m *MemoryStateStore) UpdateTaskStatus(ctx context.Context, taskID string, status TaskStatus) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	task, exists := m.tasks[taskID]
	if !exists {
		return fmt.Errorf("task not found: %s", taskID)
	}

	task.Status = status
	return nil
}

// GetAllTasks returns all tasks (useful for debugging)
func (m *MemoryStateStore) GetAllTasks() []*Task {
	m.mu.RLock()
	defer m.mu.RUnlock()

	tasks := make([]*Task, 0, len(m.tasks))
	for _, task := range m.tasks {
		tasks = append(tasks, task)
	}
	return tasks
}

// GetAllPlans returns all plans (useful for debugging)
func (m *MemoryStateStore) GetAllPlans() []*ExecutionPlan {
	m.mu.RLock()
	defer m.mu.RUnlock()

	plans := make([]*ExecutionPlan, 0, len(m.plans))
	for _, plan := range m.plans {
		plans = append(plans, plan)
	}
	return plans
}

// Clear clears all stored data (useful for testing)
func (m *MemoryStateStore) Clear() {
	m.mu.Lock()
	defer m.mu.Unlock()

	m.contexts = make(map[string]*AgentContext)
	m.tasks = make(map[string]*Task)
	m.plans = make(map[string]*ExecutionPlan)
}
