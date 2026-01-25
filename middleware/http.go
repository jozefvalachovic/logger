// Package middleware provides HTTP and TCP logging middleware for the logger package.
package middleware

import (
	"bytes"
	"context"
	"fmt"
	"io"
	"net/http"
	"runtime/debug"
	"time"

	"github.com/jozefvalachovic/logger/v4"
)

func LogHTTPMiddleware(next http.Handler, opts ...HTTPMiddlewareOption) http.Handler {
	options := DefaultHTTPMiddlewareOptions()
	for _, opt := range opts {
		opt(options)
	}
	return logHTTPMiddlewareWithOptions(next, options)
}

// logHTTPMiddlewareWithOptions is the internal implementation with resolved options
func logHTTPMiddlewareWithOptions(next http.Handler, options *HTTPMiddlewareOptions) http.Handler {
	cfg := logger.GetConfig()

	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		start := time.Now()

		// Check if path should be skipped
		fullPath := r.URL.Path
		if r.URL.RawQuery != "" {
			fullPath = r.URL.Path + "?" + r.URL.RawQuery
		}

		if shouldSkipPath(fullPath, options) {
			next.ServeHTTP(w, r)
			return
		}

		// Handle Request ID
		requestID := ""
		if options.EnableRequestID {
			requestID = r.Header.Get(options.RequestIDHeader)
			if requestID == "" {
				requestID = generateRequestID()
			}
			// Add request ID to response header
			w.Header().Set(options.RequestIDHeader, requestID)
			// Add to context
			ctx := context.WithValue(r.Context(), RequestIDKey, requestID)
			ctx = context.WithValue(ctx, RequestStartKey, start)
			r = r.WithContext(ctx)
		}

		// Call start callback
		if options.OnRequestStart != nil {
			options.OnRequestStart(r)
		}

		// Read and buffer the body for potential logging
		var bodyBytes []byte
		var bodyErr error
		truncated := false
		if r.Body != nil && options.LogBodyOnErrors {
			bodyBytes, bodyErr = io.ReadAll(io.LimitReader(r.Body, cfg.MaxBodySize+1))
			r.Body.Close()
			if int64(len(bodyBytes)) > cfg.MaxBodySize {
				truncated = true
				bodyBytes = bodyBytes[:cfg.MaxBodySize]
			}
			r.Body = io.NopCloser(bytes.NewBuffer(bodyBytes))
		}

		// Get a wrapped writer from pool
		wrapped := wrappedWriterPool.Get().(*wrappedWriter)
		wrapped.ResponseWriter = w
		wrapped.statusCode = http.StatusOK
		wrapped.captureBody = options.LogResponseBody
		if options.LogResponseBody {
			wrapped.responseBody = bufferPool.Get().(*bytes.Buffer)
			wrapped.responseBody.Reset()
		}

		// Panic recovery
		defer func() {
			if rec := recover(); rec != nil {
				wrapped.statusCode = http.StatusInternalServerError
				stack := debug.Stack()

				// Record panic in metrics
				if options.EnableMetrics && options.MetricsCollector != nil {
					options.MetricsCollector.RecordPanic(r.Method, fullPath)
				}

				// Check if path should be redacted for logging
				panicLogPath := fullPath
				if logger.ShouldRedactPath(fullPath, cfg) {
					panicLogPath = cfg.RedactMask
				}

				keyValues := []any{
					"__method", r.Method,
					"__path", panicLogPath,
					"__status", wrapped.statusCode,
					"panic", rec,
					"stack", string(stack),
				}
				if requestID != "" {
					keyValues = append(keyValues, "request_id", requestID)
				}
				// Add custom fields
				for k, v := range options.CustomFields {
					keyValues = append(keyValues, k, v)
				}

				logger.LogError(fmt.Sprintf("PANIC %s %s [%d]", r.Method, panicLogPath, wrapped.statusCode), keyValues...)

				// Try to write error response if not already written
				http.Error(w, "Internal Server Error", http.StatusInternalServerError)
			}
		}()

		next.ServeHTTP(wrapped, r)

		duration := time.Since(start)

		// Record metrics
		if options.EnableMetrics && options.MetricsCollector != nil {
			options.MetricsCollector.RecordRequest(r.Method, fullPath, wrapped.statusCode, duration)
		}

		// Call end callback
		if options.OnRequestEnd != nil {
			options.OnRequestEnd(r, wrapped.statusCode, duration)
		}

		// Emit audit event if enabled
		if options.EnableAudit && shouldAuditRequest(r.Method, options) {
			emitAuditEvent(r, wrapped.statusCode, duration, requestID, fullPath)
		}

		// Determine log level based on status code
		logLevel := getLogLevelForStatus(wrapped.statusCode, options)

		// Check if path should be redacted
		logPath := fullPath
		if logger.ShouldRedactPath(fullPath, cfg) {
			logPath = cfg.RedactMask
		}

		// Build key-value pairs
		keyValues := []any{
			"__method", r.Method,
			"__path", logPath,
			"__status", wrapped.statusCode,
			"__duration", duration.String(),
		}

		if requestID != "" {
			keyValues = append(keyValues, "request_id", requestID)
		}

		// Add custom fields
		for k, v := range options.CustomFields {
			keyValues = append(keyValues, k, v)
		}

		// Log at the appropriate level with key details in the message
		logMsg := fmt.Sprintf("%s %s [%d] %s", r.Method, logPath, wrapped.statusCode, duration)

		switch logLevel {
		case logger.Error:
			logErrorDetails(r, wrapped, options, bodyBytes, bodyErr, truncated, fullPath, requestID, cfg)
			logger.LogError(logMsg, keyValues...)
		case logger.Warn:
			logErrorDetails(r, wrapped, options, bodyBytes, bodyErr, truncated, fullPath, requestID, cfg)
			logger.LogWarn(logMsg, keyValues...)
		case logger.Debug:
			logger.LogDebug(logMsg, keyValues...)
		default:
			logger.LogInfo(logMsg, keyValues...)
		}

		// Return pooled objects
		if wrapped.responseBody != nil {
			wrapped.responseBody.Reset()
			bufferPool.Put(wrapped.responseBody)
			wrapped.responseBody = nil
		}
		wrapped.captureBody = false
		wrappedWriterPool.Put(wrapped)
	})
}
