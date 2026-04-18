package tools

import (
	"context"
	"strings"
	"sync"
	"testing"

	"kubeagent/pkg/agent/harness"
)

// recordingAuditor captures the last event so tests can assert what
// the tool emitted without touching disk or fighting concurrency.
type recordingAuditor struct {
	mu     sync.Mutex
	events []harness.AuditEvent
}

func (r *recordingAuditor) Record(_ context.Context, e harness.AuditEvent) error {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.events = append(r.events, e)
	return nil
}

func (r *recordingAuditor) last() harness.AuditEvent {
	r.mu.Lock()
	defer r.mu.Unlock()
	if len(r.events) == 0 {
		return harness.AuditEvent{}
	}
	return r.events[len(r.events)-1]
}

// TestDeleteTool_PreflightBlocksProtectedNamespace proves the Guide
// short-circuits the real DeleteResource call. We pass a nil k8s
// client — if preflight ever failed to block, we'd see a nil-pointer
// panic from the client call, which is a loud signal of regression.
func TestDeleteTool_PreflightBlocksProtectedNamespace(t *testing.T) {
	chain := harness.NewPreflightChain().
		Add(harness.NewProtectedNamespaceCheck("kube-system"))

	audit := &recordingAuditor{}

	tool := NewDeleteTool(nil).
		WithPreflight(chain).
		WithAuditor(audit)

	out, err := tool.Execute(map[string]any{
		"resource":  "pod",
		"name":      "coredns-abc",
		"namespace": "kube-system",
	})
	if err == nil {
		t.Fatalf("expected error from preflight block, got output=%q", out)
	}
	if !strings.Contains(err.Error(), "preflight") {
		t.Fatalf("expected preflight-prefixed error, got %v", err)
	}

	ev := audit.last()
	if ev.Kind != harness.AuditPreflight {
		t.Fatalf("expected preflight audit event, got %+v", ev)
	}
	if ev.Outcome != string(harness.PreflightBlock) {
		t.Fatalf("expected block outcome, got %q", ev.Outcome)
	}
	if ev.Target.Namespace != "kube-system" {
		t.Fatalf("expected namespace on audit target, got %+v", ev.Target)
	}
}

// TestCreateTool_PreflightPeeksYAML proves the YAML peek surfaces the
// right namespace/kind to the chain, and a protected-namespace YAML
// is blocked without touching the real client.
func TestCreateTool_PreflightPeeksYAML(t *testing.T) {
	chain := harness.NewPreflightChain().
		Add(harness.NewProtectedNamespaceCheck("kube-system"))

	audit := &recordingAuditor{}

	tool := NewCreateTool(nil).
		WithPreflight(chain).
		WithAuditor(audit)

	yaml := `apiVersion: v1
kind: Pod
metadata:
  name: rogue
  namespace: kube-system
spec:
  containers:
  - name: c
    image: nginx
`

	_, err := tool.Execute(map[string]any{"yaml": yaml})
	if err == nil {
		t.Fatalf("expected preflight block for kube-system create")
	}
	if !strings.Contains(err.Error(), "preflight") {
		t.Fatalf("expected preflight error, got %v", err)
	}

	ev := audit.last()
	if ev.Target.Kind != "Pod" || ev.Target.Namespace != "kube-system" {
		t.Fatalf("audit target lost YAML peek info: %+v", ev.Target)
	}
}

// TestCreateTool_DecodeFailureSkipsPreflight documents the deliberate
// design: a malformed YAML should NOT be reported as a policy block.
// We prove this by using a protected-namespace check that would
// normally block — but with garbage input, the peek fails and the
// chain is bypassed, so the error comes from the nil client (because
// we didn't wire one).
func TestCreateTool_DecodeFailureSkipsPreflight(t *testing.T) {
	chain := harness.NewPreflightChain().
		Add(harness.NewProtectedNamespaceCheck("kube-system"))

	audit := &recordingAuditor{}

	tool := NewCreateTool(nil).
		WithPreflight(chain).
		WithAuditor(audit)

	defer func() {
		// Recover from the expected nil-client panic; the point of the
		// test is to observe that no preflight audit event was recorded.
		_ = recover()
		if len(audit.events) != 0 {
			t.Fatalf("expected zero preflight events on decode failure, got %d", len(audit.events))
		}
	}()

	_, _ = tool.Execute(map[string]any{"yaml": "not a valid yaml ::: {{{"})
}
