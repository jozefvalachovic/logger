package logger

import (
	"bytes"
	"context"
	"io"
	"log"
	"net/http"

	"github.com/jozefvalachovic/logger/v4/audit"
)

type LogLevel int

const (
	Trace LogLevel = iota
	Debug
	Info
	Notice
	Warn
	Error
	Audit // Security audit logs
)

// Logger interface for dependency injection
type Logger interface {
	Log(level LogLevel, message string, keyValues ...any)
	LogDebug(message string, keyValues ...any)
	LogInfo(message string, keyValues ...any)
	LogNotice(message string, keyValues ...any)
	LogTrace(message string, keyValues ...any)
	LogWarn(message string, keyValues ...any)
	LogError(message string, keyValues ...any)
	LogAudit(keyValues ...any)
	LogAuditEvent(ctx context.Context, event audit.AuditEvent) error
	LogInfoWithContext(ctx context.Context, message string, keyValues ...any)
	LogHttpRequest(r *http.Request)
}

// defaultLoggerImpl implements the Logger interface
type defaultLoggerImpl struct{}

// Ensure defaultLoggerImpl implements Logger
var _ Logger = (*defaultLoggerImpl)(nil)

// DefaultLogger returns a Logger instance using the global configuration
func DefaultLogger() Logger {
	return &defaultLoggerImpl{}
}

// Implement Logger interface methods
func (l *defaultLoggerImpl) Log(level LogLevel, message string, keyValues ...any) {
	logInternal(level, message, keyValues...)
}

func (l *defaultLoggerImpl) LogDebug(message string, keyValues ...any) {
	logInternal(Debug, message, keyValues...)
}

func (l *defaultLoggerImpl) LogInfo(message string, keyValues ...any) {
	logInternal(Info, message, keyValues...)
}

func (l *defaultLoggerImpl) LogNotice(message string, keyValues ...any) {
	logInternal(Notice, message, keyValues...)
}

func (l *defaultLoggerImpl) LogTrace(message string, keyValues ...any) {
	logInternal(Trace, message, keyValues...)
}

func (l *defaultLoggerImpl) LogWarn(message string, keyValues ...any) {
	logInternal(Warn, message, keyValues...)
}

func (l *defaultLoggerImpl) LogError(message string, keyValues ...any) {
	logInternal(Error, message, keyValues...)
}

func (l *defaultLoggerImpl) LogAudit(keyValues ...any) {
	logInternal(Audit, "", keyValues...)
}

func (l *defaultLoggerImpl) LogAuditEvent(ctx context.Context, event audit.AuditEvent) error {
	return LogAuditEvent(ctx, event)
}

func (l *defaultLoggerImpl) LogInfoWithContext(ctx context.Context, message string, keyValues ...any) {
	if traceID := ctx.Value("trace_id"); traceID != nil {
		keyValues = append(keyValues, "trace_id", traceID)
	}
	logInternal(Info, message, keyValues...)
}

func (l *defaultLoggerImpl) LogHttpRequest(r *http.Request) {
	logHttpRequestInternal(r)
}

