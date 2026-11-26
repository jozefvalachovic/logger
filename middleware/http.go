// Package middleware provides HTTP and TCP logging middleware for the logger package.
package middleware

import (
	"bytes"
	"context"
	"fmt"
	"math/rand/v2"
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

		// Skip decision is based on the request path only (query string excluded),
		// so endpoints like /health still skip correctly when probes add query params.
		if shouldSkipPath(r.URL.Path, options) {
			next.ServeHTTP(w, r)
			return
		}

		fullPath := r.URL.Path
		if r.URL.RawQuery != "" {
			fullPath = r.URL.Path + "?" + r.URL.RawQuery
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
			// Store an enriched logger so downstream handlers can use logger.FromContext(ctx)
			child := logger.DefaultLogger().With("requestId", requestID)
			ctx = logger.NewContext(ctx, child)
			r = r.WithContext(ctx)
		}

		// Call start callback
		if options.OnRequestStart != nil {
			options.OnRequestStart(r)
		}

		// Record the request body as the handler reads it, up to MaxBodySize.
		// The handler always receives the full, untruncated body.
		var capture *captureReader
		shouldCapture := options.LogBodyOnErrors || (options.BodySampleRate > 0 && rand.Float64() < options.BodySampleRate)
		if r.Body != nil && r.Body != http.NoBody && shouldCapture {
			capture = &captureReader{ReadCloser: r.Body, limit: cfg.MaxBodySize}
			r.Body = capture
		}

		// Not pooled: a handler that keeps the writer after returning must
		// never be able to write into another request's response.
		wrapped := &wrappedWriter{
			ResponseWriter:  w,
			statusCode:      http.StatusOK,
			captureBody:     options.LogResponseBody,
			maxCaptureBytes: cfg.MaxBodySize,
		}
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
			if wrapped.statusCode >= 400 {
				options.MetricsCollector.RecordError(r.Method, fullPath, wrapped.statusCode)
			}
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
			bodyBytes, bodyErr, truncated := capture.finish()
			logErrorDetails(r, wrapped, options, bodyBytes, bodyErr, truncated, fullPath, requestID, cfg)
			logger.LogError(logMsg, keyValues...)
		case logger.Warn:
			bodyBytes, bodyErr, truncated := capture.finish()
			logErrorDetails(r, wrapped, options, bodyBytes, bodyErr, truncated, fullPath, requestID, cfg)
			logger.LogWarn(logMsg, keyValues...)
		case logger.Debug:
			logger.LogDebug(logMsg, keyValues...)
		default:
			logger.LogInfo(logMsg, keyValues...)
		}

		// Detach the response buffer before returning it to the pool, so a
		// late write from the handler cannot reach the next request's buffer.
		if buf := wrapped.responseBody; buf != nil {
			wrapped.captureBody = false
			wrapped.responseBody = nil
			buf.Reset()
			bufferPool.Put(buf)
		}
	})
}
