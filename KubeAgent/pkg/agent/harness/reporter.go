package harness

import (
	"context"
	"fmt"
	"io"
	"os"
	"strings"
	"sync"
	"time"
)

// ConsoleReporter is an AuditLogger that renders events for humans
// reading a terminal — the format we want during demos and during
// interactive use of `kubeagent fix`. It is the visual face of the
// harness: every Sensor event the system produces flows through here
// and becomes a labelled, colour-prefixed line.
//
// Not a replacement for JSONLogAuditor: in production you typically
// want both — ConsoleReporter for the operator's terminal,
// JSONLogAuditor (mounted at a file or shipped to a log collector)
// for the durable audit trail. Use Tee to wire both at once.
type ConsoleReporter struct {
	mu      sync.Mutex
	out     io.Writer
	colored bool
	// startedAt anchors the elapsed-time prefix so the log reads as
	// a stopwatch from session start, not as wallclock noise.
	startedAt time.Time
}

// NewConsoleReporter writes to the given writer. Pass os.Stdout for
// the typical interactive case. Colour is auto-enabled when the
// writer is a terminal; pass DisableColor() if the output is being
// captured (e.g. piped to less or to a log file).
func NewConsoleReporter(out io.Writer) *ConsoleReporter {
	if out == nil {
		out = os.Stdout
	}
	return &ConsoleReporter{
		out:       out,
		colored:   isTerminal(out),
		startedAt: time.Now(),
	}
}

// DisableColor turns off ANSI escape sequences. Useful when the
// reporter writes to a file or when the operator's terminal does not
// understand colour.
func (r *ConsoleReporter) DisableColor() *ConsoleReporter {
	r.colored = false
	return r
}

// Record implements AuditLogger by formatting the event into one or
// more human lines. Different event kinds get different layouts: an
// AuditAction shows verb+target+outcome; an AuditVerification shows
// the verifier's verdict and observations; a Preflight shows the
// guard's decision and reason.
func (r *ConsoleReporter) Record(_ context.Context, event AuditEvent) error {
	r.mu.Lock()
	defer r.mu.Unlock()

	r.writeHeader(event)
	r.writeBody(event)
	return nil
}

// writeHeader renders the leading status line, e.g.
//   [00:01.42] [SENSOR] verification: passed (resource pod/foo reached Running)
func (r *ConsoleReporter) writeHeader(event AuditEvent) {
	elapsed := time.Since(r.startedAt)
	timestamp := fmt.Sprintf("[%02d:%05.2f]",
		int(elapsed.Minutes()),
		elapsed.Seconds()-float64(int(elapsed.Minutes()))*60)

	tag, colour := tagForKind(event.Kind, event.Outcome)
	tagStr := r.colourise(fmt.Sprintf("[%s]", tag), colour)

	headline := event.Action
	if event.Reason != "" {
		headline = fmt.Sprintf("%s — %s", event.Action, event.Reason)
	}
	outcomeStr := r.colourise(event.Outcome, outcomeColour(event.Outcome))

	fmt.Fprintf(r.out, "%s %s %s · %s · %s\n",
		timestamp,
		tagStr,
		event.Actor,
		headline,
		outcomeStr,
	)
}

// writeBody renders the indented detail block for events that carry
// extra observations. Kept compact — one observation per line, only
// the keys that operators actually care about.
func (r *ConsoleReporter) writeBody(event AuditEvent) {
	if event.Target.Kind != "" || event.Target.Name != "" {
		fmt.Fprintf(r.out, "           target: %s/%s in %s\n",
			defaultStr(event.Target.Kind, "?"),
			defaultStr(event.Target.Name, "?"),
			defaultStr(event.Target.Namespace, "default"),
		)
	}
	if len(event.Details) == 0 {
		return
	}

	// Surface the small handful of fields that matter for the demo.
	// We don't dump the whole map — that defeats the purpose of a
	// human-readable view.
	keys := []string{"phase", "reason", "ready", "exists", "warnings"}
	for _, k := range keys {
		if v, ok := event.Details[k]; ok && !isZero(v) {
			fmt.Fprintf(r.out, "           %s: %v\n", k, v)
		}
	}
}

