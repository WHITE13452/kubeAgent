package main

import (
	"context"
	"encoding/json"
	"fmt"
	"log"

	"kubeagent/pkg/agent"
	"kubeagent/pkg/agent/specialists"
)

func main() {
	fmt.Println("=== KubeAgent Multi-Agent Framework Demo ===\n")

	// Initialize components
	logger := agent.NewSimpleLogger("KubeAgent")
	stateStore := agent.NewMemoryStateStore()

	// Initialize LLM client
	llmClient, err := agent.NewOpenAILLMClient(nil) // Uses default config from env
	if err != nil {
		log.Fatalf("Failed to create LLM client: %v", err)
	}

	// Create coordinator
	coordinator := agent.NewCoordinator(nil, llmClient, stateStore, logger)

	// Create and register specialist agents
	diagnostician := specialists.NewDiagnosticianAgent(llmClient, logger)
	remediator := specialists.NewRemediatorAgent(llmClient, logger)

	if err := coordinator.RegisterAgent(diagnostician); err != nil {
		log.Fatalf("Failed to register diagnostician: %v", err)
	}

	if err := coordinator.RegisterAgent(remediator); err != nil {
		log.Fatalf("Failed to register remediator: %v", err)
	}

	fmt.Println("✓ Coordinator and specialist agents initialized")
	fmt.Println("✓ Registered agents: Diagnostician, Remediator\n")

	// Example 1: Simple diagnosis task
	runDiagnosisExample(coordinator, stateStore, logger)

	// Example 2: Diagnosis + Remediation workflow
	runDiagnosisRemediationWorkflow(coordinator, stateStore, logger)

	// Example 3: Full request with automatic planning
	runFullRequestExample(coordinator, stateStore, logger)

	// Display metrics
	displayMetrics(coordinator)
}

func runDiagnosisExample(coordinator *agent.BaseCoordinator, stateStore *agent.MemoryStateStore, logger agent.Logger) {
	fmt.Println("\n=== Example 1: Simple Diagnosis Task ===")

	ctx := agent.NewAgentContext(
		context.Background(),
		"req-001",
		"user@example.com",
		"trace-001",
	)

	task := &agent.Task{
		ID:            "task-001",
		Type:          agent.TaskTypeDiagnose,
		Description:   "nginx-deployment-7d5c8b9f4d-x8k2l pod is in CrashLoopBackOff state",
		Status:        agent.TaskStatusPending,
		AssignedAgent: agent.AgentTypeDiagnostician,
		Input: map[string]interface{}{
			"pod_name":  "nginx-deployment-7d5c8b9f4d-x8k2l",
			"namespace": "production",
		},
	}

	result, err := coordinator.Execute(ctx, task)
	if err != nil {
		fmt.Printf("✗ Diagnosis failed: %v\n", err)
		return
	}

	fmt.Println("✓ Diagnosis completed")
	printTaskResult(result)
}

func runDiagnosisRemediationWorkflow(coordinator *agent.BaseCoordinator, stateStore *agent.MemoryStateStore, logger agent.Logger) {
	fmt.Println("\n=== Example 2: Diagnosis + Remediation Workflow ===")

	ctx := agent.NewAgentContext(
		context.Background(),
		"req-002",
		"user@example.com",
		"trace-002",
	)

	// Step 1: Diagnosis
	diagnosisTask := &agent.Task{
		ID:            "task-002-1",
		Type:          agent.TaskTypeDiagnose,
		Description:   "Pod is running out of memory and getting OOMKilled",
		Status:        agent.TaskStatusPending,
		AssignedAgent: agent.AgentTypeDiagnostician,
		Input: map[string]interface{}{
			"pod_name":  "api-server-5f7b8c9d6e-abc12",
			"namespace": "production",
		},
	}

	diagnosisResult, err := coordinator.Execute(ctx, diagnosisTask)
	if err != nil {
		fmt.Printf("✗ Diagnosis failed: %v\n", err)
		return
	}

	fmt.Println("✓ Step 1: Diagnosis completed")
	fmt.Printf("  Root Cause: %v\n", diagnosisResult.Output["root_cause"])

	// Step 2: Remediation
	remediationTask := &agent.Task{
		ID:            "task-002-2",
		Type:          agent.TaskTypeRemediate,
		Description:   "Generate fix for OOMKilled issue",
		Status:        agent.TaskStatusPending,
		AssignedAgent: agent.AgentTypeRemediator,
		Input: map[string]interface{}{
			"diagnosis":  diagnosisResult.Output,
			"root_cause": diagnosisResult.Output["root_cause"],
			"error_type": diagnosisResult.Output["error_type"],
		},
	}

	remediationResult, err := coordinator.Execute(ctx, remediationTask)
	if err != nil {
		fmt.Printf("✗ Remediation failed: %v\n", err)
		return
	}

	fmt.Println("✓ Step 2: Remediation plan generated")
	fmt.Printf("  Remediation Type: %v\n", remediationResult.Output["remediation_type"])
	fmt.Printf("  Risk Level: %v\n", remediationResult.Output["risk_level"])
	fmt.Printf("  Requires Approval: %v\n", remediationResult.Output["requires_approval"])
}

