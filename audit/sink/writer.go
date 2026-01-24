package sink

import (
	"encoding/json"
	"io"
	"sync"

	"github.com/jozefvalachovic/logger/v4/audit"
)

// WriterSink writes audit entries to an io.Writer
type WriterSink struct {
	mu sync.Mutex
	w  io.Writer
}

// NewWriterSink creates a new writer sink
func NewWriterSink(w io.Writer) *WriterSink {
	return &WriterSink{w: w}
}

// Write writes an audit entry to the writer
func (s *WriterSink) Write(entry *audit.AuditEntry) error {
	data, err := json.Marshal(entry)
	if err != nil {
		return err
	}
	data = append(data, '\n')

	s.mu.Lock()
	defer s.mu.Unlock()

	_, err = s.w.Write(data)
	return err
}

// Flush is a no-op for writer sink
func (s *WriterSink) Flush() error {
	return nil
}

// Close is a no-op for writer sink (doesn't own the writer)
func (s *WriterSink) Close() error {
	return nil
}
