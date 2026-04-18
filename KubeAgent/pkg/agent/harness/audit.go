package harness

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"sync"
	"time"
)

// AuditEventKind classifies what happened. Kept as a small closed set so
// downstream tooling (dashboards, alerting) can rely on the vocabulary.
type AuditEventKind string

const (
	// AuditPreflight is emitted by Guides before an action runs.
	AuditPreflight AuditEventKind = "preflight"

	// AuditAction is emitted when a Write tool actually executes.
	AuditAction AuditEventKind = "action"

	// AuditVerification is emitted by Sensors after an action runs.
	AuditVerification AuditEventKind = "verification"

	// AuditDecision is emitted when the agent decides to skip, retry,
	// or escalate based on harness signals.
	AuditDecision AuditEventKind = "decision"
)

// AuditEvent is a single structured record. It is intentionally
// JSON-friendly so an Audit sink can be a file, stdout, an HTTP
// collector, or a database — all without changing this type.
//
// "Why" each field exists:
//   - RequestID/TraceID: link back to the user request and distributed
//     trace, so operators can reconstruct a session end-to-end.
//   - Actor: which agent / tool produced this event. Lets you ask
//     "what did Remediator do today?".
//   - Action: short verb-phrase, e.g. "delete pod", "create deployment".
//   - Target: the Kubernetes resource the event is about.
//   - Outcome: success / failure / skipped / passed / failed.
//   - Reason: short human-readable justification (especially for
//     skipped / failed outcomes).
//   - Details: free-form structured data for that event kind.
type AuditEvent struct {
	Timestamp time.Time              `json:"timestamp"`
	Kind      AuditEventKind         `json:"kind"`
	RequestID string                 `json:"request_id,omitempty"`
	TraceID   string                 `json:"trace_id,omitempty"`
	Actor     string                 `json:"actor"`
	Action    string                 `json:"action"`
	Target    AuditTarget            `json:"target,omitempty"`
	Outcome   string                 `json:"outcome"`
	Reason    string                 `json:"reason,omitempty"`
	Details   map[string]interface{} `json:"details,omitempty"`
}

// AuditTarget is the resource an event refers to. All fields optional;
// events not tied to a specific resource (e.g. a coordinator decision)
// can leave it empty.
type AuditTarget struct {
	Kind      string `json:"kind,omitempty"`
	Name      string `json:"name,omitempty"`
	Namespace string `json:"namespace,omitempty"`
}

// AuditLogger is the Sensor for "what did the system do". Implementations
// are expected to be safe for concurrent calls because tasks run in
// parallel goroutines.
type AuditLogger interface {
	// Record persists a single event. Implementations should not block
	// the caller for IO that might fail (use buffered/async writes if
	// the sink is slow), but they MUST NOT silently drop events.
	Record(ctx context.Context, event AuditEvent) error
}

// JSONLogAuditor writes one JSON object per line to an io.Writer
// (jsonl format). This is the recommended default sink because:
//   - jsonl is trivially greppable from a shell.
//   - Many log aggregators (Loki, Cloudwatch, Datadog) ingest jsonl
//     out of the box.
//   - It survives partial writes: a torn last line can be discarded
//     without corrupting earlier records.
type JSONLogAuditor struct {
	mu     sync.Mutex
	writer io.Writer
}

// NewJSONLogAuditor wraps a writer. Use os.Stdout for development,
// or a *os.File pointing at /var/log/kubeagent/audit.jsonl in production.
// If writer is nil, falls back to os.Stdout to avoid silent drops.
func NewJSONLogAuditor(writer io.Writer) *JSONLogAuditor {
	if writer == nil {
		writer = os.Stdout
	}
	return &JSONLogAuditor{writer: writer}
}

// Record serializes the event to JSON and writes a single line. The
// mutex is intentional: the underlying writer (e.g. os.File) is not
// guaranteed to provide atomic interleaving for concurrent multi-byte
// writes, and jsonl correctness depends on one event = one line.
func (a *JSONLogAuditor) Record(_ context.Context, event AuditEvent) error {
	if event.Timestamp.IsZero() {
		event.Timestamp = time.Now()
	}

	data, err := json.Marshal(event)
	if err != nil {
		return fmt.Errorf("audit: marshal event: %w", err)
	}

	a.mu.Lock()
	defer a.mu.Unlock()

	if _, err := a.writer.Write(append(data, '\n')); err != nil {
		return fmt.Errorf("audit: write event: %w", err)
	}
	return nil
}

// NoopAuditor satisfies AuditLogger but discards everything. Use only
// in tests or when audit is intentionally disabled. Production code
// should NOT default to this — silent audit is worse than no audit
// because operators stop expecting events to appear.
type NoopAuditor struct{}

// Record discards the event.
func (NoopAuditor) Record(_ context.Context, _ AuditEvent) error { return nil }