func runFullRequestExample(coordinator *agent.BaseCoordinator, stateStore *agent.MemoryStateStore, logger agent.Logger) {
	fmt.Println("\n=== Example 3: Full Request with Automatic Planning ===")

	ctx := agent.NewAgentContext(
		context.Background(),
		"req-003",
		"user@example.com",
		"trace-003",
	)

	request := &agent.Request{
		ID:    "req-003",
		User:  "user@example.com",
		Input: "My nginx pod keeps restarting. Can you diagnose and fix it?",
		Context: map[string]interface{}{
			"pod_name":  "nginx-7d5c8b9f4d-xyz99",
			"namespace": "default",
		},
	}

	// Create execution plan
	plan, err := coordinator.Plan(ctx, request)
	if err != nil {
		fmt.Printf("✗ Planning failed: %v\n", err)
		return
	}

	fmt.Printf("✓ Execution plan created with %d tasks\n", len(plan.Tasks))
	fmt.Printf("  Execution Mode: %s\n", plan.ExecutionMode)

	for i, task := range plan.Tasks {
		fmt.Printf("  Task %d: %s (assigned to %s)\n", i+1, task.Type, task.AssignedAgent)
	}

	// Execute plan
	response, err := coordinator.ExecutePlan(ctx, plan)
	if err != nil {
		fmt.Printf("✗ Execution failed: %v\n", err)
		return
	}

	fmt.Println("\n✓ Plan execution completed")
	fmt.Printf("  Status: %s\n", response.Status)
	fmt.Printf("  Executed by: %v\n", response.ExecutedBy)
	fmt.Printf("  Duration: %v\n", response.Duration)
	fmt.Printf("\n  Final Response:\n  %s\n", response.Result)
}

func printTaskResult(task *agent.Task) {
	fmt.Printf("  Task ID: %s\n", task.ID)
	fmt.Printf("  Status: %s\n", task.Status)
	if task.Output != nil {
		output, _ := json.MarshalIndent(task.Output, "  ", "  ")
		fmt.Printf("  Output:\n  %s\n", string(output))
	}
	if task.Error != "" {
		fmt.Printf("  Error: %s\n", task.Error)
	}
}

func displayMetrics(coordinator *agent.BaseCoordinator) {
	fmt.Println("\n=== Agent Metrics ===")

	metrics := coordinator.GetMetrics()
	fmt.Printf("Coordinator:\n")
	fmt.Printf("  Tasks Completed: %d\n", metrics.TasksCompleted)
	fmt.Printf("  Tasks Failed: %d\n", metrics.TasksFailed)
	fmt.Printf("  Average Duration: %v\n", metrics.AverageDuration)

	// Get metrics from specialist agents
	diagnostician, _ := coordinator.GetAgent(agent.AgentTypeDiagnostician)
	if diagnostician != nil {
		if baseAgent, ok := diagnostician.(*specialists.DiagnosticianAgent); ok {
			diagMetrics := baseAgent.GetMetrics()
			fmt.Printf("\nDiagnostician:\n")
			fmt.Printf("  Tasks Completed: %d\n", diagMetrics.TasksCompleted)
			fmt.Printf("  Tasks Failed: %d\n", diagMetrics.TasksFailed)
			fmt.Printf("  Average Duration: %v\n", diagMetrics.AverageDuration)
		}
	}

	remediator, _ := coordinator.GetAgent(agent.AgentTypeRemediator)
	if remediator != nil {
		if baseAgent, ok := remediator.(*specialists.RemediatorAgent); ok {
			remedMetrics := baseAgent.GetMetrics()
			fmt.Printf("\nRemediator:\n")
			fmt.Printf("  Tasks Completed: %d\n", remedMetrics.TasksCompleted)
			fmt.Printf("  Tasks Failed: %d\n", remedMetrics.TasksFailed)
			fmt.Printf("  Average Duration: %v\n", remedMetrics.AverageDuration)
		}
	}
}
