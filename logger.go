package logger

import (
	"bytes"
	"context"
	"io"
	"log"
	"net/http"
)

type LogLevel int

const (
	Trace LogLevel = iota
	Debug
	Info
	Notice
	Warn
	Error
)

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
	// EnableColor: false is a valid value, so check with a pointer if you want to distinguish unset
	if cfg.TimeFormat == "" {
		cfg.TimeFormat = defaultConfig.TimeFormat
	}
	if cfg.RedactKeys == nil {
		cfg.RedactKeys = defaultConfig.RedactKeys
	}
	if cfg.RedactMask == "" {
		cfg.RedactMask = defaultConfig.RedactMask
	}
	config = cfg
	initLogger()
}

// GetConfig returns the current logger configuration.
func GetConfig() Config {
	return config
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

// Contextual Log function wrappers

// LogInfo logs an info message with optional key-value pairs
func LogInfoWithContext(ctx context.Context, message string, keyValues ...any) {
	// Extract trace ID, request ID from context if available
	if traceID := ctx.Value("trace_id"); traceID != nil {
		keyValues = append(keyValues, "trace_id", traceID)
	}
	logInternal(Info, message, keyValues...)
}

// LogHttpRequest logs details of an HTTP request
func LogHttpRequest(r *http.Request) {
	statusCode, logLevel := formatStatusCode(r.Response.StatusCode)
	// Build the full URL with query parameters
	fullPath := getFullPath(r.URL)
	endPoint := formatString(fullPath, cyan, false)
	// Format user agent
	userAgent := formatString(r.UserAgent(), purple, false)
	log.Printf("%s %s %s %s", statusCode, r.Method, endPoint, userAgent)
	// Read and log the body separately to avoid consuming it
	bodyBytes, err := io.ReadAll(r.Body)
	if err != nil {
		logInternal(Error, "Failed to read HTTP request body", "__error", err)
		return
	}
	r.Body = io.NopCloser(bytes.NewBuffer(bodyBytes))
	bodyKeyValues := bodyToKeyValues("body", bodyBytes) // Convert body to key-value pairs
	logInternal(logLevel, statusCode, bodyKeyValues...)
}
