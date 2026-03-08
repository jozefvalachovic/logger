package audit

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"path/filepath"
	"runtime"
	"testing"
	"time"
)

func validEvent() AuditEvent {
	return AuditEvent{
		Type:    AuditAuth,
		Action:  "login",
		Outcome: OutcomeSuccess,
		Actor:   AuditActor{ID: "user-1", Type: "user"},
	}
}

func TestHashChainChainAndVerify(t *testing.T) {
	hc := NewHashChain(HashChainConfig{Algorithm: "sha256"})
	entry := &AuditEntry{
		ID:        "entry-1",
		Timestamp: time.Now().UTC(),
		Event:     validEvent(),
	}

	hash, err := hc.Chain(entry)
	if err != nil {
		t.Fatalf("Chain() error: %v", err)
	}
	if hash == "" {
		t.Fatal("Chain() returned empty hash")
	}
	if entry.Hash != hash {
		t.Error("entry.Hash not set")
	}
	if entry.Sequence != 1 {
		t.Errorf("expected sequence 1, got %d", entry.Sequence)
	}

	valid, err := hc.Verify(entry)
	if err != nil {
		t.Fatalf("Verify() error: %v", err)
	}
	if !valid {
		t.Error("Verify() returned false for valid entry")
	}
}

func TestHashChainVerifyChain(t *testing.T) {
	hc := NewHashChain(HashChainConfig{Algorithm: "sha256"})
	entries := make([]AuditEntry, 3)
	for i := range entries {
		e := &AuditEntry{
			ID:        generateUUID(),
			Timestamp: time.Now().UTC(),
			Event:     validEvent(),
		}
		if _, err := hc.Chain(e); err != nil {
			t.Fatalf("Chain() error: %v", err)
		}
		entries[i] = *e
	}

	if err := hc.VerifyChain(entries); err != nil {
		t.Fatalf("VerifyChain() error: %v", err)
	}

	entries[1].Event.Action = "tampered"
	if err := hc.VerifyChain(entries); err == nil {
		t.Error("expected error for tampered chain")
	}
}

func TestHashChainHMAC(t *testing.T) {
	hc := NewHashChain(HashChainConfig{
		Algorithm:  "sha512",
		SigningKey: []byte("secret-key"),
	})
	entry := &AuditEntry{
		ID:        "entry-hmac",
		Timestamp: time.Now().UTC(),
		Event:     validEvent(),
	}

	hash, err := hc.Chain(entry)
	if err != nil {
		t.Fatalf("Chain() error: %v", err)
	}
	if hash == "" {
		t.Fatal("HMAC hash should not be empty")
	}

	valid, err := hc.Verify(entry)
	if err != nil {
		t.Fatalf("Verify() error: %v", err)
	}
	if !valid {
		t.Error("HMAC verify failed")
	}
}

func TestEventValidation(t *testing.T) {
	tests := []struct {
		name    string
		event   AuditEvent
		wantErr bool
	}{
		{
			name:    "valid event",
			event:   validEvent(),
			wantErr: false,
		},
		{
			name: "missing type",
			event: AuditEvent{
				Action:  "login",
				Outcome: OutcomeSuccess,
				Actor:   AuditActor{ID: "u1", Type: "user"},
			},
			wantErr: true,
		},
		{
			name: "missing action",
			event: AuditEvent{
				Type:    AuditAuth,
				Outcome: OutcomeSuccess,
				Actor:   AuditActor{ID: "u1", Type: "user"},
			},
			wantErr: true,
		},
		{
			name: "missing actor",
			event: AuditEvent{
				Type:    AuditAuth,
				Action:  "login",
				Outcome: OutcomeSuccess,
			},
			wantErr: true,
		},
		{
			name: "missing outcome defaults to unknown",
			event: AuditEvent{
				Type:   AuditAuth,
				Action: "login",
				Actor:  AuditActor{ID: "u1", Type: "user"},
			},
			wantErr: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := tt.event.Validate()
			if (err != nil) != tt.wantErr {
				t.Errorf("Validate() error = %v, wantErr %v", err, tt.wantErr)
			}
		})
	}
}

func TestRateLimiter(t *testing.T) {
	rl := NewRateLimiter(&RateLimitConfig{
		EventsPerSecond: 10,
		BurstSize:       5,
		DropWhenLimited: true,
	})

	for i := range 5 {
		if !rl.Allow() {
			t.Errorf("Allow() should succeed for burst token %d", i)
		}
	}

	if rl.Allow() {
		t.Error("Allow() should fail after burst exhausted")
	}
	if !rl.DropWhenLimited() {
		t.Error("DropWhenLimited() should return true")
	}
}

