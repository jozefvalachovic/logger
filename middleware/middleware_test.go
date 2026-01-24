package middleware_test

import (
	"bytes"
	"net"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/jozefvalachovic/logger/v4"
	"github.com/jozefvalachovic/logger/v4/middleware"
)

// syncWriter is a thread-safe writer for testing
type syncWriter struct {
	write func([]byte) (int, error)
}

func (sw *syncWriter) Write(p []byte) (int, error) {
	return sw.write(p)
}

// Test HTTP Middleware
func TestHTTPMiddleware(t *testing.T) {
	buf := &bytes.Buffer{}
	logger.SetConfig(logger.Config{
		Output:      buf,
		Level:       logger.LevelTrace,
		EnableColor: false,
		TimeFormat:  "15:04:05",
		MaxBodySize: 1024,
	})

	handler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte("OK"))
	})

	wrappedHandler := middleware.LogHTTPMiddleware(handler, middleware.WithLogBodyOnErrors(true))

	req := httptest.NewRequest("GET", "/test", nil)
	rec := httptest.NewRecorder()

	wrappedHandler.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Errorf("Expected status 200, got %d", rec.Code)
	}
}

// Test HTTP Middleware with Error
func TestHTTPMiddlewareError(t *testing.T) {
	buf := &bytes.Buffer{}
	logger.SetConfig(logger.Config{
		Output:      buf,
		Level:       logger.LevelTrace,
		EnableColor: false,
		TimeFormat:  "15:04:05",
		MaxBodySize: 1024,
	})

	handler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusInternalServerError)
		_, _ = w.Write([]byte("Error"))
	})

	wrappedHandler := middleware.LogHTTPMiddleware(handler, middleware.WithLogBodyOnErrors(true))

	body := `{"error":"test"}`
	req := httptest.NewRequest("POST", "/test", strings.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()

	wrappedHandler.ServeHTTP(rec, req)

	if rec.Code != http.StatusInternalServerError {
		t.Errorf("Expected status 500, got %d", rec.Code)
	}

	output := buf.String()
	if !strings.Contains(output, "Failed Request") {
		t.Error("Should log failed request")
	}
}

// Test HTTP Middleware Panic Recovery
func TestHTTPMiddlewarePanicRecovery(t *testing.T) {
	buf := &bytes.Buffer{}
	logger.SetConfig(logger.Config{
		Output:      buf,
		Level:       logger.LevelTrace,
		EnableColor: false,
		TimeFormat:  "15:04:05",
	})

	handler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		panic("test panic")
	})

	wrappedHandler := middleware.LogHTTPMiddleware(handler, middleware.WithLogBodyOnErrors(true))

	req := httptest.NewRequest("GET", "/test", nil)
	rec := httptest.NewRecorder()

	wrappedHandler.ServeHTTP(rec, req)

	if rec.Code != http.StatusInternalServerError {
		t.Errorf("Expected status 500 after panic, got %d", rec.Code)
	}

	output := buf.String()
	if !strings.Contains(output, "HTTP Panic Recovered") {
		t.Error("Should log panic recovery")
	}
	if !strings.Contains(output, "stack") {
		t.Error("Should include stack trace")
	}
}

// Test Content-Type Filtering
func TestContentTypeFiltering(t *testing.T) {
	buf := &bytes.Buffer{}
	logger.SetConfig(logger.Config{
		Output:      buf,
		Level:       logger.LevelTrace,
		EnableColor: false,
		TimeFormat:  "15:04:05",
		MaxBodySize: 1024,
	})

	handler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusBadRequest)
	})

	wrappedHandler := middleware.LogHTTPMiddleware(handler, middleware.WithLogBodyOnErrors(true))

	tests := []struct {
		name        string
		contentType string
		shouldLog   bool
	}{
		{"JSON", "application/json", true},
		{"XML", "application/xml", true},
		{"Text", "text/plain", true},
		{"Form", "application/x-www-form-urlencoded", true},
		{"Binary", "application/octet-stream", false},
		{"Image", "image/png", false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			buf.Reset()
			body := "test body content"
			req := httptest.NewRequest("POST", "/test", strings.NewReader(body))
			req.Header.Set("Content-Type", tt.contentType)
			rec := httptest.NewRecorder()

			wrappedHandler.ServeHTTP(rec, req)

			output := buf.String()
			hasBody := strings.Contains(output, "test body content")
			if tt.shouldLog && !hasBody {
				t.Errorf("Content-Type %s should log body, but didn't", tt.contentType)
			}
			if !tt.shouldLog && hasBody {
				t.Errorf("Content-Type %s should not log body, but did", tt.contentType)
			}
		})
	}
}

