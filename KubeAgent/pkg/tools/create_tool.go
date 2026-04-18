package tools

import (
	"context"
	"fmt"

	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	yamlutil "k8s.io/apimachinery/pkg/runtime/serializer/yaml"

	"kubeagent/pkg/agent/harness"
	"kubeagent/pkg/k8s"
)

// CreateTool creates a K8s resource directly via the Kubernetes API.
// The caller (agent/LLM) is responsible for generating the YAML content.
//
// Harness wiring (optional): see DeleteTool for rationale. CreateTool
// performs a lightweight YAML peek before calling the chain so the
// Guide can see kind/name/namespace without the full RESTMapping.
type CreateTool struct {
	client    *k8s.Client
	preflight *harness.PreflightChain
	auditor   harness.AuditLogger
}

func NewCreateTool(client *k8s.Client) *CreateTool {
	return &CreateTool{client: client}
}

// WithPreflight attaches a Guide chain. Nil is tolerated (no-op).
func (c *CreateTool) WithPreflight(chain *harness.PreflightChain) *CreateTool {
	c.preflight = chain
	return c
}

// WithAuditor attaches an audit sink for preflight events. Nil is tolerated.
func (c *CreateTool) WithAuditor(a harness.AuditLogger) *CreateTool {
	c.auditor = a
	return c
}

func (c *CreateTool) Name() string {
	return "CreateTool"
}

func (c *CreateTool) Description() string {
	return "用于在 Kubernetes 集群中创建资源（Pod、Service、Deployment 等），需要提供资源 YAML 内容"
}

func (c *CreateTool) ArgsSchema() string {
	return `{"type":"object","properties":{"yaml":{"type":"string","description":"要创建的 K8s 资源的 YAML 内容"}},"required":["yaml"]}`
}

func (c *CreateTool) Execute(params map[string]any) (string, error) {
	yamlContent, ok := params["yaml"].(string)
	if !ok || yamlContent == "" {
		return "", fmt.Errorf("yaml is required")
	}

	// Preflight guard. Peek at the YAML first so protected-namespace
	// and other namespace-scoped checks can fire. A decode failure here
	// is NOT a preflight block — we let the real client.CreateResource
	// return the same decode error the LLM already expects, and skip
	// preflight so a genuinely mis-specified kind doesn't masquerade as
	// a policy violation.
	if c.preflight != nil {
		if kind, name, ns, peekErr := peekResource(yamlContent); peekErr == nil {
			req := harness.PreflightRequest{
				Verb:         "create",
				ResourceKind: kind,
				ResourceName: name,
				Namespace:    ns,
			}
			res := c.preflight.Run(context.TODO(), req)
			c.recordPreflight(req, res)
			if res.Decision == harness.PreflightBlock {
				return "", fmt.Errorf("create blocked by preflight: %s", res.Reason)
			}
		}
	}

	return c.client.CreateResource(yamlContent)
}

// peekResource decodes just enough of the YAML to extract (kind, name,
// namespace) for policy decisions. It reuses the same decoder as the
// real create path so a successful peek guarantees the real call will
// at least get past YAML parsing.
func peekResource(yamlContent string) (kind, name, namespace string, err error) {
	obj := &unstructured.Unstructured{}
	dec := yamlutil.NewDecodingSerializer(unstructured.UnstructuredJSONScheme)
	if _, _, derr := dec.Decode([]byte(yamlContent), nil, obj); derr != nil {
		return "", "", "", derr
	}
	ns := obj.GetNamespace()
	if ns == "" {
		ns = "default"
	}
	return obj.GetKind(), obj.GetName(), ns, nil
}

// recordPreflight emits an audit event. See DeleteTool for policy
// rationale; this is intentionally a near-duplicate (the two write
// tools don't yet share a base because Tool is a tiny interface and
// factoring would add ceremony without removing much code).
func (c *CreateTool) recordPreflight(req harness.PreflightRequest, res *harness.PreflightResult) {
	if c.auditor == nil || res == nil {
		return
	}
	details := map[string]interface{}{}
	if len(res.Warnings) > 0 {
		details["warnings"] = res.Warnings
	}
	_ = c.auditor.Record(context.TODO(), harness.AuditEvent{
		Kind:    harness.AuditPreflight,
		Actor:   "CreateTool",
		Action:  "create",
		Outcome: string(res.Decision),
		Reason:  res.Reason,
		Target: harness.AuditTarget{
			Kind:      req.ResourceKind,
			Name:      req.ResourceName,
			Namespace: req.Namespace,
		},
		Details: details,
	})
}