func TestRateLimiterAllowN(t *testing.T) {
	rl := NewRateLimiter(&RateLimitConfig{
		EventsPerSecond: 100,
		BurstSize:       10,
	})

	if !rl.AllowN(5) {
		t.Error("AllowN(5) should succeed with 10 tokens")
	}
	if rl.AllowN(10) {
		t.Error("AllowN(10) should fail with only 5 tokens left")
	}
}

func TestRateLimiterWait(t *testing.T) {
	rl := NewRateLimiter(&RateLimitConfig{
		EventsPerSecond: 100,
		BurstSize:       1,
	})

	wait := rl.Wait()
	if wait != 0 {
		t.Errorf("Wait() should return 0 for first token, got %v", wait)
	}

	wait = rl.Wait()
	if wait <= 0 {
		t.Error("Wait() should return positive duration when tokens exhausted")
	}
}

func TestGenerateUUID(t *testing.T) {
	seen := make(map[string]bool)
	for range 1000 {
		id := generateUUID()
		if id == "" {
			t.Fatal("generateUUID() returned empty string")
		}
		if seen[id] {
			t.Fatalf("duplicate ID generated: %s", id)
		}
		seen[id] = true
	}
}

func TestWALWriteAndRecover(t *testing.T) {
	tmpDir := t.TempDir()
	walPath := filepath.Join(tmpDir, "test.wal")

	wal, err := NewWAL(WALConfig{
		Enabled:     true,
		Path:        walPath,
		SyncOnWrite: true,
	})
	if err != nil {
		t.Fatalf("NewWAL() error: %v", err)
	}

	entry := &AuditEntry{
		ID:        "wal-entry-1",
		Timestamp: time.Now().UTC(),
		Event:     validEvent(),
	}
	if err := wal.Write(entry); err != nil {
		t.Fatalf("Write() error: %v", err)
	}
	if err := wal.Flush(); err != nil {
		t.Fatalf("Flush() error: %v", err)
	}
	if err := wal.Close(); err != nil {
		t.Fatalf("Close() error: %v", err)
	}

	wal2, err := NewWAL(WALConfig{
		Enabled:     true,
		Path:        walPath,
		SyncOnWrite: true,
	})
	if err != nil {
		t.Fatalf("NewWAL() reopen error: %v", err)
	}
	defer func() { _ = wal2.Close() }()

	uncommitted, err := wal2.Recover()
	if err != nil {
		t.Fatalf("Recover() error: %v", err)
	}
	if len(uncommitted) != 1 {
		t.Fatalf("expected 1 uncommitted entry, got %d", len(uncommitted))
	}
	if uncommitted[0].ID != "wal-entry-1" {
		t.Errorf("expected entry ID wal-entry-1, got %s", uncommitted[0].ID)
	}
}

func TestWALCommitRemovesFromRecovery(t *testing.T) {
	tmpDir := t.TempDir()
	walPath := filepath.Join(tmpDir, "commit.wal")

	wal, err := NewWAL(WALConfig{
		Enabled:     true,
		Path:        walPath,
		SyncOnWrite: true,
	})
	if err != nil {
		t.Fatalf("NewWAL() error: %v", err)
	}

	entry := &AuditEntry{
		ID:        "committed-entry",
		Timestamp: time.Now().UTC(),
		Event:     validEvent(),
	}
	if err := wal.Write(entry); err != nil {
		t.Fatalf("Write() error: %v", err)
	}
	if err := wal.Commit(entry.ID); err != nil {
		t.Fatalf("Commit() error: %v", err)
	}
	if err := wal.Flush(); err != nil {
		t.Fatalf("Flush() error: %v", err)
	}
	if err := wal.Close(); err != nil {
		t.Fatalf("Close() error: %v", err)
	}

	wal2, err := NewWAL(WALConfig{
		Enabled:     true,
		Path:        walPath,
		SyncOnWrite: true,
	})
	if err != nil {
		t.Fatalf("NewWAL() reopen error: %v", err)
	}
	defer func() { _ = wal2.Close() }()

	uncommitted, err := wal2.Recover()
	if err != nil {
		t.Fatalf("Recover() error: %v", err)
	}
	if len(uncommitted) != 0 {
		t.Errorf("expected 0 uncommitted entries after commit, got %d", len(uncommitted))
	}
}

