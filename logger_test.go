package logger

import (
	"bytes"
	"context"
	"io"
	"net/http"
	"net/url"
	"os"
	"testing"
)

// Benchmark

func BenchmarkLogInfo(b *testing.B) {
	SetConfig(Config{
		Output:      io.Discard, // Don't actually write logs during benchmark
		Level:       LevelInfo,
		EnableColor: false,
		TimeFormat:  "15:04:05",
	})

	for i := 0; b.Loop(); i++ {
		LogInfo("Benchmark test", "iteration", i, "data", "test")
	}
}

// Test functions

var defaultTestConfig = Config{
	Output:      os.Stdout,
	Level:       LevelTrace,
	EnableColor: true,
	TimeFormat:  "15:04:05",
}

// TestMain sets up and tears down global state for all tests
func TestMain(m *testing.M) {
	// Global setup: set logger config for all tests
	SetConfig(defaultTestConfig)
	code := m.Run()
	// Global teardown: reset logger config if needed
	SetConfig(Config{}) // Optionally reset to zero value
	os.Exit(code)
}

// Individual tests

func TestLogDebug(t *testing.T) {
	LogDebug("Test Debug", "data", "debug")
}

func TestLogInfo(t *testing.T) {
	LogInfo("Test Info", "data", "info")
}

func TestLogNotice(t *testing.T) {
	LogNotice("Test Notice", "data", "notice")
}

func TestLogTrace(t *testing.T) {
	LogTrace("Test Trace", "data", "trace")
}

func TestLogWarn(t *testing.T) {
	LogWarn("Test Warn", "data", "warn")
}

func TestLogError(t *testing.T) {
	LogError("Test Error", "data", "error")
}

type traceIDKeyType struct{}

func TestLogInfoWithContext(t *testing.T) {
	ctx := context.WithValue(context.Background(), traceIDKeyType{}, "bench123")
	LogInfoWithContext(ctx, "Test InfoWithContext", "data", "test")
}

// Helper to create a test HTTP request with body and status code
func newTestRequest(body string, statusCode int) *http.Request {
	testUrl, _ := url.Parse("http://localhost/api/test/1")
	return &http.Request{
		Method:     "POST",
		URL:        testUrl,
		Header:     http.Header{"User-Agent": []string{"test-agent"}},
		Body:       io.NopCloser(bytes.NewBufferString(body)),
		RemoteAddr: "127.0.0.1:12345",
		Response:   &http.Response{StatusCode: statusCode},
	}
}

func TestLogHttpRequest_JSON_200(t *testing.T) {
	body := `{"name":"John","age":30,"city":"New York", "password":"mypassword"}`
	req := newTestRequest(body, 200)
	LogHttpRequest(req)
}

func TestLogHttpRequest_JSON_500(t *testing.T) {
	body := `{"name":"John","age":30,"city": false, "password":"mypassword"}`
	req := newTestRequest(body, 500)
	LogHttpRequest(req)
}

func TestLogHttpRequest_Text_200(t *testing.T) {
	body := "plain text body"
	req := newTestRequest(body, 200)
	LogHttpRequest(req)
}

func TestLogHttpRequest_Text_500(t *testing.T) {
	body := "plain text body"
	req := newTestRequest(body, 500)
	LogHttpRequest(req)
}
