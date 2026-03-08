package audit

import (
	"errors"
	"fmt"
)

// Validation errors
var (
	ErrMissingEventType = errors.New("audit: event type is required")
	ErrMissingAction    = errors.New("audit: action is required")
	ErrMissingActor     = errors.New("audit: actor ID or type is required")
)

// Configuration errors
var (
	ErrInvalidConfig     = errors.New("audit: invalid configuration")
	ErrWALPathRequired   = errors.New("audit: WAL path required when WAL is enabled")
	ErrNoSinksConfigured = errors.New("audit: no sinks configured")
	ErrInvalidSampleRate = errors.New("audit: sample rate must be between 0.0 and 1.0")
	ErrInvalidRateLimit  = errors.New("audit: rate limit must be positive")
)

// Runtime errors
var (
	ErrAuditLoggerClosed = errors.New("audit: logger is closed")
	ErrWALCorrupted      = errors.New("audit: WAL file is corrupted")
	ErrHashChainBroken   = errors.New("audit: hash chain integrity check failed")
	ErrSinkFailed        = errors.New("audit: sink write failed")
	ErrRateLimited       = errors.New("audit: rate limit exceeded")
	ErrQueryTimeout      = errors.New("audit: query timeout exceeded")
)

// Store errors
var (
	ErrStoreNotConfigured = errors.New("audit: store not configured for queries")
	ErrEntryNotFound      = errors.New("audit: entry not found")
	ErrInvalidQuery       = errors.New("audit: invalid query parameters")
)

// Typed error wrappers for use with errors.AsType[T] (Go 1.26+).

// SinkError represents an error originating from an audit sink.
type SinkError struct {
	SinkName string
	Err      error
}

func (e *SinkError) Error() string { return fmt.Sprintf("audit sink %q: %v", e.SinkName, e.Err) }
func (e *SinkError) Unwrap() error { return e.Err }

// WALError represents an error originating from the write-ahead log.
type WALError struct {
	Op  string // operation that failed (e.g., "write", "commit", "recover")
	Err error
}

func (e *WALError) Error() string { return fmt.Sprintf("audit wal %s: %v", e.Op, e.Err) }
func (e *WALError) Unwrap() error { return e.Err }

// StoreError represents an error originating from the audit store.
type StoreError struct {
	Op  string // operation that failed (e.g., "store", "query", "get")
	Err error
}

func (e *StoreError) Error() string { return fmt.Sprintf("audit store %s: %v", e.Op, e.Err) }
func (e *StoreError) Unwrap() error { return e.Err }