func TestLoggerLogAndClose(t *testing.T) {
	var buf bytes.Buffer
	cfg := DefaultConfig()
	cfg.Output = &buf
	cfg.BufferSize = 10

	logger, err := New(cfg)
	if err != nil {
		t.Fatalf("New() error: %v", err)
	}

	err = logger.Log(context.Background(), validEvent())
	if err != nil {
		t.Fatalf("Log() error: %v", err)
	}

	time.Sleep(50 * time.Millisecond)

	if err := logger.Close(); err != nil {
		t.Fatalf("Close() error: %v", err)
	}

	err = logger.Log(context.Background(), validEvent())
	if err != ErrAuditLoggerClosed {
		t.Errorf("expected ErrAuditLoggerClosed, got %v", err)
	}
}

func TestLoggerLogSync(t *testing.T) {
	var buf bytes.Buffer
	cfg := DefaultConfig()
	cfg.Output = &buf

	logger, err := New(cfg)
	if err != nil {
		t.Fatalf("New() error: %v", err)
	}
	defer func() { _ = logger.Close() }()

	err = logger.LogSync(context.Background(), validEvent())
	if err != nil {
		t.Fatalf("LogSync() error: %v", err)
	}

	if buf.Len() == 0 {
		t.Error("expected output from LogSync")
	}
}

type testMemoryStore struct {
	entries map[string]*AuditEntry
}

func (s *testMemoryStore) Store(entry *AuditEntry) error {
	s.entries[entry.ID] = entry
	return nil
}

func (s *testMemoryStore) Query(q Query) (*QueryResult, error) {
	entries := make([]AuditEntry, 0, len(s.entries))
	for _, e := range s.entries {
		entries = append(entries, *e)
	}
	return &QueryResult{Entries: entries, Total: int64(len(entries))}, nil
}

func (s *testMemoryStore) Get(id string) (*AuditEntry, error) {
	if e, ok := s.entries[id]; ok {
		return e, nil
	}
	return nil, ErrEntryNotFound
}

func (s *testMemoryStore) Close() error {
	return nil
}

func TestLoggerWithMemoryStore(t *testing.T) {
	var buf bytes.Buffer
	cfg := DefaultConfig()
	cfg.Output = &buf
	store := &testMemoryStore{
		entries: make(map[string]*AuditEntry),
	}
	cfg.Store = store

	logger, err := New(cfg)
	if err != nil {
		t.Fatalf("New() error: %v", err)
	}
	defer func() { _ = logger.Close() }()

	event := validEvent()
	if err := logger.LogSync(context.Background(), event); err != nil {
		t.Fatalf("LogSync() error: %v", err)
	}

	if len(store.entries) != 1 {
		t.Errorf("expected 1 stored entry, got %d", len(store.entries))
	}
}

func TestLoggerWithWAL(t *testing.T) {
	tmpDir := t.TempDir()
	var buf bytes.Buffer
	cfg := DefaultConfig()
	cfg.Output = &buf
	cfg.WAL = WALConfig{
		Enabled:     true,
		Path:        filepath.Join(tmpDir, "test.wal"),
		SyncOnWrite: true,
	}

	logger, err := New(cfg)
	if err != nil {
		t.Fatalf("New() error: %v", err)
	}

	if err := logger.LogSync(context.Background(), validEvent()); err != nil {
		t.Fatalf("LogSync() error: %v", err)
	}

	if err := logger.Close(); err != nil {
		t.Fatalf("Close() error: %v", err)
	}
}

func TestLoggerWithHashChain(t *testing.T) {
	var buf bytes.Buffer
	cfg := DefaultConfig()
	cfg.Output = &buf
	cfg.HashChain = HashChainConfig{
		Enabled:   true,
		Algorithm: "sha256",
	}

	logger, err := New(cfg)
	if err != nil {
		t.Fatalf("New() error: %v", err)
	}
	defer func() { _ = logger.Close() }()

	if err := logger.LogSync(context.Background(), validEvent()); err != nil {
		t.Fatalf("LogSync() error: %v", err)
	}

	if buf.Len() == 0 {
		t.Error("expected output")
	}
}

func TestLoggerRateLimited(t *testing.T) {
	var buf bytes.Buffer
	cfg := DefaultConfig()
	cfg.Output = &buf
	cfg.RateLimit = &RateLimitConfig{
		EventsPerSecond: 1,
		BurstSize:       1,
		DropWhenLimited: true,
	}

	logger, err := New(cfg)
	if err != nil {
		t.Fatalf("New() error: %v", err)
	}
	defer func() { _ = logger.Close() }()

	if err := logger.LogSync(context.Background(), validEvent()); err != nil {
		t.Fatalf("first LogSync() error: %v", err)
	}

	err = logger.LogSync(context.Background(), validEvent())
	if err != ErrRateLimited {
		t.Errorf("expected ErrRateLimited, got %v", err)
	}
}

