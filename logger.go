package logger

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"time"

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

// loggerCtxKey is the private context key used to store a Logger in a context.Context.
type loggerCtxKey struct{}

// NewContext returns a copy of ctx that carries the given Logger.
//
// Use this in middleware to store an enriched logger that downstream handlers
// can retrieve with FromContext:
//
//	child := logger.DefaultLogger().With("requestId", reqID)
//	ctx = logger.NewContext(ctx, child)
func NewContext(ctx context.Context, l Logger) context.Context {
	return context.WithValue(ctx, loggerCtxKey{}, l)
}

// FromContext extracts the Logger stored by NewContext.
// If no Logger is found, it returns DefaultLogger() (never nil).
//
//	l := logger.FromContext(r.Context())
//	l.LogInfo("handling request")
func FromContext(ctx context.Context) Logger {
	if l, ok := ctx.Value(loggerCtxKey{}).(Logger); ok {
		return l
	}
	return DefaultLogger()
}

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
	LogWithContext(ctx context.Context, level LogLevel, message string, keyValues ...any)
	LogDebugWithContext(ctx context.Context, message string, keyValues ...any)
	LogTraceWithContext(ctx context.Context, message string, keyValues ...any)
	LogNoticeWithContext(ctx context.Context, message string, keyValues ...any)
	LogWarnWithContext(ctx context.Context, message string, keyValues ...any)
	LogErrorWithContext(ctx context.Context, message string, keyValues ...any)
	LogHttpRequest(r *http.Request)
	With(keyValues ...any) Logger
	LogErrorWithStack(err error, msg string, keyValues ...any)
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
	LogInfoWithContext(ctx, message, keyValues...)
}

func (l *defaultLoggerImpl) LogWithContext(ctx context.Context, level LogLevel, message string, keyValues ...any) {
	FromContext(ctx).Log(level, message, keyValues...)
}

func (l *defaultLoggerImpl) LogDebugWithContext(ctx context.Context, message string, keyValues ...any) {
	FromContext(ctx).LogDebug(message, keyValues...)
}

func (l *defaultLoggerImpl) LogTraceWithContext(ctx context.Context, message string, keyValues ...any) {
	FromContext(ctx).LogTrace(message, keyValues...)
}

func (l *defaultLoggerImpl) LogNoticeWithContext(ctx context.Context, message string, keyValues ...any) {
	FromContext(ctx).LogNotice(message, keyValues...)
}

func (l *defaultLoggerImpl) LogWarnWithContext(ctx context.Context, message string, keyValues ...any) {
	FromContext(ctx).LogWarn(message, keyValues...)
}

func (l *defaultLoggerImpl) LogErrorWithContext(ctx context.Context, message string, keyValues ...any) {
	FromContext(ctx).LogError(message, keyValues...)
}

func (l *defaultLoggerImpl) LogHttpRequest(r *http.Request) {
	logHttpRequestInternal(r)
}

func (l *defaultLoggerImpl) With(keyValues ...any) Logger {
	return &childLogger{fields: keyValues}
}

func (l *defaultLoggerImpl) LogErrorWithStack(err error, msg string, keyValues ...any) {
	logErrorWithStackInternal(err, msg, keyValues...)
}

// childLogger is a logger with pre-set fields prepended to every log call.
type childLogger struct {
	fields []any
}

var _ Logger = (*childLogger)(nil)

func mergeKV(base []any, extra ...any) []any {
	merged := make([]any, 0, len(base)+len(extra))
	merged = append(merged, base...)
	merged = append(merged, extra...)
	return merged
}

func (l *childLogger) Log(level LogLevel, message string, keyValues ...any) {
	logInternal(level, message, mergeKV(l.fields, keyValues...)...)
}

func (l *childLogger) LogDebug(message string, keyValues ...any) {
	logInternal(Debug, message, mergeKV(l.fields, keyValues...)...)
}

func (l *childLogger) LogInfo(message string, keyValues ...any) {
	logInternal(Info, message, mergeKV(l.fields, keyValues...)...)
}

func (l *childLogger) LogNotice(message string, keyValues ...any) {
	logInternal(Notice, message, mergeKV(l.fields, keyValues...)...)
}

func (l *childLogger) LogTrace(message string, keyValues ...any) {
	logInternal(Trace, message, mergeKV(l.fields, keyValues...)...)
}

func (l *childLogger) LogWarn(message string, keyValues ...any) {
	logInternal(Warn, message, mergeKV(l.fields, keyValues...)...)
}

func (l *childLogger) LogError(message string, keyValues ...any) {
	logInternal(Error, message, mergeKV(l.fields, keyValues...)...)
}

func (l *childLogger) LogAudit(keyValues ...any) {
	logInternal(Audit, "", mergeKV(l.fields, keyValues...)...)
}

