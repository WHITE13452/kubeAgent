// Package harness implements the Sensors + Guides framework that wraps
// agent execution. Following Martin Fowler's harness model, this package
// provides:
//
//   - Sensors  (feedback control): observe what happened after the agent
//     acted, so the system can self-correct rather than fire-and-forget.
//   - Guides   (feedforward control): rules and checks applied before the
//     agent acts, so the first attempt is more likely to be correct.
//
// Verifier is the Sensor responsible for closing the loop after a
// Remediator (or any write-capable agent) finishes a task: did the action
// actually produce the desired state, or do we need to retry / escalate?
package harness

import (
	"context"
	"time"
)

// VerificationStatus represents the outcome of a post-action verification.
type VerificationStatus string

const (
	// VerificationPassed means the verifier observed the expected state.
	VerificationPassed VerificationStatus = "passed"

	// VerificationFailed means the verifier observed a state that
	// contradicts the expected outcome (e.g. pod still CrashLoopBackOff).
	VerificationFailed VerificationStatus = "failed"

	// VerificationInconclusive means the verifier could not determine
	// the outcome (e.g. resource not found, timeout, transient API error).
	// Callers should treat this as "unknown" and decide policy themselves.
	VerificationInconclusive VerificationStatus = "inconclusive"
)

// VerificationTarget describes the resource(s) and expected state the
// verifier should check after an action runs.
//
// All fields are optional; the verifier picks the most specific identifier
// available. A zero-value target produces an Inconclusive result.
type VerificationTarget struct {
	// ResourceKind is the K8s kind, e.g. "Pod", "Deployment", "Service".
	ResourceKind string

	// ResourceName is the name of a single resource. If empty, the verifier
	// may use Selector to match a set.
	ResourceName string

	// Namespace scopes the verification. Empty means "default".
	Namespace string

	// Selector is an optional label selector used when ResourceName is
	// empty (e.g. all pods of a Deployment).
	Selector string

	// ExpectedPhase, if set, is the desired phase/condition. For Pods this
	// is typically "Running"; for Deployments, "Available".
	ExpectedPhase string

	// SettleTimeout caps how long the verifier waits for the resource to
	// converge to ExpectedPhase. Defaults to 30s when zero.
	SettleTimeout time.Duration

	// PollInterval controls how often the verifier samples state.
	// Defaults to 3s when zero.
	PollInterval time.Duration
}

// VerificationResult is what a Verifier returns to the caller. It is
// machine-readable (Status, Observations) and carries a human-readable
// Summary suitable for surfacing to the user or feeding back into the LLM.
type VerificationResult struct {
	Status       VerificationStatus     `json:"status"`
	Summary      string                 `json:"summary"`
	Observations map[string]interface{} `json:"observations,omitempty"`
	Duration     time.Duration          `json:"duration"`
	CheckedAt    time.Time              `json:"checked_at"`
}

// Verifier is the Sensor abstraction. Implementations close the loop on
// agent actions by re-observing cluster state and reporting whether the
// intended outcome was achieved.
//
// Verifier is intentionally narrow: it only answers "did the change land
// and converge". Diagnosing why a verification failed is the
// Diagnostician's job; deciding whether to retry is the Remediator /
// Coordinator's job.
type Verifier interface {
	Verify(ctx context.Context, target VerificationTarget) (*VerificationResult, error)
}

// NoopVerifier always reports VerificationInconclusive. It exists so that
// callers can opt out of verification (e.g. in unit tests, or when no
// cluster client is wired in) without sprinkling nil-checks everywhere.
type NoopVerifier struct{}

// Verify implements Verifier. It returns an Inconclusive result so callers
// who treat Inconclusive as "skip" will simply move on.
func (NoopVerifier) Verify(_ context.Context, _ VerificationTarget) (*VerificationResult, error) {
	return &VerificationResult{
		Status:    VerificationInconclusive,
		Summary:   "verification skipped: no verifier configured",
		CheckedAt: time.Now(),
	}, nil
}