// tagForKind produces the [TAG] label and a colour code. The
// vocabulary is intentionally short so the eye can scan a long log:
//   GUIDE   — a Preflight check fired
//   ACTION  — the agent did a thing
//   SENSOR  — a Verifier observed an outcome
//   DECIDE  — the harness took a control-flow decision
func tagForKind(kind AuditEventKind, outcome string) (string, string) {
	switch kind {
	case AuditPreflight:
		if strings.EqualFold(outcome, "block") {
			return "GUIDE!", colourRed
		}
		return "GUIDE", colourCyan
	case AuditAction:
		return "ACTION", colourYellow
	case AuditVerification:
		return "SENSOR", colourBlue
	case AuditDecision:
		return "DECIDE", colourMagenta
	}
	return string(kind), colourReset
}

// outcomeColour pairs an outcome string with a colour. We err on the
// side of "green for good, red for bad, yellow for uncertain" because
// that matches operator intuition during incident response.
func outcomeColour(outcome string) string {
	switch strings.ToLower(outcome) {
	case "success", "passed", "allow":
		return colourGreen
	case "failure", "failed", "block":
		return colourRed
	case "inconclusive", "warn", "skipped":
		return colourYellow
	}
	return colourReset
}

func (r *ConsoleReporter) colourise(s, code string) string {
	if !r.colored || code == "" {
		return s
	}
	return code + s + colourReset
}

const (
	colourReset   = "\x1b[0m"
	colourRed     = "\x1b[31m"
	colourGreen   = "\x1b[32m"
	colourYellow  = "\x1b[33m"
	colourBlue    = "\x1b[34m"
	colourMagenta = "\x1b[35m"
	colourCyan    = "\x1b[36m"
)

// isTerminal answers "is this writer attached to an interactive tty".
// We only enable ANSI escapes when it is, so piping output to a file
// or another process doesn't pollute it with control sequences.
func isTerminal(w io.Writer) bool {
	f, ok := w.(*os.File)
	if !ok {
		return false
	}
	stat, err := f.Stat()
	if err != nil {
		return false
	}
	return (stat.Mode() & os.ModeCharDevice) != 0
}

func defaultStr(s, dflt string) string {
	if s == "" {
		return dflt
	}
	return s
}

// isZero filters out empty/false/zero values so the body block stays
// uncluttered. Booleans are treated specially: false is "interesting"
// for some keys (e.g. ready=false) and we want it shown.
func isZero(v interface{}) bool {
	if v == nil {
		return true
	}
	switch x := v.(type) {
	case string:
		return x == ""
	case []interface{}:
		return len(x) == 0
	}
	return false
}

// Tee fans out one event to multiple AuditLoggers. Use it to send
// events to ConsoleReporter (for the operator) and JSONLogAuditor
// (for the durable trail) at the same time, without making either
// implementation aware of the other.
type Tee struct {
	sinks []AuditLogger
}

// NewTee constructs a tee. Nil sinks are dropped; if the resulting
// list is empty, the tee behaves like NoopAuditor.
func NewTee(sinks ...AuditLogger) *Tee {
	clean := make([]AuditLogger, 0, len(sinks))
	for _, s := range sinks {
		if s != nil {
			clean = append(clean, s)
		}
	}
	return &Tee{sinks: clean}
}

// Record forwards to every sink. If multiple sinks fail, only the
// first error is returned — the rest are dropped because there is no
// reasonable single-error-to-return-on-multi-failure semantics, and
// we'd rather flag the first problem than silently swallow it.
func (t *Tee) Record(ctx context.Context, event AuditEvent) error {
	var firstErr error
	for _, s := range t.sinks {
		if err := s.Record(ctx, event); err != nil && firstErr == nil {
			firstErr = err
		}
	}
	return firstErr
}
