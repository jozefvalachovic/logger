package sink

import (
	"sync"

	"github.com/jozefvalachovic/logger/v4/audit"
)

// MultiSink writes to multiple sinks
type MultiSink struct {
	mu    sync.RWMutex
	sinks []audit.Sink
}

// NewMultiSink creates a new multi-sink
func NewMultiSink(sinks ...audit.Sink) *MultiSink {
	return &MultiSink{
		sinks: sinks,
	}
}

// AddSink adds a sink to the multi-sink
func (m *MultiSink) AddSink(s audit.Sink) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.sinks = append(m.sinks, s)
}

// Write writes to all sinks
func (m *MultiSink) Write(entry *audit.AuditEntry) error {
	m.mu.RLock()
	defer m.mu.RUnlock()

	var lastErr error
	for _, sink := range m.sinks {
		if err := sink.Write(entry); err != nil {
			lastErr = err
		}
	}
	return lastErr
}

// Flush flushes all sinks
func (m *MultiSink) Flush() error {
	m.mu.RLock()
	defer m.mu.RUnlock()

	var lastErr error
	for _, sink := range m.sinks {
		if err := sink.Flush(); err != nil {
			lastErr = err
		}
	}
	return lastErr
}

// Close closes all sinks
func (m *MultiSink) Close() error {
	m.mu.Lock()
	defer m.mu.Unlock()

	var lastErr error
	for _, sink := range m.sinks {
		if err := sink.Close(); err != nil {
			lastErr = err
		}
	}
	return lastErr
}
