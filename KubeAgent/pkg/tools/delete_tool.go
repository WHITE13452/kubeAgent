package tools

import (
	"context"
	"fmt"
	"strings"

	"kubeagent/pkg/agent/harness"
	"kubeagent/pkg/k8s"
)

// DeleteTool deletes a K8s resource directly via the Kubernetes API.
// Dangerous: must only be called after HumanTool returns "approved".
//
// Harness wiring (optional):
//   - WithPreflight installs a PreflightChain (Guide) that runs before the
//     real delete. A Block verdict returns an error to the LLM, which the
//     tool-use loop surfaces back as a tool_result the model can reason about.
//   - WithAuditor installs an AuditLogger sink; a preflight event is emitted
//     on every call regardless of decision, so the audit trail captures blocks.
//
// Both are optional — an unwired DeleteTool behaves exactly as before.
type DeleteTool struct {
	client    *k8s.Client
	preflight *harness.PreflightChain
	auditor   harness.AuditLogger
}

func NewDeleteTool(client *k8s.Client) *DeleteTool {
	return &DeleteTool{client: client}
}

// WithPreflight attaches a Guide chain. Nil is tolerated (no-op).
func (d *DeleteTool) WithPreflight(chain *harness.PreflightChain) *DeleteTool {
	d.preflight = chain
	return d
}

// WithAuditor attaches an audit sink for preflight events. Nil is tolerated.
func (d *DeleteTool) WithAuditor(a harness.AuditLogger) *DeleteTool {
	d.auditor = a
	return d
}

func (d *DeleteTool) Name() string {
	return "DeleteTool"
}

func (d *DeleteTool) Description() string {
	return "用于删除 Kubernetes 集群中的指定资源，这是危险操作，必须先通过 HumanTool 获得用户确认"
}

func (d *DeleteTool) ArgsSchema() string {
	return `{"type":"object","properties":{"resource":{"type":"string","description":"K8s 资源类型，例如 pod、service"},"name":{"type":"string","description":"资源实例的名称"},"namespace":{"type":"string","description":"资源所在命名空间"}},"required":["resource","name","namespace"]}`
}

func (d *DeleteTool) Execute(params map[string]any) (string, error) {
	resource, ok := params["resource"].(string)
	if !ok || resource == "" {
		return "", fmt.Errorf("resource is required")
	}
	name, ok := params["name"].(string)
	if !ok || name == "" {
		return "", fmt.Errorf("name is required")
	}
	namespace, ok := params["namespace"].(string)
	if !ok || namespace == "" {
		namespace = "default"
	}

	resource = strings.ToLower(resource)

	// Preflight guard. We intentionally do this AFTER parameter validation
	// so the chain never sees malformed requests, and BEFORE any API call
	// so a Block prevents the mutation entirely.
	if d.preflight != nil {
		req := harness.PreflightRequest{
			Verb:         "delete",
			ResourceKind: resource,
			ResourceName: name,
			Namespace:    namespace,
		}
		res := d.preflight.Run(context.TODO(), req)

		d.recordPreflight(req, res)

		if res.Decision == harness.PreflightBlock {
			return "", fmt.Errorf("delete blocked by preflight: %s", res.Reason)
		}
	}

	return d.client.DeleteResource(resource, name, namespace)
}

// recordPreflight emits an audit event for the guard decision. Errors
// from the sink are swallowed — an audit failure must never break a
// tool call. Nil auditor is a no-op.
func (d *DeleteTool) recordPreflight(req harness.PreflightRequest, res *harness.PreflightResult) {
	if d.auditor == nil || res == nil {
		return
	}
	outcome := string(res.Decision)
	details := map[string]interface{}{}
	if len(res.Warnings) > 0 {
		details["warnings"] = res.Warnings
	}
	_ = d.auditor.Record(context.TODO(), harness.AuditEvent{
		Kind:    harness.AuditPreflight,
		Actor:   "DeleteTool",
		Action:  "delete",
		Outcome: outcome,
		Reason:  res.Reason,
		Target: harness.AuditTarget{
			Kind:      req.ResourceKind,
			Name:      req.ResourceName,
			Namespace: req.Namespace,
		},
		Details: details,
	})
}
