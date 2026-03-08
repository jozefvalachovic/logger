package middleware

import (
	"bytes"
	"context"
	"crypto/rand"
	"encoding/hex"
	"fmt"
	"net/http"
	"slices"
	"strings"
	"sync"
	"time"

	"github.com/jozefvalachovic/logger/v4"
	"github.com/jozefvalachovic/logger/v4/audit"
)

// Context keys for request metadata
type contextKey string

const (
	// RequestIDKey is the context key for request ID
	RequestIDKey contextKey = "request_id"
	// RequestStartKey is the context key for request start time
	RequestStartKey contextKey = "request_start"
)

// wrappedWriter is used to capture the status code and response body of HTTP responses
type wrappedWriter struct {
	http.ResponseWriter
	statusCode      int
	responseBody    *bytes.Buffer
	captureBody     bool
	maxCaptureBytes int64 // Maximum bytes to capture for response body
	capturedBytes   int64
}

// WriteHeader captures the status code for logging
func (w *wrappedWriter) WriteHeader(statusCode int) {
	w.ResponseWriter.WriteHeader(statusCode)
	w.statusCode = statusCode
}

// Write captures the response body if enabled, up to maxCaptureBytes
func (w *wrappedWriter) Write(b []byte) (int, error) {
	if w.captureBody && w.responseBody != nil && w.capturedBytes < w.maxCaptureBytes {
		remaining := w.maxCaptureBytes - w.capturedBytes
		toCapture := min(int64(len(b)), remaining)
		w.responseBody.Write(b[:toCapture])
		w.capturedBytes += toCapture
	}
	return w.ResponseWriter.Write(b)
}

// Flush ensures that the underlying ResponseWriter's Flush method is called if it exists
func (w *wrappedWriter) Flush() {
	if flusher, ok := w.ResponseWriter.(http.Flusher); ok {
		flusher.Flush()
	}
}

// Optional: ensure at compile time that wrappedWriter implements http.Flusher
var _ http.Flusher = (*wrappedWriter)(nil)

// Pools for memory optimization
var (
	wrappedWriterPool = sync.Pool{
		New: func() any {
			return new(wrappedWriter{statusCode: http.StatusOK})
		},
	}

	bufferPool = sync.Pool{
		New: func() any {
			return new(bytes.Buffer)
		},
	}
)

// shouldLogBody checks if the content type is appropriate for logging
func shouldLogBody(contentType string) bool {
	contentType = strings.ToLower(contentType)
	// Log only text-based content types
	allowedTypes := []string{
		"application/json",
		"application/xml",
		"text/",
		"application/x-www-form-urlencoded",
	}

	for _, allowed := range allowedTypes {
		if strings.Contains(contentType, allowed) {
			return true
		}
	}
	return false
}

// generateRequestID generates a random request ID
func generateRequestID() string {
	b := make([]byte, 16)
	if _, err := rand.Read(b); err != nil {
		// Fallback to timestamp-based ID if crypto/rand fails
		return fmt.Sprintf("%d", time.Now().UnixNano())
	}
	return hex.EncodeToString(b)
}

// GetRequestID extracts the request ID from context
func GetRequestID(ctx context.Context) string {
	if id, ok := ctx.Value(RequestIDKey).(string); ok {
		return id
	}
	return ""
}

// GetRequestStart extracts the request start time from context
func GetRequestStart(ctx context.Context) time.Time {
	if t, ok := ctx.Value(RequestStartKey).(time.Time); ok {
		return t
	}
	return time.Time{}
}

// shouldSkipPath checks if a path should be skipped from logging
func shouldSkipPath(path string, options *HTTPMiddlewareOptions) bool {
	// Check exact matches
	if slices.Contains(options.SkipPaths, path) {
		return true
	}
	// Check prefix matches
	for _, prefix := range options.SkipPathPrefixes {
		if strings.HasPrefix(path, prefix) {
			return true
		}
	}
	return false
}

// getLogLevelForStatus determines the log level based on status code and options
func getLogLevelForStatus(statusCode int, options *HTTPMiddlewareOptions) logger.LogLevel {
	// Check for exact status code match first
	if options.LogLevelByStatus != nil {
		if level, ok := options.LogLevelByStatus[statusCode]; ok {
			return level
		}
		// Check for status code class (e.g., 400 for all 4xx, 500 for all 5xx)
		statusClass := (statusCode / 100) * 100
		if level, ok := options.LogLevelByStatus[statusClass]; ok {
			return level
		}
	}

	// Default behavior based on status code
	switch {
	case statusCode >= 500:
		return logger.Error
	case statusCode >= 400:
		return logger.Warn
	case statusCode >= 300:
		return logger.Info
	default:
		return logger.Info
	}
}

// shouldAuditRequest checks if a request method should be audited
func shouldAuditRequest(method string, options *HTTPMiddlewareOptions) bool {
	if len(options.AuditEventTypes) == 0 {
		return true // Audit all methods if not specified
	}
	for _, m := range options.AuditEventTypes {
		if strings.EqualFold(m, method) {
			return true
		}
	}
	return false
}

