package agent

import (
	"fmt"
	"log"
	"os"
	"time"
)

// SimpleLogger is a simple implementation of Logger
type SimpleLogger struct {
	prefix string
	logger *log.Logger
}

// NewSimpleLogger creates a new simple logger
func NewSimpleLogger(prefix string) *SimpleLogger {
	return &SimpleLogger{
		prefix: prefix,
		logger: log.New(os.Stdout, "", 0),
	}
}

func (l *SimpleLogger) formatMessage(level, msg string, fields map[string]interface{}) string {
	timestamp := time.Now().Format("2006-01-02 15:04:05")
	message := fmt.Sprintf("[%s] %s %s: %s", timestamp, l.prefix, level, msg)

	if len(fields) > 0 {
		message += " {"
		first := true
		for k, v := range fields {
			if !first {
				message += ", "
			}
			message += fmt.Sprintf("%s=%v", k, v)
			first = false
		}
		message += "}"
	}

	return message
}

// Debug logs a debug message
func (l *SimpleLogger) Debug(msg string, fields map[string]interface{}) {
	l.logger.Println(l.formatMessage("DEBUG", msg, fields))
}

// Info logs an info message
func (l *SimpleLogger) Info(msg string, fields map[string]interface{}) {
	l.logger.Println(l.formatMessage("INFO", msg, fields))
}

// Warn logs a warning message
func (l *SimpleLogger) Warn(msg string, fields map[string]interface{}) {
	l.logger.Println(l.formatMessage("WARN", msg, fields))
}

// Error logs an error message
func (l *SimpleLogger) Error(msg string, fields map[string]interface{}) {
	l.logger.Println(l.formatMessage("ERROR", msg, fields))
}

// NoOpLogger is a logger that does nothing
type NoOpLogger struct{}

// NewNoOpLogger creates a new no-op logger
func NewNoOpLogger() *NoOpLogger {
	return &NoOpLogger{}
}

// Debug does nothing
func (l *NoOpLogger) Debug(msg string, fields map[string]interface{}) {}

// Info does nothing
func (l *NoOpLogger) Info(msg string, fields map[string]interface{}) {}

// Warn does nothing
func (l *NoOpLogger) Warn(msg string, fields map[string]interface{}) {}

// Error does nothing
func (l *NoOpLogger) Error(msg string, fields map[string]interface{}) {}
