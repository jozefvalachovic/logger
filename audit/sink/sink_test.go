package sink

import (
	"bytes"
	"testing"
	"time"

	"github.com/jozefvalachovic/logger/v4/audit"
)

func testEntry() *audit.AuditEntry {
	return &audit.AuditEntry{
		ID:        "test-entry-1",
		Timestamp: time.Now().UTC(),
		Event: audit.AuditEvent{
			Type:    audit.AuditAuth,
			Action:  "login",
			Outcome: audit.OutcomeSuccess,
			Actor:   audit.AuditActor{ID: "user-1", Type: "user"},
		},
		SchemaVersion: audit.CurrentSchemaVersion,
	}
}

func TestWriterSink(t *testing.T) {
	var buf bytes.Buffer
	s := NewWriterSink(&buf)

	if err := s.Write(testEntry()); err != nil {
		t.Fatalf("Write() error: %v", err)
	}

	if buf.Len() == 0 {
		t.Error("expected output from writer sink")
	}

	if err := s.Flush(); err != nil {
		t.Errorf("Flush() error: %v", err)
	}
	if err := s.Close(); err != nil {
		t.Errorf("Close() error: %v", err)
	}
}

func TestMultiSink(t *testing.T) {
	var buf1, buf2 bytes.Buffer
	s1 := NewWriterSink(&buf1)
	s2 := NewWriterSink(&buf2)

	multi := NewMultiSink(s1, s2)

	if err := multi.Write(testEntry()); err != nil {
		t.Fatalf("Write() error: %v", err)
	}

	if buf1.Len() == 0 {
		t.Error("sink1 should have output")
	}
	if buf2.Len() == 0 {
		t.Error("sink2 should have output")
	}

	if err := multi.Flush(); err != nil {
		t.Errorf("Flush() error: %v", err)
	}
	if err := multi.Close(); err != nil {
		t.Errorf("Close() error: %v", err)
	}
}

func TestFileSinkWriteAndRotate(t *testing.T) {
	tmpDir := t.ArtifactDir()

	s, err := NewFileSink(FileSinkConfig{
		Path:    tmpDir + "/audit.jsonl",
		MaxSize: 200,
	})
	if err != nil {
		t.Fatalf("NewFileSink() error: %v", err)
	}

	for i := range 10 {
		entry := testEntry()
		entry.ID = "entry-" + time.Now().Format("150405.000000000")
		if err := s.Write(entry); err != nil {
			t.Fatalf("Write() error on iteration %d: %v", i, err)
		}
	}

	if err := s.Flush(); err != nil {
		t.Errorf("Flush() error: %v", err)
	}
	if err := s.Close(); err != nil {
		t.Errorf("Close() error: %v", err)
	}
}