func (l *childLogger) LogAuditEvent(ctx context.Context, event audit.AuditEvent) error {
	return LogAuditEvent(ctx, event)
}

func (l *childLogger) LogInfoWithContext(ctx context.Context, message string, keyValues ...any) {
	LogInfoWithContext(ctx, message, mergeKV(l.fields, keyValues...)...)
}

func (l *childLogger) LogWithContext(ctx context.Context, level LogLevel, message string, keyValues ...any) {
	FromContext(ctx).Log(level, message, mergeKV(l.fields, keyValues...)...)
}

func (l *childLogger) LogDebugWithContext(ctx context.Context, message string, keyValues ...any) {
	FromContext(ctx).LogDebug(message, mergeKV(l.fields, keyValues...)...)
}

func (l *childLogger) LogTraceWithContext(ctx context.Context, message string, keyValues ...any) {
	FromContext(ctx).LogTrace(message, mergeKV(l.fields, keyValues...)...)
}

func (l *childLogger) LogNoticeWithContext(ctx context.Context, message string, keyValues ...any) {
	FromContext(ctx).LogNotice(message, mergeKV(l.fields, keyValues...)...)
}

func (l *childLogger) LogWarnWithContext(ctx context.Context, message string, keyValues ...any) {
	FromContext(ctx).LogWarn(message, mergeKV(l.fields, keyValues...)...)
}

func (l *childLogger) LogErrorWithContext(ctx context.Context, message string, keyValues ...any) {
	FromContext(ctx).LogError(message, mergeKV(l.fields, keyValues...)...)
}

func (l *childLogger) LogHttpRequest(r *http.Request) {
	logHttpRequestInternal(r)
}

func (l *childLogger) With(keyValues ...any) Logger {
	return &childLogger{fields: mergeKV(l.fields, keyValues...)}
}

func (l *childLogger) LogErrorWithStack(err error, msg string, keyValues ...any) {
	logErrorWithStackInternal(err, msg, mergeKV(l.fields, keyValues...)...)
}

// With creates a child logger with pre-set key-value fields.
func With(keyValues ...any) Logger {
	return &childLogger{fields: keyValues}
}

// LogErrorWithStack logs an error with type information and stack trace.
func LogErrorWithStack(err error, msg string, keyValues ...any) {
	logErrorWithStackInternal(err, msg, keyValues...)
}

func logErrorWithStackInternal(err error, msg string, keyValues ...any) {
	kv := []any{
		"error", err.Error(),
		"error_type", fmt.Sprintf("%T", err),
	}
	var chain []string
	for ue := errors.Unwrap(err); ue != nil; ue = errors.Unwrap(ue) {
		chain = append(chain, ue.Error())
	}
	if len(chain) > 0 {
		kv = append(kv, "error_chain", chain)
	}
	kv = append(kv, "stack", GetStackTrace())
	kv = append(kv, keyValues...)
	logInternal(Error, msg, kv...)
}

// IfTrace calls fn only if the current log level would output Trace messages.
func IfTrace(fn func()) {
	level := globalConfig.Load().Level
	if level <= LevelTrace {
		fn()
	}
}

// IfDebug calls fn only if the current log level would output Debug messages.
func IfDebug(fn func()) {
	level := globalConfig.Load().Level
	if level <= slog.LevelDebug {
		fn()
	}
}

// IfInfo calls fn only if the current log level would output Info messages.
func IfInfo(fn func()) {
	level := globalConfig.Load().Level
	if level <= slog.LevelInfo {
		fn()
	}
}

// IfWarn calls fn only if the current log level would output Warn messages.
func IfWarn(fn func()) {
	level := globalConfig.Load().Level
	if level <= slog.LevelWarn {
		fn()
	}
}

// IfError calls fn only if the current log level would output Error messages.
func IfError(fn func()) {
	level := globalConfig.Load().Level
	if level <= slog.LevelError {
		fn()
	}
}

