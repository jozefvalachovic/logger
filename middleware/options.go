package middleware

import (
	"net/http"
	"time"

	"github.com/jozefvalachovic/logger/v4"
)

// HTTPMiddlewareOptions configures the HTTP logging middleware
type HTTPMiddlewareOptions struct {
	// LogBodyOnErrors logs request body when status is 4xx/5xx
	LogBodyOnErrors bool
	// LogResponseBody logs response body on errors
	LogResponseBody bool
	// EnableRequestID generates or extracts request IDs
	EnableRequestID bool
	// RequestIDHeader is the header to extract/set request ID (default: X-Request-ID)
	RequestIDHeader string
	// EnableAudit emits audit events for requests
	EnableAudit bool
	// AuditEventTypes specifies which request types to audit (nil = all)
	AuditEventTypes []string // e.g., ["POST", "PUT", "DELETE"]
	// SkipPaths excludes paths from logging (exact match or prefix)
	SkipPaths []string
	// SkipPathPrefixes excludes paths starting with these prefixes
	SkipPathPrefixes []string
	// LogLevelByStatus maps status code ranges to log levels
	LogLevelByStatus map[int]logger.LogLevel
	// CustomFields adds custom fields to every log entry
	CustomFields map[string]any
	// EnableMetrics enables request metrics collection
	EnableMetrics bool
	// MetricsCollector is an optional custom metrics collector
	MetricsCollector MetricsCollector
	// OnRequestStart callback before request processing
	OnRequestStart func(r *http.Request)
	// OnRequestEnd callback after request processing
	OnRequestEnd func(r *http.Request, statusCode int, duration time.Duration)
	// BodySampleRate samples request bodies for a percentage of all requests (0.0-1.0)
	// When > 0, bodies are captured and logged even for successful requests.
	BodySampleRate float64
}

// HTTPMiddlewareOption is a functional option for configuring middleware
type HTTPMiddlewareOption func(*HTTPMiddlewareOptions)

// DefaultHTTPMiddlewareOptions returns the default options
func DefaultHTTPMiddlewareOptions() *HTTPMiddlewareOptions {
	return &HTTPMiddlewareOptions{
		LogBodyOnErrors:  false,
		LogResponseBody:  false,
		EnableRequestID:  false,
		RequestIDHeader:  "X-Request-ID",
		EnableAudit:      false,
		EnableMetrics:    false,
		SkipPaths:        nil,
		SkipPathPrefixes: nil,
		LogLevelByStatus: nil,
		CustomFields:     nil,
	}
}

// WithLogBodyOnErrors enables logging request body on 4xx/5xx errors
func WithLogBodyOnErrors(enabled bool) HTTPMiddlewareOption {
	return func(o *HTTPMiddlewareOptions) {
		o.LogBodyOnErrors = enabled
	}
}

// WithLogResponseBody enables logging response body on errors
func WithLogResponseBody(enabled bool) HTTPMiddlewareOption {
	return func(o *HTTPMiddlewareOptions) {
		o.LogResponseBody = enabled
	}
}

// WithRequestID enables request ID generation/extraction
func WithRequestID(enabled bool) HTTPMiddlewareOption {
	return func(o *HTTPMiddlewareOptions) {
		o.EnableRequestID = enabled
	}
}

// WithRequestIDHeader sets the header name for request ID
func WithRequestIDHeader(header string) HTTPMiddlewareOption {
	return func(o *HTTPMiddlewareOptions) {
		o.RequestIDHeader = header
	}
}

// WithAudit enables audit event emission for requests
func WithAudit(enabled bool) HTTPMiddlewareOption {
	return func(o *HTTPMiddlewareOptions) {
		o.EnableAudit = enabled
	}
}

// WithAuditMethods specifies which HTTP methods to audit (nil = all)
func WithAuditMethods(methods ...string) HTTPMiddlewareOption {
	return func(o *HTTPMiddlewareOptions) {
		o.AuditEventTypes = methods
	}
}

// WithSkipPaths excludes exact paths from logging
func WithSkipPaths(paths ...string) HTTPMiddlewareOption {
	return func(o *HTTPMiddlewareOptions) {
		o.SkipPaths = paths
	}
}

// WithSkipPathPrefixes excludes paths starting with prefixes from logging
func WithSkipPathPrefixes(prefixes ...string) HTTPMiddlewareOption {
	return func(o *HTTPMiddlewareOptions) {
		o.SkipPathPrefixes = prefixes
	}
}

// WithLogLevel sets a custom log level for a status code
func WithLogLevel(statusCode int, level logger.LogLevel) HTTPMiddlewareOption {
	return func(o *HTTPMiddlewareOptions) {
		if o.LogLevelByStatus == nil {
			o.LogLevelByStatus = make(map[int]logger.LogLevel)
		}
		o.LogLevelByStatus[statusCode] = level
	}
}

// WithLogLevels sets custom log levels for status code ranges
func WithLogLevels(levels map[int]logger.LogLevel) HTTPMiddlewareOption {
	return func(o *HTTPMiddlewareOptions) {
		o.LogLevelByStatus = levels
	}
}

// WithCustomFields adds custom fields to every log entry
func WithCustomFields(fields map[string]any) HTTPMiddlewareOption {
	return func(o *HTTPMiddlewareOptions) {
		o.CustomFields = fields
	}
}

// WithMetrics enables metrics collection
func WithMetrics(enabled bool) HTTPMiddlewareOption {
	return func(o *HTTPMiddlewareOptions) {
		o.EnableMetrics = enabled
		if enabled && o.MetricsCollector == nil {
			o.MetricsCollector = NewDefaultMetricsCollector()
		}
	}
}

// WithMetricsCollector sets a custom metrics collector
func WithMetricsCollector(collector MetricsCollector) HTTPMiddlewareOption {
	return func(o *HTTPMiddlewareOptions) {
		o.MetricsCollector = collector
		o.EnableMetrics = collector != nil
	}
}

// WithOnRequestStart sets a callback for request start
func WithOnRequestStart(fn func(r *http.Request)) HTTPMiddlewareOption {
	return func(o *HTTPMiddlewareOptions) {
		o.OnRequestStart = fn
	}
}

// WithOnRequestEnd sets a callback for request end
func WithOnRequestEnd(fn func(r *http.Request, statusCode int, duration time.Duration)) HTTPMiddlewareOption {
	return func(o *HTTPMiddlewareOptions) {
		o.OnRequestEnd = fn
	}
}

// WithBodySampleRate sets the probability of logging request body for all requests (0.0-1.0).
func WithBodySampleRate(rate float64) HTTPMiddlewareOption {
	return func(o *HTTPMiddlewareOptions) {
		o.BodySampleRate = rate
	}
}