// Test Path Redaction
func TestPathRedaction(t *testing.T) {
	buf := &bytes.Buffer{}
	logger.SetConfig(logger.Config{
		Output:      buf,
		Level:       logger.LevelTrace,
		EnableColor: false,
		TimeFormat:  "15:04:05",
		RedactPaths: []string{"/admin", "/secret"},
		RedactMask:  "[REDACTED]",
	})

	handler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	})

	wrappedHandler := middleware.LogHTTPMiddleware(handler, middleware.WithLogBodyOnErrors(true))

	tests := []struct {
		path         string
		shouldRedact bool
	}{
		{"/admin/users", true},
		{"/secret/keys", true},
		{"/public/data", false},
		{"/api/admin", true}, // contains /admin
	}

	for _, tt := range tests {
		t.Run(tt.path, func(t *testing.T) {
			buf.Reset()
			req := httptest.NewRequest("GET", tt.path, nil)
			rec := httptest.NewRecorder()

			wrappedHandler.ServeHTTP(rec, req)

			output := buf.String()
			if tt.shouldRedact && !strings.Contains(output, "[REDACTED]") {
				t.Errorf("Path %s should be redacted", tt.path)
			}
			if !tt.shouldRedact && strings.Contains(output, "[REDACTED]") {
				t.Errorf("Path %s should not be redacted", tt.path)
			}
		})
	}
}

// Test TCP Middleware
func TestTCPMiddleware(t *testing.T) {
	buf := &bytes.Buffer{}
	var mu sync.Mutex
	safeWrite := func(p []byte) (n int, err error) {
		mu.Lock()
		defer mu.Unlock()
		return buf.Write(p)
	}
	safeRead := func() string {
		mu.Lock()
		defer mu.Unlock()
		return buf.String()
	}

	logger.SetConfig(logger.Config{
		Output:      &syncWriter{write: safeWrite},
		Level:       logger.LevelTrace,
		EnableColor: false,
		TimeFormat:  "15:04:05",
	})

	// Create a pipe to simulate a connection
	server, client := net.Pipe()
	defer func() { _ = client.Close() }()

	done := make(chan bool)
	handler := func(conn net.Conn) {
		// Simulate some work
		time.Sleep(10 * time.Millisecond)
		done <- true
	}

	wrappedHandler := middleware.LogTCPMiddleware(handler)

	go wrappedHandler(server)
	<-done
	time.Sleep(5 * time.Millisecond) // Give time for deferred cleanup

	output := safeRead()
	if !strings.Contains(output, "TCP Connection Started") {
		t.Error("Should log TCP connection start")
	}
	if !strings.Contains(output, "TCP Connection Ended") {
		t.Error("Should log TCP connection end")
	}
	if !strings.Contains(output, "duration") {
		t.Error("Should log connection duration")
	}
}

// Test TCP Middleware Panic Recovery
func TestTCPMiddlewarePanicRecovery(t *testing.T) {
	buf := &bytes.Buffer{}
	var mu sync.Mutex
	safeWrite := func(p []byte) (n int, err error) {
		mu.Lock()
		defer mu.Unlock()
		return buf.Write(p)
	}
	safeRead := func() string {
		mu.Lock()
		defer mu.Unlock()
		return buf.String()
	}

	logger.SetConfig(logger.Config{
		Output:      &syncWriter{write: safeWrite},
		Level:       logger.LevelTrace,
		EnableColor: false,
		TimeFormat:  "15:04:05",
	})

	server, client := net.Pipe()
	defer func() { _ = client.Close() }()

	done := make(chan bool)
	handler := func(conn net.Conn) {
		defer func() { done <- true }()
		panic("tcp panic test")
	}

	wrappedHandler := middleware.LogTCPMiddleware(handler)

	go wrappedHandler(server)
	<-done
	time.Sleep(5 * time.Millisecond) // Give time for deferred cleanup

	output := safeRead()
	if !strings.Contains(output, "TCP Panic recovered") {
		t.Error("Should log TCP panic recovery")
	}
	if !strings.Contains(output, "stack") {
		t.Error("Should include stack trace in panic log")
	}
}