// emitAuditEvent emits an audit event for an HTTP request
func emitAuditEvent(r *http.Request, statusCode int, duration time.Duration, requestID, fullPath string) {
	outcome := audit.OutcomeSuccess
	if statusCode >= 400 {
		outcome = audit.OutcomeFailure
	}
	if statusCode == 401 || statusCode == 403 {
		outcome = audit.OutcomeDenied
	}

	event := audit.AuditEvent{
		Type:    audit.AuditAPIAccess,
		Action:  r.Method + " " + fullPath,
		Outcome: outcome,
		Actor: audit.AuditActor{
			IP:        getClientIP(r),
			UserAgent: r.UserAgent(),
		},
		Resource: &audit.AuditResource{
			ID:   fullPath,
			Type: "http_endpoint",
		},
		Metadata: map[string]any{
			"method":      r.Method,
			"path":        fullPath,
			"status_code": statusCode,
			"duration_ms": duration.Milliseconds(),
			"request_id":  requestID,
		},
	}

	// Extract user ID from context if available
	if userID := r.Context().Value("user_id"); userID != nil {
		if id, ok := userID.(string); ok {
			event.Actor.ID = id
			event.Actor.Type = "user"
		} else {
			event.Actor.ID = fmt.Sprintf("%v", userID)
			event.Actor.Type = "user"
		}
	} else {
		event.Actor.Type = "anonymous"
	}

	_ = logger.LogAuditEvent(r.Context(), event)
}

// getClientIP extracts the client IP from a request.
// WARNING: X-Forwarded-For and X-Real-IP headers can be spoofed by clients.
// Only trust these headers when the request comes through a trusted reverse proxy.
// In production, configure your proxy to overwrite (not append) these headers.
func getClientIP(r *http.Request) string {
	// Check X-Forwarded-For header (for proxied requests)
	// Note: Only the rightmost IP added by a trusted proxy is reliable.
	// The leftmost IP is client-supplied and may be forged.
	if xff := r.Header.Get("X-Forwarded-For"); xff != "" {
		ips := strings.Split(xff, ",")
		if len(ips) > 0 {
			return strings.TrimSpace(ips[0])
		}
	}
	// Check X-Real-IP header
	if xri := r.Header.Get("X-Real-IP"); xri != "" {
		return xri
	}
	// Fall back to RemoteAddr
	ip := r.RemoteAddr
	if idx := strings.LastIndex(ip, ":"); idx != -1 {
		ip = ip[:idx]
	}
	return ip
}

// logErrorDetails logs detailed error information for failed requests
func logErrorDetails(r *http.Request, wrapped *wrappedWriter, options *HTTPMiddlewareOptions,
	bodyBytes []byte, bodyErr error, truncated bool, fullPath, requestID string, cfg logger.Config) {

	contentType := r.Header.Get("Content-Type")
	shouldLog := shouldLogBody(contentType)

	if !options.LogBodyOnErrors && !options.LogResponseBody {
		return
	}

	// Check if path should be redacted
	logPath := fullPath
	if logger.ShouldRedactPath(fullPath, cfg) {
		logPath = cfg.RedactMask
	}

	keyValues := []any{
		"__method", r.Method,
		"__path", logPath,
		"__status", wrapped.statusCode,
	}

	if requestID != "" {
		keyValues = append(keyValues, "request_id", requestID)
	}

	// Add custom fields
	for k, v := range options.CustomFields {
		keyValues = append(keyValues, k, v)
	}

	// Log request body
	if options.LogBodyOnErrors && shouldLog {
		if bodyErr != nil {
			logger.LogError("Failed to read HTTP request body for error logging", "__error", bodyErr)
		} else if len(bodyBytes) > 0 {
			if truncated {
				bodyStr := string(bodyBytes) + "..."
				keyValues = append(keyValues, "request_body", bodyStr)
			} else {
				bodyKeyValues := logger.BodyToKeyValues("request_body", bodyBytes)
				keyValues = append(keyValues, bodyKeyValues...)
			}
		}
	}

	// Log response body
	if options.LogResponseBody && wrapped.responseBody != nil && wrapped.responseBody.Len() > 0 {
		respContentType := wrapped.Header().Get("Content-Type")
		if shouldLogBody(respContentType) {
			// Use Peek to preview response body without consuming the buffer
			peekSize := min(int64(wrapped.responseBody.Len()), cfg.MaxBodySize)
			respBody, _ := wrapped.responseBody.Peek(int(peekSize))
			if int64(wrapped.responseBody.Len()) > cfg.MaxBodySize {
				keyValues = append(keyValues, "response_body", string(respBody)+"...")
			} else {
				respKeyValues := logger.BodyToKeyValues("response_body", respBody)
				keyValues = append(keyValues, respKeyValues...)
			}
		}
	}

	logger.LogError("Failed Request", keyValues...)
}
