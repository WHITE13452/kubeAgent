package harness

import (
	"context"
	"errors"
	"testing"
)

// stubCheck is a hand-written PreflightCheck used for chain composition
// tests. We avoid a mocking library because the surface is one method
// and explicit code keeps tests readable.
type stubCheck struct {
	name   string
	result *PreflightResult
	err    error
	calls  int
}

func (s *stubCheck) Name() string { return s.name }
func (s *stubCheck) Check(_ context.Context, _ PreflightRequest) (*PreflightResult, error) {
	s.calls++
	return s.result, s.err
}

// TestPreflightChain_AllowsWhenEmpty: a chain with no checks is a
// permissive default. This matters because callers may build chains
// dynamically and end up with zero checks; that should not block work.
func TestPreflightChain_AllowsWhenEmpty(t *testing.T) {
	chain := NewPreflightChain()
	res := chain.Run(context.Background(), PreflightRequest{Verb: "create"})
	if res.Decision != PreflightAllow {
		t.Fatalf("expected allow, got %v", res.Decision)
	}
}

// TestPreflightChain_BlocksFirstBlock: chain short-circuits on the
// first block, so we don't waste work on subsequent (possibly slow)
// checks once the verdict is sealed.
func TestPreflightChain_BlocksFirstBlock(t *testing.T) {
	blocker := &stubCheck{name: "first", result: &PreflightResult{Decision: PreflightBlock, Reason: "no"}}
	downstream := &stubCheck{name: "second", result: &PreflightResult{Decision: PreflightAllow}}

	chain := NewPreflightChain().Add(blocker).Add(downstream)
	res := chain.Run(context.Background(), PreflightRequest{Verb: "create"})

	if res.Decision != PreflightBlock {
		t.Fatalf("expected block, got %v", res.Decision)
	}
	if downstream.calls != 0 {
		t.Fatalf("downstream check must not run after block; got %d calls", downstream.calls)
	}
}

// TestPreflightChain_FailClosed converts check errors into Block when
// FailClosed is true (the default for write paths).
func TestPreflightChain_FailClosed(t *testing.T) {
	chain := NewPreflightChain().
		Add(&stubCheck{name: "broken", err: errors.New("boom")})

	res := chain.Run(context.Background(), PreflightRequest{Verb: "delete"})
	if res.Decision != PreflightBlock {
		t.Fatalf("fail-closed should block on error; got %v", res.Decision)
	}
}

// TestPreflightChain_FailOpen lets erroring checks pass through, but
// adds a warning so the issue is visible.
func TestPreflightChain_FailOpen(t *testing.T) {
	chain := NewPreflightChain()
	chain.FailClosed = false
	chain.Add(&stubCheck{name: "broken", err: errors.New("boom")})

	res := chain.Run(context.Background(), PreflightRequest{Verb: "delete"})
	if res.Decision != PreflightAllow {
		t.Fatalf("fail-open should allow on error; got %v", res.Decision)
	}
	if len(res.Warnings) == 0 {
		t.Fatal("fail-open allow must produce a warning")
	}
}

// TestProtectedNamespaceCheck verifies the policy of "writes to known
// system namespaces are blocked, reads pass". This is the most common
// safety net and worth a focused test.
func TestProtectedNamespaceCheck(t *testing.T) {
	check := NewProtectedNamespaceCheck("kube-system", "monitoring")

	// Read verb against protected ns: allowed.
	res, err := check.Check(context.Background(), PreflightRequest{
		Verb: "get", Namespace: "kube-system",
	})
	if err != nil {
		t.Fatalf("unexpected err: %v", err)
	}
	if res.Decision != PreflightAllow {
		t.Fatalf("read should be allowed; got %v", res.Decision)
	}

	// Write to protected ns: blocked.
	res, err = check.Check(context.Background(), PreflightRequest{
		Verb: "delete", Namespace: "kube-system",
	})
	if err != nil {
		t.Fatalf("unexpected err: %v", err)
	}
	if res.Decision != PreflightBlock {
		t.Fatalf("write to protected ns should be blocked; got %v", res.Decision)
	}

	// Write to non-protected ns: allowed.
	res, _ = check.Check(context.Background(), PreflightRequest{
		Verb: "delete", Namespace: "default",
	})
	if res.Decision != PreflightAllow {
		t.Fatalf("write to default should be allowed; got %v", res.Decision)
	}

	// Case-insensitive: configured "kube-system", request "KUBE-SYSTEM".
	res, _ = check.Check(context.Background(), PreflightRequest{
		Verb: "delete", Namespace: "KUBE-SYSTEM",
	})
	if res.Decision != PreflightBlock {
		t.Fatalf("namespace match must be case-insensitive; got %v", res.Decision)
	}
}