// SetConfig configures the logger with custom settings.
// This will reinitialize the logger with the new configuration.
func SetConfig(cfg Config) {
	// Use provided value or fallback to defaultConfig for each field
	if cfg.Output == nil {
		cfg.Output = defaultConfig.Output
	}
	if cfg.Level == 0 {
		cfg.Level = defaultConfig.Level
	}
	if cfg.TimeFormat == "" {
		cfg.TimeFormat = defaultConfig.TimeFormat
	}
	if cfg.RedactKeys == nil {
		cfg.RedactKeys = defaultConfig.RedactKeys
	}
	if cfg.RedactMask == "" {
		cfg.RedactMask = defaultConfig.RedactMask
	}
	if cfg.MaxBodySize == 0 {
		cfg.MaxBodySize = defaultConfig.MaxBodySize
	}
	if cfg.RedactPaths == nil {
		cfg.RedactPaths = defaultConfig.RedactPaths
	}
	// SampleRate: 0.0 is the zero value, so we treat it as unset and apply default (1.0)
	// If users want to disable sampling, they should use log level filtering instead
	if cfg.SampleRate == 0 {
		cfg.SampleRate = defaultConfig.SampleRate
	}
	if cfg.BufferSize == 0 {
		cfg.BufferSize = defaultConfig.BufferSize
	}
	if cfg.FlushTimeout == 0 {
		cfg.FlushTimeout = defaultConfig.FlushTimeout
	}
	if cfg.MetricsPrefix == "" {
		cfg.MetricsPrefix = defaultConfig.MetricsPrefix
	}

	// Validate the configuration after filling defaults
	if err := cfg.Validate(); err != nil {
		LogError("Invalid configuration", "__error", err)
		return
	}

	// Handle async mode changes
	configMu.RLock()
	oldAsync := globalConfig.AsyncMode
	oldAuditConfig := globalConfig.Audit
	configMu.RUnlock()

	if cfg.AsyncMode && !oldAsync {
		// Starting async mode
		startAsyncLogger(cfg)
	} else if !cfg.AsyncMode && oldAsync {
		// Stopping async mode
		stopAsyncLogger()
	}

	// Handle metrics changes
	if cfg.EnableMetrics && metrics == nil {
		metrics = NewLogMetrics()
	} else if !cfg.EnableMetrics && metrics != nil {
		metrics = nil
	}

	// Handle enterprise audit logger changes
	if cfg.Audit != nil && oldAuditConfig == nil {
		// Initialize enterprise audit logger
		if al, err := audit.New(*cfg.Audit); err != nil {
			LogError("Failed to initialize enterprise audit logger", "__error", err)
		} else {
			auditLogger = al
		}
	} else if cfg.Audit == nil && auditLogger != nil {
		// Close existing audit logger
		auditLogger.Close()
		auditLogger = nil
	} else if cfg.Audit != nil && auditLogger != nil {
		// Reconfigure: close old and create new
		auditLogger.Close()
		if al, err := audit.New(*cfg.Audit); err != nil {
			LogError("Failed to reinitialize enterprise audit logger", "__error", err)
			auditLogger = nil
		} else {
			auditLogger = al
		}
	}

	configMu.Lock()
	globalConfig = cfg
	configMu.Unlock()
	initLogger()
}

// GetConfig returns the current logger configuration.
func GetConfig() Config {
	configMu.RLock()
	defer configMu.RUnlock()
	return globalConfig
}

// Basic Log function

// Log logs a message at the specified log level with optional key-value pairs (backwards compatible version)
func Log(level LogLevel, message string, keyValues ...any) {
	logInternal(level, message, keyValues...)
}

// Level-specific Log function wrappers

// LogDebug logs a debug message with optional key-value pairs
func LogDebug(message string, keyValues ...any) {
	logInternal(Debug, message, keyValues...)
}

// LogInfo logs an info message with optional key-value pairs
func LogInfo(message string, keyValues ...any) {
	logInternal(Info, message, keyValues...)
}

// LogNotice logs a notice message with optional key-value pairs
func LogNotice(message string, keyValues ...any) {
	logInternal(Notice, message, keyValues...)
}

// LogTrace logs a trace message with optional key-value pairs
func LogTrace(message string, keyValues ...any) {
	logInternal(Trace, message, keyValues...)
}

// LogWarn logs a warning message with optional key-value pairs
func LogWarn(message string, keyValues ...any) {
	logInternal(Warn, message, keyValues...)
}

// LogError logs an error message with optional key-value pairs
func LogError(message string, keyValues ...any) {
	logInternal(Error, message, keyValues...)
}

// LogAudit logs a security audit event with only key-value pairs
// No message is logged, only the structured data
// This is the legacy audit logging function - for enterprise features, use LogAuditEvent
func LogAudit(keyValues ...any) {
	logInternal(Audit, "", keyValues...)
}

