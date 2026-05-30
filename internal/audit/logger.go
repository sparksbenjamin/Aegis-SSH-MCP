// Package audit provides structured JSON logging for every command attempt
// that passes through the Aegis security filter.  All log output goes to
// stderr so that stdout remains uncontaminated for the MCP stdio transport.
package audit

import (
	"encoding/json"
	"fmt"
	"os"
	"sync"
	"time"
)

// Entry is the canonical audit record.  Every field is always emitted so
// downstream log aggregators (Loki, Elastic, etc.) get a consistent schema.
type Entry struct {
	Timestamp        string `json:"timestamp"`
	AgentAlias       string `json:"agent_alias"`
	TargetHost       string `json:"target_host,omitempty"`
	CommandRequested string `json:"command_requested"`
	// ValidationResult is one of: PASS | FAIL | EXEC_ERROR | SYSTEM
	ValidationResult string `json:"validation_result"`
	// ValidationReason provides human-readable context for any non-PASS result.
	ValidationReason string `json:"validation_reason,omitempty"`
	// OutputSummary is a ≤200-char preview of stdout; never contains redacted data.
	OutputSummary string `json:"output_summary,omitempty"`
	ExitCode      int    `json:"exit_code,omitempty"`
	DurationMs    int64  `json:"duration_ms"`
	// StealthMode is true when the Aegis filter returned a honeypot response
	// instead of a real error.
	StealthMode bool `json:"stealth_mode,omitempty"`
}

// Logger serialises audit entries as newline-delimited JSON to a writer
// (default: os.Stderr).  All writes are serialised under a mutex so it is
// safe to call Log from concurrent Goroutines.
type Logger struct {
	mu  sync.Mutex
	out *os.File
}

// New returns a Logger that writes to stderr.
func New() *Logger {
	return &Logger{out: os.Stderr}
}

// NewWithFile returns a Logger that appends to the file at path (creates it
// if absent).  Useful when running outside Docker and stderr is noisy.
func NewWithFile(path string) (*Logger, error) {
	f, err := os.OpenFile(path, os.O_CREATE|os.O_WRONLY|os.O_APPEND, 0o600)
	if err != nil {
		return nil, fmt.Errorf("audit: open log file %s: %w", path, err)
	}
	return &Logger{out: f}, nil
}

// Log stamps the entry with the current UTC time, marshals it to JSON and
// writes it as a single line.  Marshal errors are surfaced inline rather
// than swallowed.
func (l *Logger) Log(e Entry) {
	e.Timestamp = time.Now().UTC().Format(time.RFC3339Nano)

	data, err := json.Marshal(e)
	if err != nil {
		l.mu.Lock()
		fmt.Fprintf(l.out, `{"timestamp":%q,"error":"marshal failed: %v"}`+"\n",
			e.Timestamp, err)
		l.mu.Unlock()
		return
	}

	l.mu.Lock()
	fmt.Fprintln(l.out, string(data))
	l.mu.Unlock()
}

// System is a convenience wrapper for internal / watcher events.
func (l *Logger) System(action, result, reason string) {
	l.Log(Entry{
		AgentAlias:       "aegis-system",
		CommandRequested: action,
		ValidationResult: result,
		ValidationReason: reason,
	})
}
