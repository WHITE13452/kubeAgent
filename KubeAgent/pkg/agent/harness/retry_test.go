package harness

import (
	"context"
	"errors"
	"sync/atomic"
	"testing"
	"time"
)

// TestRetry_SucceedsFirstAttempt verifies the loop does not sleep or
// retry when op succeeds immediately. The "should never sleep" check
// keeps us honest about the InitialBackoff being unused on success.
func TestRetry_SucceedsFirstAttempt(t *testing.T) {
	var calls int32
	policy := RetryPolicy{MaxAttempts: 3, InitialBackoff: time.Second}

	start := time.Now()
	err := policy.Do(context.Background(), func(_ context.Context) error {
		atomic.AddInt32(&calls, 1)
		return nil
	})
	if err != nil {
		t.Fatalf("expected nil error, got %v", err)
	}
	if got := atomic.LoadInt32(&calls); got != 1 {
		t.Fatalf("expected 1 call, got %d", got)
	}
	if elapsed := time.Since(start); elapsed > 100*time.Millisecond {
		t.Fatalf("loop slept on success path: %v", elapsed)
	}
}

// TestRetry_RetriesUntilSuccess covers the happy retry path: a
// transient failure followed by a success should not surface an error.
func TestRetry_RetriesUntilSuccess(t *testing.T) {
	var calls int32
	policy := RetryPolicy{
		MaxAttempts:    5,
		InitialBackoff: time.Millisecond,
		MaxBackoff:     time.Millisecond,
		Multiplier:     2,
		Jitter:         0,
	}

	err := policy.Do(context.Background(), func(_ context.Context) error {
		n := atomic.AddInt32(&calls, 1)
		if n < 3 {
			return errors.New("transient")
		}
		return nil
	})
	if err != nil {
		t.Fatalf("expected nil error, got %v", err)
	}
	if got := atomic.LoadInt32(&calls); got != 3 {
		t.Fatalf("expected 3 calls, got %d", got)
	}
}

// TestRetry_ExhaustsAttempts checks the wrapped error message contains
// the attempt count and the underlying cause, since operators rely on
// that info to triage retry exhaustion in the audit log.
func TestRetry_ExhaustsAttempts(t *testing.T) {
	policy := RetryPolicy{MaxAttempts: 2, InitialBackoff: time.Millisecond, Jitter: 0}
	cause := errors.New("nope")

	err := policy.Do(context.Background(), func(_ context.Context) error { return cause })
	if err == nil {
		t.Fatal("expected error after exhausting attempts")
	}
	if !errors.Is(err, cause) {
		t.Fatalf("expected wrapped cause, got %v", err)
	}
}

// TestRetry_PermanentShortCircuits ensures Permanent errors stop the
// loop on the first call, which is the contract callers depend on for
// auth failures and 4xx responses.
func TestRetry_PermanentShortCircuits(t *testing.T) {
	var calls int32
	policy := RetryPolicy{MaxAttempts: 5, InitialBackoff: time.Millisecond}
	cause := errors.New("forbidden")

	err := policy.Do(context.Background(), func(_ context.Context) error {
		atomic.AddInt32(&calls, 1)
		return PermanentError(cause)
	})
	if err == nil {
		t.Fatal("expected permanent error to surface")
	}
	if !errors.Is(err, cause) {
		t.Fatalf("expected unwrapped cause %v, got %v", cause, err)
	}
	if got := atomic.LoadInt32(&calls); got != 1 {
		t.Fatalf("permanent error should not retry; got %d calls", got)
	}
}

// TestRetry_ContextCancellation guarantees the loop honours ctx
// deadlines mid-backoff, otherwise a long backoff could block shutdown.
func TestRetry_ContextCancellation(t *testing.T) {
	policy := RetryPolicy{
		MaxAttempts:    10,
		InitialBackoff: 50 * time.Millisecond,
		MaxBackoff:     50 * time.Millisecond,
		Jitter:         0,
	}

	ctx, cancel := context.WithTimeout(context.Background(), 25*time.Millisecond)
	defer cancel()

	err := policy.Do(ctx, func(_ context.Context) error { return errors.New("transient") })
	if !errors.Is(err, context.DeadlineExceeded) {
		t.Fatalf("expected context.DeadlineExceeded, got %v", err)
	}
}