// Test HTTP Middleware with Request ID
func TestHTTPMiddlewareRequestID(t *testing.T) {
	buf := &bytes.Buffer{}
	logger.SetConfig(logger.Config{
		Output:      buf,
		Level:       logger.LevelTrace,
		EnableColor: false,
		TimeFormat:  "15:04:05",
		MaxBodySize: 1024,
	})

	handler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// Check that request ID is in context
		reqID := middleware.GetRequestID(r.Context())
		if reqID == "" {
			t.Error("Request ID should be in context")
		}
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte("OK"))
	})

	wrappedHandler := middleware.LogHTTPMiddleware(handler,
		middleware.WithRequestID(true),
	)

	req := httptest.NewRequest("GET", "/test", nil)
	rec := httptest.NewRecorder()

	wrappedHandler.ServeHTTP(rec, req)

	// Should have request ID in response header
	if rec.Header().Get("X-Request-ID") == "" {
		t.Error("Should set X-Request-ID response header")
	}
}

// Test HTTP Middleware with custom Request ID header
func TestHTTPMiddlewareCustomRequestIDHeader(t *testing.T) {
	buf := &bytes.Buffer{}
	logger.SetConfig(logger.Config{
		Output:      buf,
		Level:       logger.LevelTrace,
		EnableColor: false,
		TimeFormat:  "15:04:05",
		MaxBodySize: 1024,
	})

	handler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	})

	wrappedHandler := middleware.LogHTTPMiddleware(handler,
		middleware.WithRequestID(true),
		middleware.WithRequestIDHeader("X-Correlation-ID"),
	)

	// Test extraction of existing ID
	req := httptest.NewRequest("GET", "/test", nil)
	req.Header.Set("X-Correlation-ID", "existing-correlation-id")
	rec := httptest.NewRecorder()

	wrappedHandler.ServeHTTP(rec, req)

	// Should use existing correlation ID
	if rec.Header().Get("X-Correlation-ID") != "existing-correlation-id" {
		t.Error("Should preserve existing correlation ID")
	}
}

// Test HTTP Middleware with Skip Paths
func TestHTTPMiddlewareSkipPaths(t *testing.T) {
	logger.SetConfig(logger.Config{
		Output:      &bytes.Buffer{},
		Level:       logger.LevelTrace,
		EnableColor: false,
		TimeFormat:  "15:04:05",
	})

	var loggedPaths []string
	var mu sync.Mutex

	handler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	})

	wrappedHandler := middleware.LogHTTPMiddleware(handler,
		middleware.WithSkipPaths("/health", "/ready"),
		middleware.WithSkipPathPrefixes("/metrics"),
		middleware.WithOnRequestEnd(func(r *http.Request, statusCode int, duration time.Duration) {
			mu.Lock()
			loggedPaths = append(loggedPaths, r.URL.Path)
			mu.Unlock()
		}),
	)

	tests := []struct {
		path      string
		shouldLog bool
	}{
		{"/health", false},
		{"/ready", false},
		{"/metrics/cpu", false},
		{"/api/users", true},
	}

	for _, tt := range tests {
		t.Run(tt.path, func(t *testing.T) {
			mu.Lock()
			loggedPaths = nil
			mu.Unlock()

			req := httptest.NewRequest("GET", tt.path, nil)
			rec := httptest.NewRecorder()

			wrappedHandler.ServeHTTP(rec, req)

			mu.Lock()
			wasLogged := len(loggedPaths) > 0
			mu.Unlock()

			if tt.shouldLog && !wasLogged {
				t.Errorf("Path %s should be logged", tt.path)
			}
			if !tt.shouldLog && wasLogged {
				t.Errorf("Path %s should be skipped", tt.path)
			}
		})
	}
}

