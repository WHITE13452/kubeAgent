package harness

import (
	"context"
	"fmt"
	"strings"
	"time"

	"kubeagent/pkg/k8s"
)

// K8sVerifier is the production Sensor. It polls cluster state via the
// k8s client and decides whether a target resource has converged to its
// expected phase within a bounded time window.
//
// Design notes:
//   - Verification runs in a poll loop (not a watch) because the call
//     volume is low (one verification per remediation), and polling keeps
//     the implementation independent of the dynamic informer machinery.
//   - "Inconclusive" is a first-class outcome. We never invent a
//     pass/fail result we are not certain of: e.g. when the resource kind
//     is unknown to deriveReadiness, we return Inconclusive so callers
//     can choose to escalate to a human or to an LLM-based inspector.
type K8sVerifier struct {
	client *k8s.Client
}

// NewK8sVerifier wires a k8s client into a verifier.
// Pass nil to fall back to NoopVerifier semantics at the call site.
func NewK8sVerifier(client *k8s.Client) *K8sVerifier {
	return &K8sVerifier{client: client}
}

// Verify implements Verifier by polling the target resource's state.
// It returns when one of these is true:
//   - The resource matches the expected phase (Passed).
//   - The resource is in a clearly-bad terminal state, e.g.
//     CrashLoopBackOff (Failed). Terminal-bad reasons are detected by
//     isTerminalFailure; everything else keeps polling.
//   - SettleTimeout elapses without a verdict (Inconclusive).
//   - The kind is one the underlying state derivation cannot judge
//     (Inconclusive immediately).
//
// The returned error is reserved for "could not even attempt the check"
// situations (e.g. nil client, malformed target). A failed verification
// is NOT an error — it is a successful observation of a bad state.
func (v *K8sVerifier) Verify(ctx context.Context, target VerificationTarget) (*VerificationResult, error) {
	start := time.Now()

	if v.client == nil {
		return nil, fmt.Errorf("k8s verifier: client is nil")
	}
	if target.ResourceKind == "" || target.ResourceName == "" {
		// Without a concrete target we can't poll. We refuse to guess.
		return &VerificationResult{
			Status:    VerificationInconclusive,
			Summary:   "verifier: target missing kind or name",
			CheckedAt: time.Now(),
		}, nil
	}

	timeout := target.SettleTimeout
	if timeout <= 0 {
		timeout = 30 * time.Second
	}
	interval := target.PollInterval
	if interval <= 0 {
		interval = 3 * time.Second
	}
	namespace := target.Namespace
	if namespace == "" {
		namespace = "default"
	}

	deadline := time.Now().Add(timeout)
	resourceArg := strings.ToLower(target.ResourceKind)

	// We only want to call the API on a tick boundary, but we also want
	// to short-circuit on ctx cancellation. ticker + select does both.
	ticker := time.NewTicker(interval)
	defer ticker.Stop()

	var lastState *k8s.ResourceState
	for {
		state, err := v.client.GetResourceState(resourceArg, target.ResourceName, namespace)
		if err != nil {
			// Treat API errors as transient; surface them only if we
			// time out without ever getting a clean read.
			lastState = nil
			_ = err
		} else {
			lastState = state

			// Special case: if the caller wanted "Deleted" and the
			// resource is absent, we are done.
			if target.ExpectedPhase == "Deleted" && !state.Exists {
				return passed(start, "resource deleted as expected", state), nil
			}

			if state.Exists {
				// Hard-fail fast on terminal error reasons so we don't
				// burn the full timeout on a pod that will never recover.
				if isTerminalFailure(state.Reason) {
					return failed(start,
						fmt.Sprintf("resource %s/%s in terminal failure state: %s",
							state.Kind, state.Name, state.Reason),
						state), nil
				}

				if matchesExpected(state, target.ExpectedPhase) {
					return passed(start,
						fmt.Sprintf("resource %s/%s reached expected phase %q",
							state.Kind, state.Name, effectivePhase(target)),
						state), nil
				}
			}
		}

		// Decide whether to poll again or give up.
		if time.Now().After(deadline) {
			return inconclusiveOrFailed(start, target, lastState), nil
		}

		select {
		case <-ctx.Done():
			return inconclusiveOrFailed(start, target, lastState), ctx.Err()
		case <-ticker.C:
			// continue polling
		}
	}
}

// matchesExpected returns true when the live state matches what the
// caller asked for. An empty ExpectedPhase means "just be Ready".
func matchesExpected(state *k8s.ResourceState, expected string) bool {
	if expected == "" {
		return state.Ready
	}
	if strings.EqualFold(expected, state.Phase) {
		return true
	}
	if strings.EqualFold(expected, "Ready") && state.Ready {
		return true
	}
	if strings.EqualFold(expected, "Available") && state.Ready {
		return true
	}
	return false
}

// isTerminalFailure flags reasons we should never wait out: the pod is
// looping or stuck pulling and more time will not help.
func isTerminalFailure(reason string) bool {
	switch reason {
	case "CrashLoopBackOff", "ImagePullBackOff", "ErrImagePull",
		"CreateContainerConfigError", "InvalidImageName":
		return true
	}
	return false
}

func effectivePhase(t VerificationTarget) string {
	if t.ExpectedPhase != "" {
		return t.ExpectedPhase
	}
	return "Ready"
}

func passed(start time.Time, summary string, state *k8s.ResourceState) *VerificationResult {
	return &VerificationResult{
		Status:       VerificationPassed,
		Summary:      summary,
		Observations: stateToObservations(state),
		Duration:     time.Since(start),
		CheckedAt:    time.Now(),
	}
}

func failed(start time.Time, summary string, state *k8s.ResourceState) *VerificationResult {
	return &VerificationResult{
		Status:       VerificationFailed,
		Summary:      summary,
		Observations: stateToObservations(state),
		Duration:     time.Since(start),
		CheckedAt:    time.Now(),
	}
}

// inconclusiveOrFailed picks the right verdict when the timeout fires.
// If we never managed to read state, that's Inconclusive (we don't know).
// If we read state and it just hadn't converged, that's Failed (we know
// it didn't reach the expected phase in the allotted time).
func inconclusiveOrFailed(start time.Time, target VerificationTarget, lastState *k8s.ResourceState) *VerificationResult {
	if lastState == nil {
		return &VerificationResult{
			Status:    VerificationInconclusive,
			Summary:   "verifier: could not read resource state before timeout",
			Duration:  time.Since(start),
			CheckedAt: time.Now(),
		}
	}
	return failed(start,
		fmt.Sprintf("resource %s/%s did not reach phase %q within %s (last phase=%q reason=%q)",
			lastState.Kind, lastState.Name, effectivePhase(target),
			target.SettleTimeout, lastState.Phase, lastState.Reason),
		lastState)
}

func stateToObservations(state *k8s.ResourceState) map[string]interface{} {
	if state == nil {
		return nil
	}
	return map[string]interface{}{
		"kind":      state.Kind,
		"name":      state.Name,
		"namespace": state.Namespace,
		"exists":    state.Exists,
		"phase":     state.Phase,
		"reason":    state.Reason,
		"ready":     state.Ready,
	}
}
