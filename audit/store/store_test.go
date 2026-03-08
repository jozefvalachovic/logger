package store

import (
	"testing"
	"time"

	"github.com/jozefvalachovic/logger/v4/audit"
)

func makeEntry(id string, eventType audit.AuditEventType, ts time.Time) *audit.AuditEntry {
	return &audit.AuditEntry{
		ID:        id,
		Timestamp: ts,
		Event: audit.AuditEvent{
			Type:    eventType,
			Action:  "test-action",
			Outcome: audit.OutcomeSuccess,
			Actor:   audit.AuditActor{ID: "actor-1", Type: "user"},
		},
		SchemaVersion: audit.CurrentSchemaVersion,
	}
}

func TestMemoryStoreBasic(t *testing.T) {
	s := NewMemoryStore(MemoryStoreConfig{MaxSize: 100})

	now := time.Now().UTC()
	entry := makeEntry("e1", audit.AuditAuth, now)
	if err := s.Store(entry); err != nil {
		t.Fatalf("Store() error: %v", err)
	}

	got, err := s.Get("e1")
	if err != nil {
		t.Fatalf("Get() error: %v", err)
	}
	if got.ID != "e1" {
		t.Errorf("Get() returned ID %q, want %q", got.ID, "e1")
	}

	if s.Count() != 1 {
		t.Errorf("Count() = %d, want 1", s.Count())
	}
}

func TestMemoryStoreEviction(t *testing.T) {
	s := NewMemoryStore(MemoryStoreConfig{MaxSize: 3})

	now := time.Now().UTC()
	for i := range 5 {
		entry := makeEntry("e"+string(rune('0'+i)), audit.AuditAuth, now.Add(time.Duration(i)*time.Second))
		if err := s.Store(entry); err != nil {
			t.Fatalf("Store() error: %v", err)
		}
	}

	if s.Count() != 3 {
		t.Errorf("Count() = %d, want 3 after eviction", s.Count())
	}

	if _, err := s.Get("e0"); err != audit.ErrEntryNotFound {
		t.Errorf("Get(e0) expected ErrEntryNotFound, got %v", err)
	}
	if _, err := s.Get("e1"); err != audit.ErrEntryNotFound {
		t.Errorf("Get(e1) expected ErrEntryNotFound, got %v", err)
	}
}

func TestMemoryStoreQuery(t *testing.T) {
	s := NewMemoryStore(MemoryStoreConfig{MaxSize: 100})

	now := time.Now().UTC()
	_ = s.Store(makeEntry("e1", audit.AuditAuth, now))
	_ = s.Store(makeEntry("e2", audit.AuditDataAccess, now.Add(time.Second)))
	_ = s.Store(makeEntry("e3", audit.AuditAuth, now.Add(2*time.Second)))

	result, err := s.Query(audit.Query{
		EventTypes: []audit.AuditEventType{audit.AuditAuth},
		Limit:      10,
	})
	if err != nil {
		t.Fatalf("Query() error: %v", err)
	}
	if len(result.Entries) != 2 {
		t.Errorf("Query() returned %d entries, want 2", len(result.Entries))
	}
}

func TestMemoryStoreClear(t *testing.T) {
	s := NewMemoryStore(MemoryStoreConfig{MaxSize: 100})
	_ = s.Store(makeEntry("e1", audit.AuditAuth, time.Now()))
	s.Clear()
	if s.Count() != 0 {
		t.Errorf("Count() after Clear() = %d, want 0", s.Count())
	}
}

func TestFileStoreRoundTrip(t *testing.T) {
	tmpDir := t.ArtifactDir()

	s, err := NewFileStore(FileStoreConfig{BasePath: tmpDir})
	if err != nil {
		t.Fatalf("NewFileStore() error: %v", err)
	}

	now := time.Now().UTC()
	entry := makeEntry("f1", audit.AuditAuth, now)
	if err := s.Store(entry); err != nil {
		t.Fatalf("Store() error: %v", err)
	}

	got, err := s.Get("f1")
	if err != nil {
		t.Fatalf("Get() error: %v", err)
	}
	if got.ID != "f1" {
		t.Errorf("Get() ID = %q, want %q", got.ID, "f1")
	}

	result, err := s.Query(audit.Query{Limit: 10})
	if err != nil {
		t.Fatalf("Query() error: %v", err)
	}
	if len(result.Entries) != 1 {
		t.Errorf("Query() returned %d entries, want 1", len(result.Entries))
	}
}

func TestFileStoreGetNotFound(t *testing.T) {
	tmpDir := t.ArtifactDir()
	s, _ := NewFileStore(FileStoreConfig{BasePath: tmpDir})

	_, err := s.Get("nonexistent")
	if err != audit.ErrEntryNotFound {
		t.Errorf("Get() expected ErrEntryNotFound, got %v", err)
	}
}
