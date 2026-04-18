package harness

import (
	"context"
	"testing"

	"kubeagent/pkg/k8s"
)

// TestNoopVerifier_ReturnsInconclusive documents the expected-by-callers
// behaviour: a missing verifier never claims success.
func TestNoopVerifier_ReturnsInconclusive(t *testing.T) {
	res, err := NoopVerifier{}.Verify(context.Background(), VerificationTarget{
		ResourceKind: "Pod", ResourceName: "x",
	})
	if err != nil {
		t.Fatalf("unexpected err: %v", err)
	}
	if res.Status != VerificationInconclusive {
		t.Fatalf("expected Inconclusive, got %v", res.Status)
	}
}

// TestK8sVerifier_RejectsNilClient guards against silent misconfiguration:
// a verifier with no client must error rather than report Inconclusive,
// because it is a programmer mistake, not an observation.
func TestK8sVerifier_RejectsNilClient(t *testing.T) {
	v := NewK8sVerifier(nil)
	_, err := v.Verify(context.Background(), VerificationTarget{
		ResourceKind: "Pod", ResourceName: "x",
	})
	if err == nil {
		t.Fatal("expected error from nil client; got nil")
	}
}

// TestK8sVerifier_MissingTargetIsInconclusive: when the caller forgets
// to populate the target we should refuse to guess.
func TestK8sVerifier_MissingTargetIsInconclusive(t *testing.T) {
	// Pass a non-nil but empty client; we rely on the early target
	// validation, which runs before any API call.
	v := &K8sVerifier{client: &k8s.Client{}}
	res, err := v.Verify(context.Background(), VerificationTarget{})
	if err != nil {
		t.Fatalf("unexpected err: %v", err)
	}
	if res.Status != VerificationInconclusive {
		t.Fatalf("expected Inconclusive, got %v", res.Status)
	}
}

// TestMatchesExpected covers the small but nuanced phase-matching rules
// in one focused test: empty means "be ready", "Available" maps to
// Ready, case-insensitive Phase compare.
func TestMatchesExpected(t *testing.T) {
	cases := []struct {
		name     string
		state    *k8s.ResourceState
		expected string
		want     bool
	}{
		{"empty matches Ready", &k8s.ResourceState{Ready: true}, "", true},
		{"empty does not match not-ready", &k8s.ResourceState{Ready: false}, "", false},
		{"phase matches case-insensitive", &k8s.ResourceState{Phase: "Running"}, "running", true},
		{"Available alias for Ready", &k8s.ResourceState{Ready: true}, "Available", true},
		{"unknown phase does not match", &k8s.ResourceState{Phase: "Pending"}, "Running", false},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			if got := matchesExpected(c.state, c.expected); got != c.want {
				t.Fatalf("want %v, got %v", c.want, got)
			}
		})
	}
}

// TestIsTerminalFailure pins down which container-state reasons we
// consider hopeless. Adding new entries here is a deliberate policy
// change and should require touching this test.
func TestIsTerminalFailure(t *testing.T) {
	terminal := []string{
		"CrashLoopBackOff", "ImagePullBackOff", "ErrImagePull",
		"CreateContainerConfigError", "InvalidImageName",
	}
	for _, r := range terminal {
		if !isTerminalFailure(r) {
			t.Fatalf("%q should be terminal", r)
		}
	}
	if isTerminalFailure("ContainerCreating") {
		t.Fatal("ContainerCreating is transient, not terminal")
	}
	if isTerminalFailure("") {
		t.Fatal("empty reason is not terminal")
	}
}
