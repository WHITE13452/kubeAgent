package specialists

import (
	"encoding/json"
	"fmt"
	"os"
	"time"

	"kubeagent/pkg/agent"
	"kubeagent/pkg/agent/harness"
)

// RemediatorAgent specializes in generating and applying fixes.
//
// As of the harness refactor, Remediator no longer fire-and-forgets.
// After the LLM-driven tool loop finishes, Remediator runs a
// post-action Verifier (the Sensor) and writes the outcome to the
// AuditLogger. Both components are optional: omit them and the agent
// behaves as before.
type RemediatorAgent struct {
	*agent.BaseAgent

	// verifier closes the loop on each remediation. Defaults to
	// harness.NoopVerifier when not set.
	verifier harness.Verifier

	// auditor records what the agent did and what verification said.
	// Defaults to harness.NoopAuditor when not set.
	auditor harness.AuditLogger

	// skills sources the LLM-facing system prompt. nil means use the
	// inline fallback (kept for backward compatibility).
	skills *harness.Skills
}

// NewRemediatorAgent creates a new remediator agent without any
// harness sensors wired in. Call WithVerifier / WithAuditor to enable
// post-action verification and structured audit logging.
func NewRemediatorAgent(llmClient agent.LLMClient, logger agent.Logger) *RemediatorAgent {
	config := &agent.AgentConfig{
		Name:        "remediator",
		Type:        agent.AgentTypeRemediator,
		Description: "Generates fixes, creates patches, and remediates issues",
		MaxRetries:  3,
		Timeout:     2 * time.Minute,
	}

	return &RemediatorAgent{
		BaseAgent: agent.NewBaseAgent(config, llmClient, logger),
		verifier:  harness.NoopVerifier{},
		auditor:   harness.NoopAuditor{},
	}
}

// WithVerifier installs a post-action verifier. Pass nil to keep the
// no-op default. Returned receiver enables fluent wiring.
func (r *RemediatorAgent) WithVerifier(v harness.Verifier) *RemediatorAgent {
	if v != nil {
		r.verifier = v
	}
	return r
}

// WithAuditor installs an audit sink. Pass nil to keep the no-op default.
func (r *RemediatorAgent) WithAuditor(a harness.AuditLogger) *RemediatorAgent {
	if a != nil {
		r.auditor = a
	}
	return r
}

// WithSkills installs a Skills registry for prompt sourcing.
// Pass nil to keep the inline fallback prompt.
func (r *RemediatorAgent) WithSkills(s *harness.Skills) *RemediatorAgent {
	r.skills = s
	return r
}

// CanHandle checks if the remediator can handle a task type
func (r *RemediatorAgent) CanHandle(taskType agent.TaskType) bool {
	return taskType == agent.TaskTypeRemediate
}

// Execute runs the remediation tool loop, then verifies the outcome.
//
// Verification policy:
//   - If the LLM tool loop fails outright, we still emit an AuditAction
//     with outcome="failure" and skip verification (nothing to verify).
//   - If the LLM finishes but the diagnosis lacked a concrete target
//     (no pod_name / namespace), verification falls through to
//     Inconclusive. We surface that to the user rather than pretending
//     we confirmed a fix.
//   - The verification result becomes part of task.Output under
//     "verification", so downstream consumers (Coordinator, UI, tests)
//     can inspect it without re-running the check.
func (r *RemediatorAgent) Execute(ctx *agent.AgentContext, task *agent.Task) (*agent.Task, error) {
	startTime := time.Now()

	task.Status = agent.TaskStatusRunning
	now := time.Now()
	task.StartedAt = &now

	rootCause, _ := task.Input["root_cause"].(string)
	errorType, _ := task.Input["error_type"].(string)
	diagnosis, _ := task.Input["diagnosis"].(map[string]any)

	if rootCause == "" {
		rootCause = task.Description
	}

	// Phase 1: run the LLM-driven remediation tool loop.
	result, err := r.remediate(ctx, rootCause, errorType, diagnosis)
	if err != nil {
		// Audit the failure before returning, so operators see it.
		r.audit(ctx, harness.AuditAction, task,
			"remediation_failed", "failure", err.Error(), nil)

		task.Status = agent.TaskStatusFailed
		task.Error = err.Error()
		completedAt := time.Now()
		task.CompletedAt = &completedAt
		return task, err
	}

	// Audit the successful action — note "successful" only means the
	// LLM tool loop returned without error; the Verifier decides whether
	// the cluster actually converged.
	r.audit(ctx, harness.AuditAction, task,
		"remediation_applied", "success", "",
		map[string]interface{}{"remediation": result})

	// Phase 2: verify the action took effect.
	verification := r.verifyOutcome(ctx, task)
	if verification != nil {
		result["verification"] = verification

		// Audit the verification outcome regardless of result so the
		// audit trail tells the full story.
		r.audit(ctx, harness.AuditVerification, task,
			"post_action_verify", string(verification.Status),
			verification.Summary, verification.Observations)

		// A failed verification flips the task status to Failed even
		// though the LLM thought it was done. This is the whole point
		// of the Sensor: catch open-loop optimism.
		if verification.Status == harness.VerificationFailed {
			task.Status = agent.TaskStatusFailed
			task.Error = "post-action verification failed: " + verification.Summary
			task.Output = result
			task.Output["remediation_time"] = time.Since(startTime).String()
			completedAt := time.Now()
			task.CompletedAt = &completedAt
			return task, fmt.Errorf("verification failed: %s", verification.Summary)
		}
	}

	task.Status = agent.TaskStatusCompleted
	task.Output = result
	task.Output["remediation_time"] = time.Since(startTime).String()

	completedAt := time.Now()
	task.CompletedAt = &completedAt

	return task, nil
}

