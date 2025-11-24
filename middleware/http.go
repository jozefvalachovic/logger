package middleware

import (
	"bytes"
	"io"
	"log"
	"net/http"
	"strings"
	"sync"
	"time"

	"github.com/jozefvalachovic/logger/v3"
)

// wrappedWriter is used to capture the status code of HTTP responses
type wrappedWriter struct {
	http.ResponseWriter
	statusCode int
}

// WriteHeader captures the status code for logging
func (w *wrappedWriter) WriteHeader(statusCode int) {
	w.ResponseWriter.WriteHeader(statusCode)
	w.statusCode = statusCode
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
		New: func() interface{} {
			return &wrappedWriter{statusCode: http.StatusOK}
		},
	}

	bufferPool = sync.Pool{
		New: func() interface{} {
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

// LogHTTPMiddleware is an HTTP middleware that logs incoming requests and their details
func LogHTTPMiddleware(next http.Handler, logBodyOnErrors bool) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		start := time.Now()
		cfg := logger.GetConfig()

		// Check if path should be completely redacted
		fullPath := logger.GetFullPath(r.URL)
		if logger.ShouldRedactPath(fullPath, cfg) {
			// Use logger to write to configured output
			logger.LogInfo("HTTP Request [REDACTED]", "__method", r.Method, "__path", cfg.RedactMask)
			next.ServeHTTP(w, r)
			return
		}

		// Read and buffer the body before processing (with configurable size limit)
		var bodyBytes []byte
		var bodyErr error
		var truncated bool
		maxBodySize := cfg.MaxBodySize

		contentType := r.Header.Get("Content-Type")
		shouldLog := shouldLogBody(contentType)

		if logBodyOnErrors && shouldLog && r.Body != nil {
			// Get buffer from pool
			buf := bufferPool.Get().(*bytes.Buffer)
			buf.Reset()
			defer bufferPool.Put(buf)

			limitedReader := io.LimitReader(r.Body, maxBodySize+1) // Read one extra byte to detect truncation
			bodyBytes, bodyErr = io.ReadAll(limitedReader)
			_ = r.Body.Close()

			// Check if we hit the limit
			if int64(len(bodyBytes)) > maxBodySize {
				bodyBytes = bodyBytes[:maxBodySize]
				truncated = true
			}

			// Restore the body for the handler
			r.Body = io.NopCloser(bytes.NewBuffer(bodyBytes))
		}

		// Get wrapped writer from pool
		wrapped := wrappedWriterPool.Get().(*wrappedWriter)
		wrapped.ResponseWriter = w
		wrapped.statusCode = http.StatusOK
		defer wrappedWriterPool.Put(wrapped)

		// Recover from panics with stack trace
		defer func() {
			if err := recover(); err != nil {
				stack := logger.GetStackTrace()
				logger.LogError("HTTP Panic recovered",
					"__error", err,
					"method", r.Method,
					"path", fullPath,
					"stack", stack,
				)
				http.Error(w, "Internal Server Error", http.StatusInternalServerError)
			}
		}()

		next.ServeHTTP(wrapped, r)

		statusCode, _ := logger.FormatStatusCode(wrapped.statusCode)
		endPoint := logger.FormatString(fullPath, logger.Cyan, false)
		duration := time.Since(start).String()

		// Log the request
		log.Printf("%s %s %s %s", statusCode, r.Method, endPoint, duration)

		// If status code is 4xx or 5xx, log the request body
		if logBodyOnErrors && wrapped.statusCode >= 400 && wrapped.statusCode <= 599 {
			if bodyErr != nil {
				logger.LogError("Failed to read HTTP request body for error logging", "__error", bodyErr)
			} else if shouldLog {
				keyValues := []any{
					"__method", r.Method,
					"__path", fullPath,
					"__status", wrapped.statusCode,
				}

				// Add ellipsis if truncated
				if truncated {
					bodyStr := string(bodyBytes) + "..."
					bodyKeyValues := []any{"body", bodyStr}
					keyValues = append(keyValues, bodyKeyValues...)
				} else {
					bodyKeyValues := logger.BodyToKeyValues("body", bodyBytes)
					keyValues = append(keyValues, bodyKeyValues...)
				}

				logger.LogError("Failed Request", keyValues...)
			}
		}
	})
}
