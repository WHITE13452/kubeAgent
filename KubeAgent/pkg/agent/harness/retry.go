package harness

import (
	"context"
	"errors"
	"fmt"
	"math/rand"
	"time"
)

// RetryPolicy is a transparent, dependency-free retry strategy. It is
// not a circuit breaker; it does not track failure rates across calls.
// Use it to wrap a single operation that might transiently fail
// (LLM API call, K8s API call, etc.).
//
// Why hand-rolled instead of pulling in cenkalti/backoff or k8s.io/client-go's
// wait.Backoff:
//   - The agent has no other dep on those packages and the policy is small.
//   - We want jitter on by default and a clean ctx-aware loop.
//   - Keeping it local makes it trivial to audit during code review.
type RetryPolicy struct {
	// MaxAttempts is the total number of attempts (NOT additional retries).
	// MaxAttempts=1 means "try once, never retry". Defaults to 3 when <=0.
	MaxAttempts int

	// InitialBackoff is the wait before the second attempt. Defaults to
	// 500ms when zero.
	InitialBackoff time.Duration

	// MaxBackoff caps the exponential growth. Defaults to 10s when zero.
	MaxBackoff time.Duration

	// Multiplier controls exponential growth. Defaults to 2.0 when <=1.
	Multiplier float64

	// Jitter adds up to ±Jitter*currentBackoff random noise to each
	// wait. 0 disables jitter; 0.2 (20%) is a good default to avoid
	// thundering herds. Defaults to 0.2 when negative.
	Jitter float64
}

// DefaultRetryPolicy returns a sane policy for transient external calls.
// Values chosen for human-paced operations: 3 attempts, ~3.5s worst case.
func DefaultRetryPolicy() RetryPolicy {
	return RetryPolicy{
		MaxAttempts:    3,
		InitialBackoff: 500 * time.Millisecond,
		MaxBackoff:     10 * time.Second,
		Multiplier:     2.0,
		Jitter:         0.2,
	}
}

// Permanent wraps an error to signal "do not retry this". The retry
// loop unwraps and returns it immediately. Use this for known-fatal
// conditions like 4xx auth errors.
type Permanent struct{ Err error }

// Error implements error.
func (p *Permanent) Error() string {
	if p == nil || p.Err == nil {
		return "permanent error"
	}
	return "permanent: " + p.Err.Error()
}

// Unwrap allows errors.Is / errors.As to peer through.
func (p *Permanent) Unwrap() error { return p.Err }

// PermanentError is a convenience constructor.
func PermanentError(err error) error {
	if err == nil {
		return nil
	}
	return &Permanent{Err: err}
}

// Do runs op under the policy. It returns the last error from op when
// all attempts fail, or context cancellation if ctx fires first.
//
// op should be idempotent: this loop will call it up to MaxAttempts
// times if it keeps returning non-permanent errors. The intended caller
// is single K8s API or LLM HTTP request, both of which are safe to
// re-issue.
func (p RetryPolicy) Do(ctx context.Context, op func(ctx context.Context) error) error {
	policy := p.normalised()

	var lastErr error
	backoff := policy.InitialBackoff

	for attempt := 1; attempt <= policy.MaxAttempts; attempt++ {
		if err := ctx.Err(); err != nil {
			return err
		}

		err := op(ctx)
		if err == nil {
			return nil
		}

		// Don't retry if the caller marked the error fatal.
		var perm *Permanent
		if errors.As(err, &perm) {
			if perm.Err == nil {
				return errors.New("permanent error")
			}
			return perm.Err
		}

		lastErr = err

		// No more attempts? bail.
		if attempt == policy.MaxAttempts {
			break
		}

		// Sleep with jitter before the next attempt.
		wait := jittered(backoff, policy.Jitter)
		select {
		case <-ctx.Done():
			return ctx.Err()
		case <-time.After(wait):
		}

		backoff = nextBackoff(backoff, policy.Multiplier, policy.MaxBackoff)
	}

	return fmt.Errorf("retry: %d attempts exhausted: %w", policy.MaxAttempts, lastErr)
}

// normalised fills defaults for any zero/invalid fields. Called every
// Do() because RetryPolicy values may be constructed inline by callers.
func (p RetryPolicy) normalised() RetryPolicy {
	if p.MaxAttempts <= 0 {
		p.MaxAttempts = 3
	}
	if p.InitialBackoff <= 0 {
		p.InitialBackoff = 500 * time.Millisecond
	}
	if p.MaxBackoff <= 0 {
		p.MaxBackoff = 10 * time.Second
	}
	if p.Multiplier <= 1 {
		p.Multiplier = 2.0
	}
	if p.Jitter < 0 {
		p.Jitter = 0.2
	}
	return p
}

func nextBackoff(current time.Duration, mul float64, max time.Duration) time.Duration {
	next := time.Duration(float64(current) * mul)
	if next > max {
		next = max
	}
	return next
}

// jittered adds ±jitter fraction noise. Capped at 0..2x current to keep
// the worst case bounded. Uses math/rand because we don't need crypto
// randomness for backoff.
func jittered(d time.Duration, jitter float64) time.Duration {
	if jitter <= 0 {
		return d
	}
	// rand.Float64() ∈ [0,1); shift to [-1,1).
	delta := (rand.Float64()*2 - 1) * jitter
	scale := 1 + delta
	if scale < 0 {
		scale = 0
	}
	return time.Duration(float64(d) * scale)
}