// Analyze implements SpecialistAgent
func (r *RemediatorAgent) Analyze(ctx *agent.AgentContext, input map[string]any) (map[string]any, error) {
	rootCause, _ := input["root_cause"].(string)
	errorType, _ := input["error_type"].(string)
	diagnosis, _ := input["diagnosis"].(map[string]any)

	return r.remediate(ctx, rootCause, errorType, diagnosis)
}

// remediate runs the agentic tool-use loop to generate and apply fixes
func (r *RemediatorAgent) remediate(ctx *agent.AgentContext, rootCause, errorType string, diagnosis map[string]any) (map[string]any, error) {
	systemPrompt := r.remediatePrompt()

	diagnosisJSON, _ := json.Marshal(diagnosis)
	userPrompt := fmt.Sprintf(`Remediate the following Kubernetes issue:

Root Cause: %s
Error Type: %s
Diagnosis Details: %s

Use the available tools to fix the issue. Ask for human confirmation before applying dangerous changes.
Return a summary in JSON format when done.`, rootCause, errorType, string(diagnosisJSON))

	response, err := r.RunToolLoop(ctx, systemPrompt, userPrompt, 0)
	if err != nil {
		return nil, fmt.Errorf("remediation failed: %w", err)
	}

	var result map[string]any
	if err := json.Unmarshal([]byte(response), &result); err != nil {
		// LLM returned non-JSON text - wrap it
		return map[string]any{
			"remediation_type":   "manual",
			"actions_taken":      []string{response},
			"verification_steps": []string{"Verify the fix manually"},
			"risk_level":         "medium",
		}, nil
	}

	return result, nil
}

// verifyOutcome runs the configured Verifier against whatever target the
// task carried. We extract the target from task.Input (the same fields
// the Diagnostician produced), because that is the most authoritative
// source for "what we were trying to fix".
//
// Returns nil when verification is impossible (no target to check). The
// caller treats nil as "no information added" rather than as success.
func (r *RemediatorAgent) verifyOutcome(ctx *agent.AgentContext, task *agent.Task) *harness.VerificationResult {
	target := extractVerificationTarget(task)
	if target.ResourceKind == "" || target.ResourceName == "" {
		// Nothing concrete to verify — be honest about it.
		return &harness.VerificationResult{
			Status:    harness.VerificationInconclusive,
			Summary:   "no concrete resource target available for verification",
			CheckedAt: time.Now(),
		}
	}

	result, err := r.verifier.Verify(ctx.Context(), target)
	if err != nil {
		// A Verifier error is not a verification failure; it means we
		// could not even attempt the check. Surface as Inconclusive.
		return &harness.VerificationResult{
			Status:    harness.VerificationInconclusive,
			Summary:   fmt.Sprintf("verifier error: %v", err),
			CheckedAt: time.Now(),
		}
	}
	return result
}

