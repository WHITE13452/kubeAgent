package harness

import (
	"context"
	"fmt"
	"strings"

	"kubeagent/pkg/k8s"
)

// PreflightDecision is the verdict returned by a Guide. It is
// deliberately separate from VerificationStatus because pre-action
// guards have a different vocabulary: they don't observe outcomes,
// they accept or block plans.
type PreflightDecision string

const (
	// PreflightAllow means the action may proceed.
	PreflightAllow PreflightDecision = "allow"

	// PreflightBlock means the action must NOT proceed. The Reason
	// field on the result explains why; callers should surface that to
	// the user (or feed it back into the LLM as a tool error).
	PreflightBlock PreflightDecision = "block"

	// PreflightWarn means the action may proceed but the caller should
	// log/surface the warning. Use for non-fatal concerns: e.g. action
	// targets a system namespace, but user has confirmed.
	PreflightWarn PreflightDecision = "warn"
)

// PreflightRequest describes a candidate action for inspection.
// Mirrors AuditTarget intentionally so the same struct can travel
// from a tool call site through preflight, action, and audit.
type PreflightRequest struct {
	// Verb is the action being attempted: "create", "delete", "patch".
	Verb string

	// ResourceKind is the K8s kind, e.g. "Pod", "Deployment".
	ResourceKind string

	// ResourceName is the name of the resource. May be empty for
	// list/create-from-yaml flows where the name is not yet known.
	ResourceName string

	// Namespace scopes the action.
	Namespace string

	// Metadata carries free-form context (e.g. labels, owner refs)
	// that custom checks can consult.
	Metadata map[string]interface{}
}

// PreflightResult bundles the verdict with a human-readable explanation
// and an optional list of advisory notes (for warn-level issues that
// do not block).
type PreflightResult struct {
	Decision PreflightDecision `json:"decision"`
	Reason   string            `json:"reason,omitempty"`
	Warnings []string          `json:"warnings,omitempty"`
}

// PreflightCheck is a single Guide. Implementations should be cheap,
// deterministic, and side-effect-free — they run on the hot path
// before every potentially-mutating tool call.
type PreflightCheck interface {
	// Name identifies this check in audit logs and error messages.
	Name() string

	// Check inspects the request and returns a verdict. An error means
	// the check itself could not run (e.g. K8s API unreachable); the
	// caller decides whether to treat that as block or allow per
	// fail-closed/fail-open policy.
	Check(ctx context.Context, req PreflightRequest) (*PreflightResult, error)
}

// PreflightChain runs checks in registration order and short-circuits
// on the first Block. Warns accumulate across all checks.
//
// Use a chain to compose orthogonal concerns: namespace-allowlist,
// resource-existence, RBAC-feasibility, etc., each as a separate check.
type PreflightChain struct {
	checks []PreflightCheck

	// FailClosed controls behaviour when a check itself errors.
	// true  -> treat error as Block (safer default for write paths).
	// false -> treat error as Allow with a warning (use when checks
	//          depend on a flaky external service).
	FailClosed bool
}

// NewPreflightChain creates an empty chain. Add checks via Add.
// Defaults to fail-closed because Guides exist to prevent harm.
func NewPreflightChain() *PreflightChain {
	return &PreflightChain{FailClosed: true}
}

// Add appends a check. Order matters: cheaper / more decisive checks
// should come first to minimise wasted work on rejected requests.
func (c *PreflightChain) Add(check PreflightCheck) *PreflightChain {
	if check != nil {
		c.checks = append(c.checks, check)
	}
	return c
}

// Run executes the chain. Returns the first Block, otherwise an Allow
// (possibly with accumulated warnings).
func (c *PreflightChain) Run(ctx context.Context, req PreflightRequest) *PreflightResult {
	warnings := make([]string, 0)
	for _, check := range c.checks {
		res, err := check.Check(ctx, req)
		if err != nil {
			if c.FailClosed {
				return &PreflightResult{
					Decision: PreflightBlock,
					Reason:   fmt.Sprintf("preflight %q errored (fail-closed): %v", check.Name(), err),
					Warnings: warnings,
				}
			}
			warnings = append(warnings, fmt.Sprintf("preflight %q skipped (fail-open): %v", check.Name(), err))
			continue
		}
		if res == nil {
			continue
		}
		if res.Decision == PreflightBlock {
			res.Warnings = append(warnings, res.Warnings...)
			return res
		}
		warnings = append(warnings, res.Warnings...)
	}
	return &PreflightResult{Decision: PreflightAllow, Warnings: warnings}
}

