package harness

import (
	"bytes"
	"context"
	"encoding/json"
	"strings"
	"sync"
	"testing"
	"time"
)

// TestJSONLogAuditor_WritesOneLinePerEvent verifies the jsonl invariant
// that downstream tooling depends on: each Record produces exactly one
// newline-terminated JSON object.
func TestJSONLogAuditor_WritesOneLinePerEvent(t *testing.T) {
	var buf bytes.Buffer
	a := NewJSONLogAuditor(&buf)

	events := []AuditEvent{
		{Kind: AuditAction, Actor: "remediator", Action: "delete", Outcome: "success"},
		{Kind: AuditVerification, Actor: "remediator", Action: "verify", Outcome: "passed"},
	}
	for _, e := range events {
		if err := a.Record(context.Background(), e); err != nil {
			t.Fatalf("record: %v", err)
		}
	}

	lines := strings.Split(strings.TrimRight(buf.String(), "\n"), "\n")
	if len(lines) != len(events) {
		t.Fatalf("expected %d lines, got %d (raw=%q)", len(events), len(lines), buf.String())
	}
	for i, line := range lines {
		var got AuditEvent
		if err := json.Unmarshal([]byte(line), &got); err != nil {
			t.Fatalf("line %d not valid json: %v", i, err)
		}
		if got.Actor != events[i].Actor {
			t.Fatalf("line %d: actor mismatch: %q vs %q", i, got.Actor, events[i].Actor)
		}
	}
}

// TestJSONLogAuditor_StampsTimestamp ensures Record fills in a default
// Timestamp when the caller omits it. Operators rely on this so they
// don't have to remember to populate it at every call site.
func TestJSONLogAuditor_StampsTimestamp(t *testing.T) {
	var buf bytes.Buffer
	a := NewJSONLogAuditor(&buf)

	before := time.Now()
	_ = a.Record(context.Background(), AuditEvent{Kind: AuditAction, Actor: "x"})
	after := time.Now()

	var got AuditEvent
	if err := json.Unmarshal([]byte(strings.TrimSpace(buf.String())), &got); err != nil {
		t.Fatalf("decode: %v", err)
	}
	if got.Timestamp.Before(before) || got.Timestamp.After(after) {
		t.Fatalf("auto-timestamp out of bounds: %v not in [%v,%v]",
			got.Timestamp, before, after)
	}
}

// TestJSONLogAuditor_ConcurrentSafe stresses the mutex: many goroutines
// recording at once must not interleave bytes within a single line.
// Failure mode would be a malformed json line.
func TestJSONLogAuditor_ConcurrentSafe(t *testing.T) {
	var buf bytes.Buffer
	a := NewJSONLogAuditor(&buf)

	const writers = 16
	const events = 100

	var wg sync.WaitGroup
	wg.Add(writers)
	for w := 0; w < writers; w++ {
		go func(id int) {
			defer wg.Done()
			for i := 0; i < events; i++ {
				_ = a.Record(context.Background(), AuditEvent{
					Kind:    AuditAction,
					Actor:   "load-test",
					Action:  "noop",
					Outcome: "success",
				})
			}
		}(w)
	}
	wg.Wait()

	lines := strings.Split(strings.TrimRight(buf.String(), "\n"), "\n")
	if len(lines) != writers*events {
		t.Fatalf("expected %d lines, got %d", writers*events, len(lines))
	}
	for i, line := range lines {
		var e AuditEvent
		if err := json.Unmarshal([]byte(line), &e); err != nil {
			t.Fatalf("line %d not valid json (%q): %v", i, line, err)
		}
	}
}

// TestNoopAuditor is trivial but documents the contract: NoopAuditor
// never errors and never writes. We assert "no error" because callers
// switch on it without nil-checking.
func TestNoopAuditor(t *testing.T) {
	if err := (NoopAuditor{}).Record(context.Background(), AuditEvent{}); err != nil {
		t.Fatalf("noop should never error; got %v", err)
	}
}