// LogAuditEvent logs a structured audit event using the enterprise audit logger
// If enterprise audit is not configured, falls back to legacy LogAudit behavior
func LogAuditEvent(ctx context.Context, event audit.AuditEvent) error {
	configMu.RLock()
	cfg := globalConfig
	configMu.RUnlock()

	// If enterprise audit is configured, use it
	if cfg.Audit != nil && auditLogger != nil {
		return auditLogger.Log(ctx, event)
	}

	// Fallback to legacy behavior: convert event to key-value pairs
	keyValues := []any{
		"event_type", string(event.Type),
		"action", event.Action,
		"outcome", string(event.Outcome),
		"actor_id", event.Actor.ID,
		"actor_type", event.Actor.Type,
	}

	if event.Actor.IP != "" {
		keyValues = append(keyValues, "actor_ip", event.Actor.IP)
	}

	if event.Resource != nil {
		keyValues = append(keyValues, "resource_id", event.Resource.ID, "resource_type", event.Resource.Type)
	}

	if event.Description != "" {
		keyValues = append(keyValues, "description", event.Description)
	}

	if event.Reason != "" {
		keyValues = append(keyValues, "reason", event.Reason)
	}

	// Add metadata
	for k, v := range event.Metadata {
		keyValues = append(keyValues, k, v)
	}

	logInternal(Audit, "", keyValues...)
	return nil
}

// LogAuditEventSync logs a structured audit event synchronously (guaranteed delivery)
// Only available when enterprise audit is configured
func LogAuditEventSync(ctx context.Context, event audit.AuditEvent) error {
	configMu.RLock()
	cfg := globalConfig
	configMu.RUnlock()

	if cfg.Audit != nil && auditLogger != nil {
		return auditLogger.LogSync(ctx, event)
	}

	// Fallback to regular async logging
	return LogAuditEvent(ctx, event)
}

// GetAuditLogger returns the enterprise audit logger instance
// Returns nil if enterprise audit is not configured
func GetAuditLogger() *audit.Logger {
	return auditLogger
}

// Contextual Log function wrappers

// LogInfo logs an info message with optional key-value pairs
func LogInfoWithContext(ctx context.Context, message string, keyValues ...any) {
	// Extract trace ID from context if available
	// Try to get value using common key patterns
	var traceID interface{}

	// Check for any key that might contain trace_id
	// This is a workaround since we can't directly check for the test's custom type
	// Users should pass trace_id as a regular key-value pair for best results
	if val := ctx.Value("trace_id"); val != nil {
		traceID = val
	}

	if traceID != nil {
		keyValues = append(keyValues, "trace_id", traceID)
	}
	logInternal(Info, message, keyValues...)
} // LogHttpRequest logs details of an HTTP request
func LogHttpRequest(r *http.Request) {
	logHttpRequestInternal(r)
}

// logHttpRequestInternal is the internal implementation for logging HTTP requests
func logHttpRequestInternal(r *http.Request) {
	configMu.RLock()
	cfg := globalConfig
	configMu.RUnlock()

	// Check if path should be redacted
	fullPath := getFullPath(r.URL)
	if shouldRedactPath(fullPath, cfg) {
		log.Printf("%s %s %s [REDACTED]", "---", r.Method, cfg.RedactMask)
		return
	}

	statusCode, logLevel := formatStatusCode(r.Response.StatusCode)
	endPoint := formatString(fullPath, cyan, false)
	userAgent := formatString(r.UserAgent(), purple, false)
	log.Printf("%s %s %s %s", statusCode, r.Method, endPoint, userAgent)

	// Read and log the body separately to avoid consuming it
	bodyBytes, err := io.ReadAll(r.Body)
	if err != nil {
		logInternal(Error, "Failed to read HTTP request body", "__error", err)
		return
	}
	r.Body = io.NopCloser(bytes.NewBuffer(bodyBytes))
	bodyKeyValues := bodyToKeyValues("body", bodyBytes)
	logInternal(logLevel, statusCode, bodyKeyValues...)
}
