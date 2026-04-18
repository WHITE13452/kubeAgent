package harness

import (
	"bytes"
	"context"
	"errors"
	"strings"
	"testing"
)

// TestConsoleReporter_RendersHeaderAndBody covers the basic shape:
// each event becomes a header line with tag/actor/outcome plus an
// indented body when the event carries target/details. We check
// substrings rather than exact format so future cosmetic tweaks
// don't break the test.
func TestConsoleReporter_RendersHeaderAndBody(t *testing.T) {
	var buf bytes.Buffer
	r := NewConsoleReporter(&buf).DisableColor()

	err := r.Record(context.Background(), AuditEvent{
		Kind:    AuditVerification,
		Actor:   "remediator",
		Action:  "post_action_verify",
		Outcome: "passed",
		Target:  AuditTarget{Kind: "Pod", Name: "nginx-1", Namespace: "default"},
		Details: map[string]interface{}{
			"phase": "Running",
			"ready": true,
		},
	})
	if err != nil {
		t.Fatalf("record: %v", err)
	}

	got := buf.String()
	mustContain(t, got, "[SENSOR]", "remediator", "post_action_verify", "passed",
		"target: Pod/nginx-1 in default", "phase: Running")
}

// TestConsoleReporter_GuideBlockTag checks that a Block outcome flips
// the GUIDE tag to the visually loud "GUIDE!" variant. This matters
// because in a demo the operator must instantly see when a guard
// rejected an action.
func TestConsoleReporter_GuideBlockTag(t *testing.T) {
	var buf bytes.Buffer
	r := NewConsoleReporter(&buf).DisableColor()

	_ = r.Record(context.Background(), AuditEvent{
		Kind:    AuditPreflight,
		Actor:   "remediator",
		Action:  "delete pod",
		Outcome: "block",
		Reason:  "namespace kube-system is protected",
	})
	got := buf.String()
	mustContain(t, got, "[GUIDE!]", "block", "namespace kube-system is protected")
}

// TestConsoleReporter_NoColorWhenDisabled guards against ANSI escapes
// leaking into captured output (CI logs, demo recordings).
func TestConsoleReporter_NoColorWhenDisabled(t *testing.T) {
	var buf bytes.Buffer
	r := NewConsoleReporter(&buf).DisableColor()

	_ = r.Record(context.Background(), AuditEvent{
		Kind: AuditAction, Actor: "x", Action: "noop", Outcome: "success",
	})
	if strings.Contains(buf.String(), "\x1b[") {
		t.Fatalf("expected no ANSI escapes; got %q", buf.String())
	}
}

// TestTee_FansOutToAllSinks verifies the multi-sink contract: every
// sink sees every event, in the order Record was called.
func TestTee_FansOutToAllSinks(t *testing.T) {
	a := &recordingSink{}
	b := &recordingSink{}
	tee := NewTee(a, b, nil) // nil should be silently dropped

	for i := 0; i < 3; i++ {
		_ = tee.Record(context.Background(), AuditEvent{Action: "x"})
	}

	if len(a.events) != 3 || len(b.events) != 3 {
		t.Fatalf("expected 3 events per sink, got a=%d b=%d", len(a.events), len(b.events))
	}
}

// TestTee_ReturnsFirstErrorButContinues guarantees that one flaky
// sink does not stop other sinks from receiving the event. This is
// the contract callers depend on when one sink is a critical durable
// log and another is a best-effort console.
func TestTee_ReturnsFirstErrorButContinues(t *testing.T) {
	bad := &recordingSink{err: errors.New("disk full")}
	good := &recordingSink{}

	tee := NewTee(bad, good)
	err := tee.Record(context.Background(), AuditEvent{Action: "x"})

	if err == nil {
		t.Fatal("expected first sink error to surface")
	}
	if len(good.events) != 1 {
		t.Fatalf("good sink should still receive event despite bad one failing; got %d events", len(good.events))
	}
}

// recordingSink is a tiny test double: capture every event and
// optionally return a configured error from Record.
type recordingSink struct {
	events []AuditEvent
	err    error
}

func (s *recordingSink) Record(_ context.Context, e AuditEvent) error {
	s.events = append(s.events, e)
	return s.err
}

func mustContain(t *testing.T, haystack string, needles ...string) {
	t.Helper()
	for _, n := range needles {
		if !strings.Contains(haystack, n) {
			t.Fatalf("output missing %q\nfull output:\n%s", n, haystack)
		}
	}
}
