package logger

import (
	"bytes"
	"io"
	"log"
	"net"
	"net/http"
	"time"
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

// LogHTTPMiddleware is an HTTP middleware that logs incoming requests and their details
func LogHTTPMiddleware(next http.Handler, logBodyOnErrors bool) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		start := time.Now()

		// Wrap the response writer to capture the status code
		wrapped := &wrappedWriter{
			ResponseWriter: w,
			statusCode:     http.StatusOK,
		}

		// Recover from panics
		defer func() {
			if err := recover(); err != nil {
				LogError("HTTP Panic recovered",
					"__error", err,
					"body", r.Body,
					"method", r.Method,
					"path", getFullPath(r.URL),
				)
				http.Error(w, "Internal Server Error", http.StatusInternalServerError)
			}
		}()

		next.ServeHTTP(wrapped, r)

		statusCode, _ := formatStatusCode(wrapped.statusCode)
		// Build the full URL with query parameters
		fullPath := getFullPath(r.URL)
		endPoint := formatString(fullPath, cyan, false)
		// Calculate the duration of the request
		duration := time.Since(start).String()

		// Log the request
		log.Printf("%s %s %s %s", statusCode, r.Method, endPoint, duration)
		// If status code is 4xx or 5xx, log the request body (for detailed error audit)
		if logBodyOnErrors && wrapped.statusCode >= 400 && wrapped.statusCode <= 599 {
			bodyBytes, err := io.ReadAll(r.Body)
			if err != nil {
				LogError("Failed to read HTTP request body for error logging", "__error", err)
			} else {
				keyValues := []any{
					"__method", r.Method,
					"__path", getFullPath(r.URL),
					"__status", wrapped.statusCode,
				}
				// Restore the body for potential further use
				r.Body = io.NopCloser(bytes.NewBuffer(bodyBytes))
				bodyKeyValues := bodyToKeyValues("body", bodyBytes)
				keyValues = append(keyValues, bodyKeyValues...)

				LogError("Failed Request", keyValues...)
			}
		}
	})
}

// LogTCPMiddleware logs when a TCP connection is started and ended, and recovers from panics
func LogTCPMiddleware(next func(conn net.Conn)) func(conn net.Conn) {
	return func(conn net.Conn) {
		start := time.Now()
		remoteAddr := conn.RemoteAddr().String()
		LogTrace("TCP Connection Started", "remote", remoteAddr)

		defer func() {
			if err := recover(); err != nil {
				LogError("TCP Panic recovered",
					"__error", err,
					"remote", remoteAddr,
				)
			}
			duration := time.Since(start).String()
			LogTrace("TCP Connection Ended", "remote", remoteAddr, "duration", duration)
		}()

		next(conn)
	}
}
