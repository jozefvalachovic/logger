package audit

import "errors"

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