// Test HTTP Middleware Options Pattern
func TestHTTPMiddlewareOptionsPattern(t *testing.T) {
	buf := &bytes.Buffer{}
	logger.SetConfig(logger.Config{
		Output:      buf,
		Level:       logger.LevelTrace,
		EnableColor: false,
		TimeFormat:  "15:04:05",
		MaxBodySize: 1024,
	})

	var startCalled, endCalled bool
	var capturedStatus int
	var capturedDuration time.Duration

	handler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusCreated)
	})

	wrappedHandler := middleware.LogHTTPMiddleware(handler,
		middleware.WithLogBodyOnErrors(true),
		middleware.WithRequestID(true),
		middleware.WithCustomFields(map[string]any{"app": "test"}),
		middleware.WithOnRequestStart(func(r *http.Request) {
			startCalled = true
		}),
		middleware.WithOnRequestEnd(func(r *http.Request, statusCode int, duration time.Duration) {
			endCalled = true
			capturedStatus = statusCode
			capturedDuration = duration
		}),
	)

	req := httptest.NewRequest("POST", "/api/test", nil)
	rec := httptest.NewRecorder()

	wrappedHandler.ServeHTTP(rec, req)

	if !startCalled {
		t.Error("OnRequestStart callback should be called")
	}
	if !endCalled {
		t.Error("OnRequestEnd callback should be called")
	}
	if capturedStatus != http.StatusCreated {
		t.Errorf("Expected status 201, got %d", capturedStatus)
	}
	if capturedDuration <= 0 {
		t.Error("Duration should be positive")
	}
}

// Test HTTP Middleware with explicit options struct
func TestHTTPMiddlewareWithExplicitOptions(t *testing.T) {
	buf := &bytes.Buffer{}
	logger.SetConfig(logger.Config{
		Output:      buf,
		Level:       logger.LevelTrace,
		EnableColor: false,
		TimeFormat:  "15:04:05",
		MaxBodySize: 1024,
	})

	handler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		reqID := middleware.GetRequestID(r.Context())
		if reqID == "" {
			t.Error("Request ID should be in context")
		}
		w.WriteHeader(http.StatusOK)
	})

	wrappedHandler := middleware.LogHTTPMiddleware(handler,
		middleware.WithLogBodyOnErrors(true),
		middleware.WithRequestID(true),
		middleware.WithRequestIDHeader("X-Request-ID"),
		middleware.WithSkipPaths("/health"),
	)

	req := httptest.NewRequest("GET", "/test", nil)
	rec := httptest.NewRecorder()

	wrappedHandler.ServeHTTP(rec, req)

	if rec.Header().Get("X-Request-ID") == "" {
		t.Error("Should set request ID header")
	}
}

// Test Response Body Logging on Errors
func TestHTTPMiddlewareResponseBodyLogging(t *testing.T) {
	buf := &bytes.Buffer{}
	logger.SetConfig(logger.Config{
		Output:      buf,
		Level:       logger.LevelTrace,
		EnableColor: false,
		TimeFormat:  "15:04:05",
		MaxBodySize: 1024,
	})

	handler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusBadRequest)
		_, _ = w.Write([]byte(`{"error":"validation failed"}`))
	})

	wrappedHandler := middleware.LogHTTPMiddleware(handler,
		middleware.WithLogBodyOnErrors(true),
		middleware.WithLogResponseBody(true),
	)

	req := httptest.NewRequest("POST", "/test", strings.NewReader(`{"name":"test"}`))
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()

	wrappedHandler.ServeHTTP(rec, req)

	output := buf.String()
	// The response body logging writes to the logger output which includes validation failed
	if !strings.Contains(output, "validation failed") && !strings.Contains(output, "request_body") {
		t.Error("Should log request or response body on error")
	}
}