// --- Built-in checks --------------------------------------------------

// ProtectedNamespaceCheck blocks any write into a namespace marked
// "protected" (e.g. kube-system, monitoring). The intent is to make
// agents conservative by default, even if the LLM's plan is correct.
// Operators opt resources in by passing an explicit allowlist, never
// by adding more namespaces to the protected list at call time.
type ProtectedNamespaceCheck struct {
	// Protected is the set of namespace names to block writes against.
	Protected map[string]struct{}
}

// NewProtectedNamespaceCheck builds a check from a list of namespace
// names. A nil/empty list means the check is a no-op allow.
func NewProtectedNamespaceCheck(namespaces ...string) *ProtectedNamespaceCheck {
	set := make(map[string]struct{}, len(namespaces))
	for _, n := range namespaces {
		set[strings.ToLower(n)] = struct{}{}
	}
	return &ProtectedNamespaceCheck{Protected: set}
}

// Name implements PreflightCheck.
func (p *ProtectedNamespaceCheck) Name() string { return "protected-namespace" }

// Check blocks writes to any namespace in the protected set.
func (p *ProtectedNamespaceCheck) Check(_ context.Context, req PreflightRequest) (*PreflightResult, error) {
	if !isMutating(req.Verb) {
		return &PreflightResult{Decision: PreflightAllow}, nil
	}
	ns := strings.ToLower(req.Namespace)
	if ns == "" {
		// Empty namespace defaults to "default" which is not protected,
		// so we allow it without error.
		return &PreflightResult{Decision: PreflightAllow}, nil
	}
	if _, blocked := p.Protected[ns]; blocked {
		return &PreflightResult{
			Decision: PreflightBlock,
			Reason: fmt.Sprintf("namespace %q is protected against %s by policy",
				req.Namespace, req.Verb),
		}, nil
	}
	return &PreflightResult{Decision: PreflightAllow}, nil
}

// ResourceExistsCheck verifies that a target resource actually exists
// before a delete/patch is attempted. Catches typos and stale state.
//
// For "create" verbs it inverts the polarity: existence becomes a
// blocker (avoids "AlreadyExists" 409s), with a friendlier message.
type ResourceExistsCheck struct {
	client *k8s.Client
}

// NewResourceExistsCheck wires a k8s client. Pass nil to disable
// (the chain will simply allow when this check is omitted).
func NewResourceExistsCheck(client *k8s.Client) *ResourceExistsCheck {
	return &ResourceExistsCheck{client: client}
}

// Name implements PreflightCheck.
func (r *ResourceExistsCheck) Name() string { return "resource-exists" }

// Check enforces the existence invariant matching the verb.
func (r *ResourceExistsCheck) Check(_ context.Context, req PreflightRequest) (*PreflightResult, error) {
	if r.client == nil {
		return &PreflightResult{Decision: PreflightAllow}, nil
	}
	if req.ResourceKind == "" || req.ResourceName == "" {
		// Nothing concrete to look up — skip without blocking.
		return &PreflightResult{Decision: PreflightAllow}, nil
	}
	state, err := r.client.GetResourceState(strings.ToLower(req.ResourceKind), req.ResourceName, req.Namespace)
	if err != nil {
		return nil, err
	}
	switch strings.ToLower(req.Verb) {
	case "delete", "patch", "update", "scale":
		if !state.Exists {
			return &PreflightResult{
				Decision: PreflightBlock,
				Reason: fmt.Sprintf("%s %s/%s not found in namespace %q (cannot %s)",
					req.ResourceKind, req.ResourceKind, req.ResourceName, req.Namespace, req.Verb),
			}, nil
		}
	case "create":
		if state.Exists {
			return &PreflightResult{
				Decision: PreflightBlock,
				Reason: fmt.Sprintf("%s %s/%s already exists in namespace %q",
					req.ResourceKind, req.ResourceKind, req.ResourceName, req.Namespace),
			}, nil
		}
	}
	return &PreflightResult{Decision: PreflightAllow}, nil
}

// isMutating returns true for verbs that change cluster state. Used by
// checks that are only relevant to writes.
func isMutating(verb string) bool {
	switch strings.ToLower(verb) {
	case "create", "delete", "update", "patch", "scale", "apply":
		return true
	}
	return false
}