// extractVerificationTarget derives a verification target from the task
// input. Conventions follow what the Diagnostician populates today:
// pod_name + namespace. Future agents that mutate other kinds should
// add their own keys here.
func extractVerificationTarget(task *agent.Task) harness.VerificationTarget {
	target := harness.VerificationTarget{}
	if task == nil || task.Input == nil {
		return target
	}

	if v, ok := task.Input["resource_kind"].(string); ok && v != "" {
		target.ResourceKind = v
	}
	if v, ok := task.Input["resource_name"].(string); ok && v != "" {
		target.ResourceName = v
	}
	if target.ResourceKind == "" {
		// Back-compat: most diagnostician outputs only carry pod_name.
		if _, ok := task.Input["pod_name"]; ok {
			target.ResourceKind = "Pod"
		} else if _, ok := task.Input["podName"]; ok {
			target.ResourceKind = "Pod"
		}
	}
	if target.ResourceName == "" {
		if v, ok := task.Input["pod_name"].(string); ok && v != "" {
			target.ResourceName = v
		} else if v, ok := task.Input["podName"].(string); ok && v != "" {
			target.ResourceName = v
		}
	}
	if v, ok := task.Input["namespace"].(string); ok && v != "" {
		target.Namespace = v
	}
	if v, ok := task.Input["expected_phase"].(string); ok && v != "" {
		target.ExpectedPhase = v
	}
	return target
}

// audit is a tiny helper that swallows audit errors (they are logged via
// the standard logger) so a flaky audit sink can never break a real
// remediation.
func (r *RemediatorAgent) audit(
	ctx *agent.AgentContext,
	kind harness.AuditEventKind,
	task *agent.Task,
	action, outcome, reason string,
	details map[string]interface{},
) {
	if r.auditor == nil {
		return
	}
	event := harness.AuditEvent{
		Kind:      kind,
		RequestID: ctx.RequestID,
		TraceID:   ctx.TraceID,
		Actor:     "remediator",
		Action:    action,
		Outcome:   outcome,
		Reason:    reason,
		Details:   details,
	}
	if task != nil {
		event.Target = harness.AuditTarget{
			Kind:      stringFrom(task.Input, "resource_kind"),
			Name:      firstNonEmpty(stringFrom(task.Input, "resource_name"), stringFrom(task.Input, "pod_name"), stringFrom(task.Input, "podName")),
			Namespace: stringFrom(task.Input, "namespace"),
		}
	}
	if err := r.auditor.Record(ctx.Context(), event); err != nil {
		// Audit failure must NEVER break remediation. Drop to stderr so
		// the failure is at least visible to the operator running the
		// process, then continue. We deliberately avoid the agent
		// logger here to keep this method free of indirect deps.
		fmt.Fprintf(os.Stderr, "[audit] failed to record event: %v\n", err)
	}
}

func stringFrom(m map[string]interface{}, key string) string {
	if m == nil {
		return ""
	}
	if v, ok := m[key].(string); ok {
		return v
	}
	return ""
}

func firstNonEmpty(values ...string) string {
	for _, v := range values {
		if v != "" {
			return v
		}
	}
	return ""
}

// fallbackRemediatePrompt mirrors pkg/agent/skills/remediate.md so the
// agent still works when no Skills registry is wired in. Keep the two
// in sync; the markdown file is authoritative.
const fallbackRemediatePrompt = `You are a Kubernetes remediation expert. You have tools to create/delete resources, execute kubectl commands, and ask for human approval.

Your workflow:
1. Analyze the diagnosis and generate a remediation plan
2. For dangerous operations (delete, modify production resources), use the HumanTool to ask for confirmation before proceeding
3. Apply the fix using available tools (CreateTool, DeleteTool, KubeTool)
4. Report what actions were taken

Return your final result in JSON format:
{
  "remediation_type": "patch|config_change|restart|scale",
  "actions_taken": ["action1", "action2"],
  "verification_steps": ["Step 1", "Step 2"],
  "risk_level": "low|medium|high"
}`

// remediatePrompt returns the active system prompt. Skills win over the
// inline fallback when registered.
func (r *RemediatorAgent) remediatePrompt() string {
	if r.skills != nil {
		if body, ok := r.skills.Get("remediate"); ok && body != "" {
			return body
		}
	}
	return fallbackRemediatePrompt
}