// SetConfig configures the logger with custom settings.
// This will reinitialize the logger with the new configuration.
func SetConfig(cfg Config) {
	// Use provided value or fallback to defaultConfig for each field
	if cfg.Output == nil {
		cfg.Output = defaultConfig.Output
	}
	if cfg.Level == 0 && !cfg.LevelSet {
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
	// SampleRate: use SampleRateSet to distinguish "not specified" from "explicitly set to 0"
	if !cfg.SampleRateSet && cfg.SampleRate == 0 {
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
	configWriteMu.Lock()
	oldCfg := globalConfig.Load()
	oldAsync := oldCfg.AsyncMode
	oldAuditConfig := oldCfg.Audit

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

	// Handle dedup changes
	if cfg.EnableDedup && dedupMgr == nil {
		window := cfg.DedupWindow
		if window == 0 {
			window = 5 * time.Second
		}
		startDedup(window)
	} else if !cfg.EnableDedup && dedupMgr != nil {
		stopDedup()
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
		_ = auditLogger.Close()
		auditLogger = nil
	} else if cfg.Audit != nil && auditLogger != nil {
		// Reconfigure: close old and create new
		_ = auditLogger.Close()
		if al, err := audit.New(*cfg.Audit); err != nil {
			LogError("Failed to reinitialize enterprise audit logger", "__error", err)
			auditLogger = nil
		} else {
			auditLogger = al
		}
	}

	globalConfig.Store(&cfg)
	configWriteMu.Unlock()
	initLogger()
}

// GetConfig returns the current logger configuration.
func GetConfig() Config {
	return *globalConfig.Load()
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
	cfg := *globalConfig.Load()

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
	cfg := *globalConfig.Load()

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

// TraceIDKey is the typed context key for trace ID extraction.
// Use this type when storing trace IDs in context to ensure they are extracted by LogInfoWithContext.
type traceIDKeyType struct{}

// TraceIDContextKey is the context key for trace IDs used by LogInfoWithContext.
var TraceIDContextKey = traceIDKeyType{}

// Contextual Log function wrappers

// Deprecated: LogInfoWithContext extracts trace_id from ctx for backward compatibility.
// Prefer storing an enriched logger via NewContext and using LogWithContext / FromContext instead.
func LogInfoWithContext(ctx context.Context, message string, keyValues ...any) {
	// Extract trace ID from context using the typed key first, then fall back to string key
	var traceID any

	if val := ctx.Value(TraceIDContextKey); val != nil {
		traceID = val
	} else if val := ctx.Value("trace_id"); val != nil {
		traceID = val
	}

	if traceID != nil {
		keyValues = append(keyValues, "trace_id", traceID)
	}
	logInternal(Info, message, keyValues...)
}

// LogWithContext retrieves the Logger from ctx (see NewContext / FromContext) and logs at the given level.
func LogWithContext(ctx context.Context, level LogLevel, message string, keyValues ...any) {
	FromContext(ctx).Log(level, message, keyValues...)
}

// LogDebugWithContext retrieves the Logger from ctx and logs at Debug level.
func LogDebugWithContext(ctx context.Context, message string, keyValues ...any) {
	FromContext(ctx).LogDebug(message, keyValues...)
}

// LogTraceWithContext retrieves the Logger from ctx and logs at Trace level.
func LogTraceWithContext(ctx context.Context, message string, keyValues ...any) {
	FromContext(ctx).LogTrace(message, keyValues...)
}

// LogNoticeWithContext retrieves the Logger from ctx and logs at Notice level.
func LogNoticeWithContext(ctx context.Context, message string, keyValues ...any) {
	FromContext(ctx).LogNotice(message, keyValues...)
}

// LogWarnWithContext retrieves the Logger from ctx and logs at Warn level.
func LogWarnWithContext(ctx context.Context, message string, keyValues ...any) {
	FromContext(ctx).LogWarn(message, keyValues...)
}

// LogErrorWithContext retrieves the Logger from ctx and logs at Error level.
func LogErrorWithContext(ctx context.Context, message string, keyValues ...any) {
	FromContext(ctx).LogError(message, keyValues...)
}

// LogHttpRequest logs details of an HTTP request
func LogHttpRequest(r *http.Request) {
	logHttpRequestInternal(r)
}

// logHttpRequestInternal is the internal implementation for logging HTTP requests
func logHttpRequestInternal(r *http.Request) {
	cfg := *globalConfig.Load()

	// Check if path should be redacted
	fullPath := getFullPath(r.URL)
	logPath := fullPath
	if shouldRedactPath(fullPath, cfg) {
		logPath = cfg.RedactMask
	}

	// Get status code and determine log level
	statusCode := 0
	if r.Response != nil {
		statusCode = r.Response.StatusCode
	}
	_, logLevel := formatStatusCode(statusCode)

	// Build key-value pairs
	keyValues := []any{
		"__method", r.Method,
		"__path", logPath,
		"__status", statusCode,
		"__user_agent", r.UserAgent(),
	}

	// Read and log the body if present
	if r.Body != nil {
		bodyBytes, err := io.ReadAll(io.LimitReader(r.Body, cfg.MaxBodySize))
		if err != nil {
			logInternal(Error, "Failed to read HTTP request body", "__error", err)
			return
		}
		r.Body = io.NopCloser(bytes.NewBuffer(bodyBytes))
		if len(bodyBytes) > 0 {
			bodyKeyValues := bodyToKeyValues("body", bodyBytes)
			keyValues = append(keyValues, bodyKeyValues...)
		}
	}

	// Log with key details in the message
	logMsg := fmt.Sprintf("%s %s [%d]", r.Method, logPath, statusCode)
	logInternal(logLevel, logMsg, keyValues...)
}
