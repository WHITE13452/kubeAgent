package audit

import (
	"encoding/json"
	"fmt"
	"os"
	"sync"
	"time"

	"kubeagent/pkg/agent"
)

// AuditEntry represents a single audit log entry
type AuditEntry struct {
	Timestamp    time.Time              `json:"timestamp"`
	RequestID    string                 `json:"request_id"`
	AgentType    string                 `json:"agent_type"`
	UserID       string                 `json:"user_id"`
	Action       string                 `json:"action"`
	Resource     string                 `json:"resource"`
	Namespace    string                 `json:"namespace,omitempty"`
	Verb         string                 `json:"verb"`
	Result       string                 `json:"result"`
	Error        string                 `json:"error,omitempty"`
	Metadata     map[string]interface{} `json:"metadata,omitempty"`
	DurationMs   int64                  `json:"duration_ms"`
}

// AuditLogger logs all K8s API calls for security auditing
type AuditLogger struct {
	mu         sync.Mutex
	entries    []AuditEntry
	maxEntries int
	outputFile *os.File
	logger     agent.Logger
}

// NewAuditLogger creates a new audit logger
func NewAuditLogger(logger agent.Logger, outputPath string) (*AuditLogger, error) {
	l := &AuditLogger{
		entries:    make([]AuditEntry, 0),
		maxEntries: 10000,
		logger:     logger,
	}

	if outputPath != "" {
		f, err := os.OpenFile(outputPath, os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0644)
		if err != nil {
			return nil, fmt.Errorf("failed to open audit log file: %w", err)
		}
		l.outputFile = f
	}

	return l, nil
}

// Log logs an audit entry
func (a *AuditLogger) Log(entry AuditEntry) {
	a.mu.Lock()
	defer a.mu.Unlock()

	entry.Timestamp = time.Now().UTC()
	a.entries = append(a.entries, entry)

	// Trim if exceeds max
	if len(a.entries) > a.maxEntries {
		a.entries = a.entries[a.maxEntries:]
	}

	// Write to file if available
	if a.outputFile != nil {
		data, _ := json.Marshal(entry)
		a.outputFile.Write(append(data, '\n'))
	}

	// Also log to agent logger
	a.logger.Info(fmt.Sprintf("[AUDIT] %s %s %s/%s", entry.Verb, entry.AgentType, entry.Resource, entry.Namespace),
		map[string]interface{}{
			"request_id": entry.RequestID,
			"result":     entry.Result,
		})
}

// LogAction logs a K8s API action
func (a *AuditLogger) LogAction(requestID, agentType, userID, action, resource, namespace, verb, result string, durationMs int64) {
	a.Log(AuditEntry{
		RequestID:  requestID,
		AgentType:  agentType,
		UserID:     userID,
		Action:     action,
		Resource:   resource,
		Namespace:  namespace,
		Verb:       verb,
		Result:     result,
		DurationMs: durationMs,
	})
}

// GetEntries returns all audit entries
func (a *AuditLogger) GetEntries() []AuditEntry {
	a.mu.Lock()
	defer a.mu.Unlock()
	entries := make([]AuditEntry, len(a.entries))
	copy(entries, a.entries)
	return entries
}

// FilterByAgent filters entries by agent type
func (a *AuditLogger) FilterByAgent(agentType string) []AuditEntry {
	a.mu.Lock()
	defer a.mu.Unlock()

	var filtered []AuditEntry
	for _, e := range a.entries {
		if e.AgentType == agentType {
			filtered = append(filtered, e)
		}
	}
	return filtered
}

// FilterByUser filters entries by user ID
func (a *AuditLogger) FilterByUser(userID string) []AuditEntry {
	a.mu.Lock()
	defer a.mu.Unlock()

	var filtered []AuditEntry
	for _, e := range a.entries {
		if e.UserID == userID {
			filtered = append(filtered, e)
		}
	}
	return filtered
}

// Close closes the audit logger
func (a *AuditLogger) Close() error {
	if a.outputFile != nil {
		return a.outputFile.Close()
	}
	return nil
}
