package logger

import (
	"bytes"
	"context"
	"io"
	"log/slog"
	"net/http"
	"net/url"
	"os"
	"strings"
	"sync"
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

// Test Config Validation
func TestConfigValidate(t *testing.T) {
	tests := []struct {
		name    string
		config  Config
		wantErr bool
	}{
		{
			name: "valid config",
			config: Config{
				Output:      os.Stdout,
				TimeFormat:  "2006-01-02",
				RedactMask:  "***",
				MaxBodySize: 1024,
			},
			wantErr: false,
		},
		{
			name: "nil output",
			config: Config{
				Output:     nil,
				TimeFormat: "2006-01-02",
				RedactMask: "***",
			},
			wantErr: true,
		},
		{
			name: "empty time format",
			config: Config{
				Output:     os.Stdout,
				TimeFormat: "",
				RedactMask: "***",
			},
			wantErr: true,
		},
		{
			name: "empty redact mask",
			config: Config{
				Output:     os.Stdout,
				TimeFormat: "2006-01-02",
				RedactMask: "",
			},
			wantErr: true,
		},
		{
			name: "negative max body size",
			config: Config{
				Output:      os.Stdout,
				TimeFormat:  "2006-01-02",
				RedactMask:  "***",
				MaxBodySize: -100,
			},
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := tt.config.Validate()
			if (err != nil) != tt.wantErr {
				t.Errorf("Config.Validate() error = %v, wantErr %v", err, tt.wantErr)
			}
		})
	}
}

// Test Redaction
func TestRedaction(t *testing.T) {
	buf := &bytes.Buffer{}
	SetConfig(Config{
		Output:      buf,
		Level:       LevelInfo,
		EnableColor: false,
		TimeFormat:  "15:04:05",
		RedactKeys:  []string{"password", "secret", "token", "authorization", "bearer", "api_key"},
		RedactMask:  "[REDACTED]",
		MaxBodySize: 1024,
	})

	LogInfo("Test redaction",
		"username", "john",
		"password", "supersecret123",
		"token", "abc123",
		"api_key", "key123",
		"normal_field", "visible",
	)

	output := buf.String()
	if strings.Contains(output, "supersecret123") {
		t.Error("Password should be redacted")
	}
	if strings.Contains(output, "abc123") {
		t.Error("Token should be redacted")
	}
	if strings.Contains(output, "key123") {
		t.Error("API key should be redacted")
	}
	if !strings.Contains(output, "[REDACTED]") {
		t.Error("Redaction mask should be present")
	}
	if !strings.Contains(output, "visible") {
		t.Error("Normal fields should not be redacted")
	}
}

// Test Context Propagation

func TestContextPropagation(t *testing.T) {
	buf := &bytes.Buffer{}
	SetConfig(Config{
		Output:      buf,
		Level:       LevelInfo,
		EnableColor: false,
		TimeFormat:  "15:04:05",
	})

	ctx := context.WithValue(context.Background(), "trace_id", "trace-123")
	LogInfoWithContext(ctx, "Test with context", "data", "test")

	output := buf.String()
	if !strings.Contains(output, "trace-123") {
		t.Error("Trace ID should be present in log output")
	}
}

// Test Concurrent Logging
func TestConcurrentLogging(t *testing.T) {
	buf := &bytes.Buffer{}
	SetConfig(Config{
		Output:      buf,
		Level:       LevelInfo,
		EnableColor: false,
		TimeFormat:  "15:04:05",
	})

	var wg sync.WaitGroup
	concurrency := 100

	for i := 0; i < concurrency; i++ {
		wg.Add(1)
		go func(id int) {
			defer wg.Done()
			LogInfo("Concurrent log", "goroutine", id)
		}(i)
	}

	wg.Wait()

	output := buf.String()
	// Just verify it didn't panic and produced some output
	if len(output) == 0 {
		t.Error("No output from concurrent logging")
	}
}

// Test Lazy Evaluation
func TestLazyEvaluation(t *testing.T) {
	buf := &bytes.Buffer{}
	SetConfig(Config{
		Output:      buf,
		Level:       slog.LevelWarn, // Only warn and error
		EnableColor: false,
		TimeFormat:  "15:04:05",
	})

	// These should not be logged
	LogDebug("Debug message", "key", "value")
	LogInfo("Info message", "key", "value")
	LogTrace("Trace message", "key", "value")

	// These should be logged
	LogWarn("Warn message", "key", "value")
	LogError("Error message", "key", "value")

	output := buf.String()
	if strings.Contains(output, "Debug message") {
		t.Error("Debug should not be logged when level is Warn")
	}
	if strings.Contains(output, "Info message") {
		t.Error("Info should not be logged when level is Warn")
	}
	if strings.Contains(output, "Trace message") {
		t.Error("Trace should not be logged when level is Warn")
	}
	if !strings.Contains(output, "Warn message") {
		t.Error("Warn should be logged")
	}
	if !strings.Contains(output, "Error message") {
		t.Error("Error should be logged")
	}
}

// Test Logger Interface
func TestLoggerInterface(t *testing.T) {
	buf := &bytes.Buffer{}
	SetConfig(Config{
		Output:      buf,
		Level:       LevelInfo,
		EnableColor: false,
		TimeFormat:  "15:04:05",
	})

	var logger Logger = DefaultLogger()

	logger.LogInfo("Test via interface", "key", "value")

	output := buf.String()
	if !strings.Contains(output, "Test via interface") {
		t.Error("Logger interface should work")
	}
}
