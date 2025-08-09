package logger

import (
	"fmt"
	"log"
	"net/http"
	"time"
)

// Forward WriteHeader as before
func (w *wrappedWriter) WriteHeader(statusCode int) {
	w.ResponseWriter.WriteHeader(statusCode)
	w.statusCode = statusCode
}

// Forward Flush to the underlying ResponseWriter if it supports http.Flusher
func (w *wrappedWriter) Flush() {
	if flusher, ok := w.ResponseWriter.(http.Flusher); ok {
		flusher.Flush()
	}
}

// Optional: ensure at compile time that wrappedWriter implements http.Flusher
var _ http.Flusher = (*wrappedWriter)(nil)

func LogMiddleware(next http.Handler) http.Handler {
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
					"error", err,
					"method", r.Method,
					"path", getFullPath(r.URL),
				)
				http.Error(w, "Internal Server Error", http.StatusInternalServerError)
			}
		}()

		next.ServeHTTP(wrapped, r)

		var statusCode string
		// Colorize the status code based on the first letter
		switch wrapped.statusCode / 100 {
		case 2:
			statusCode = formatString(fmt.Sprintf("%d", wrapped.statusCode), green, false)
		case 3:
			statusCode = formatString(fmt.Sprintf("%d", wrapped.statusCode), blue, false)
		case 4, 5:
			statusCode = formatString(fmt.Sprintf("%d", wrapped.statusCode), red, false)
		default:
			statusCode = fmt.Sprintf("%d", wrapped.statusCode)
		}

		// Build the full URL with query parameters
		fullPath := getFullPath(r.URL)
		endPoint := formatString(fullPath, cyan, false)

		// Calculate the duration of the request
		duration := time.Since(start).String()

		// Log the request
		log.Printf("%s %s %s %s", statusCode, r.Method, endPoint, duration)
	})
}
