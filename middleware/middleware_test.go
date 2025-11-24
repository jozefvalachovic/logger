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

	"github.com/jozefvalachovic/logger/v3"
	"github.com/jozefvalachovic/logger/v3/middleware"
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

	wrappedHandler := middleware.LogHTTPMiddleware(handler, true)

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

	wrappedHandler := middleware.LogHTTPMiddleware(handler, true)

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

	wrappedHandler := middleware.LogHTTPMiddleware(handler, true)

	req := httptest.NewRequest("GET", "/test", nil)
	rec := httptest.NewRecorder()

	wrappedHandler.ServeHTTP(rec, req)

	if rec.Code != http.StatusInternalServerError {
		t.Errorf("Expected status 500 after panic, got %d", rec.Code)
	}

	output := buf.String()
	if !strings.Contains(output, "Panic recovered") {
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

	wrappedHandler := middleware.LogHTTPMiddleware(handler, true)

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

	wrappedHandler := middleware.LogHTTPMiddleware(handler, true)

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