func TestConfigValidation(t *testing.T) {
	tests := []struct {
		name    string
		cfg     Config
		wantErr bool
	}{
		{
			name:    "default config is valid",
			cfg:     DefaultConfig(),
			wantErr: false,
		},
		{
			name: "WAL enabled without path",
			cfg: func() Config {
				c := DefaultConfig()
				c.WAL.Enabled = true
				c.WAL.Path = ""
				return c
			}(),
			wantErr: true,
		},
		{
			name: "invalid sample rate",
			cfg: func() Config {
				c := DefaultConfig()
				c.SampleRate = 2.0
				return c
			}(),
			wantErr: true,
		},
		{
			name: "invalid rate limit",
			cfg: func() Config {
				c := DefaultConfig()
				c.RateLimit = &RateLimitConfig{EventsPerSecond: -1}
				return c
			}(),
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := tt.cfg.Validate()
			if (err != nil) != tt.wantErr {
				t.Errorf("Validate() error = %v, wantErr %v", err, tt.wantErr)
			}
		})
	}
}

func TestCompliancePresets(t *testing.T) {
	presets := []ComplianceStandard{
		ComplianceSOC2, ComplianceHIPAA, CompliancePCIDSS, ComplianceGDPR, ComplianceFedRAMP,
	}

	for _, preset := range presets {
		t.Run(string(preset), func(t *testing.T) {
			cfg := DefaultConfig()
			cfg.WithCompliance(preset)

			if cfg.Compliance != preset {
				t.Errorf("expected compliance %s, got %s", preset, cfg.Compliance)
			}
			if !cfg.HashChain.Enabled {
				t.Error("expected hash chain enabled for compliance preset")
			}
		})
	}
}

// TestLoggerGoroutineLifecycle verifies that the audit logger's background
// goroutine terminates after Close(), preventing goroutine leaks.
// Run with -artifacts flag to preserve test output, or with
// GOEXPERIMENT=goroutineleakprofile for detailed leak profiling.
func TestLoggerGoroutineLifecycle(t *testing.T) {
	var buf bytes.Buffer
	cfg := DefaultConfig()
	cfg.Output = &buf
	cfg.BufferSize = 10

	before := runtime.NumGoroutine()

	logger, err := New(cfg)
	if err != nil {
		t.Fatalf("New() error: %v", err)
	}

	// Logger should spawn a background goroutine
	afterStart := runtime.NumGoroutine()
	if afterStart <= before {
		t.Log("warning: goroutine count did not increase after New()")
	}

	// Log a few events
	for range 5 {
		_ = logger.Log(context.Background(), validEvent())
	}
	time.Sleep(50 * time.Millisecond)

	if err := logger.Close(); err != nil {
		t.Fatalf("Close() error: %v", err)
	}

	// Allow goroutines to wind down
	time.Sleep(100 * time.Millisecond)

	afterClose := runtime.NumGoroutine()
	leaked := afterClose - before
	if leaked > 1 {
		t.Errorf("potential goroutine leak: %d goroutines still running after Close() (before=%d, after=%d)",
			leaked, before, afterClose)
	}
}

// TestTypedErrorsWithAsType verifies that typed errors returned by the audit
// logger can be extracted using errors.AsType[T] (Go 1.26+).
func TestTypedErrorsWithAsType(t *testing.T) {
	sinkErr := &SinkError{SinkName: "file", Err: ErrSinkFailed}

	// Wrap in another error
	wrapped := fmt.Errorf("processing failed: %w", sinkErr)

	// Extract using errors.AsType
	if se, ok := errors.AsType[*SinkError](wrapped); ok {
		if se.SinkName != "file" {
			t.Errorf("SinkName = %q, want %q", se.SinkName, "file")
		}
	} else {
		t.Error("errors.AsType[*SinkError] should find the SinkError")
	}

	walErr := &WALError{Op: "write", Err: ErrWALCorrupted}
	wrapped = fmt.Errorf("audit failed: %w", walErr)

	if we, ok := errors.AsType[*WALError](wrapped); ok {
		if we.Op != "write" {
			t.Errorf("Op = %q, want %q", we.Op, "write")
		}
	} else {
		t.Error("errors.AsType[*WALError] should find the WALError")
	}

	storeErr := &StoreError{Op: "query", Err: ErrInvalidQuery}
	wrapped = fmt.Errorf("query failed: %w", storeErr)

	if se, ok := errors.AsType[*StoreError](wrapped); ok {
		if se.Op != "query" {
			t.Errorf("Op = %q, want %q", se.Op, "query")
		}
	} else {
		t.Error("errors.AsType[*StoreError] should find the StoreError")
	}
}
