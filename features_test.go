package logger

import (
	"bytes"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"testing"
	"time"
)

// syncWriter is a thread-safe writer for testing
type syncWriter struct {
	mu  sync.Mutex
	buf *bytes.Buffer
}

func newSyncWriter() *syncWriter {
	return &syncWriter{buf: &bytes.Buffer{}}
}

func (sw *syncWriter) Write(p []byte) (int, error) {
	sw.mu.Lock()
	defer sw.mu.Unlock()
	return sw.buf.Write(p)
}

func (sw *syncWriter) String() string {
	sw.mu.Lock()
	defer sw.mu.Unlock()
	return sw.buf.String()
}

func TestLogSampling(t *testing.T) {
	tests := []struct {
		name       string
		sampleRate float64
		sampleSeed int64
		message    string
		expectLog  bool
	}{
		{
			name:       "rate 1.0 always logs",
			sampleRate: 1.0,
			sampleSeed: 42,
			message:    "test message",
			expectLog:  true,
		},
		{
			name:       "rate 0.001 rarely logs",
			sampleRate: 0.001,
			sampleSeed: 42,
			message:    "test message",
			expectLog:  false, // Very unlikely with this seed
		},
		{
			name:       "rate 0.5 samples some",
			sampleRate: 0.5,
			sampleSeed: 42,
			message:    "test message",
			expectLog:  true, // With seed 42, this particular message passes
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var buf bytes.Buffer
			SetConfig(Config{
				Output:     &buf,
				Level:      LevelTrace,
				SampleRate: tt.sampleRate,
				SampleSeed: tt.sampleSeed,
			})

			LogInfo(tt.message)

			output := buf.String()
			hasLog := strings.Contains(output, tt.message)

			if tt.expectLog && !hasLog {
				t.Errorf("Expected log output, but got none")
			}
			if !tt.expectLog && hasLog {
				t.Errorf("Expected no log output, but got: %s", output)
			}
		})
	}
}

func TestLogRotation(t *testing.T) {
	tmpDir := t.TempDir()
	logFile := filepath.Join(tmpDir, "test.log")

	config := &RotationConfig{
		MaxSize:    100, // 100 bytes for easy testing
		MaxAge:     time.Hour,
		MaxBackups: 3,
		Compress:   false,
	}

	writer, err := NewRotatingWriter(logFile, config)
	if err != nil {
		t.Fatalf("Failed to create rotating writer: %v", err)
	}
	defer func() { _ = writer.Close() }()

	// Write data exceeding MaxSize
	data := strings.Repeat("X", 120)
	_, err = writer.Write([]byte(data))
	if err != nil {
		t.Fatalf("Failed to write: %v", err)
	}

	// Check that rotation happened
	matches, err := filepath.Glob(logFile + ".*")
	if err != nil {
		t.Fatalf("Failed to glob: %v", err)
	}

	if len(matches) == 0 {
		t.Error("Expected rotated backup file, but found none")
	}
}

func TestAsyncLogging(t *testing.T) {
	sw := newSyncWriter()
	SetConfig(Config{
		Output:       sw,
		Level:        LevelTrace,
		AsyncMode:    true,
		BufferSize:   10,
		FlushTimeout: 100 * time.Millisecond,
	})

	// Give time for async worker to start
	time.Sleep(50 * time.Millisecond)

	testMsg := "async test message"
	LogInfo(testMsg)

	// Wait for async processing
	time.Sleep(200 * time.Millisecond)

	output := sw.String()
	if !strings.Contains(output, testMsg) {
		t.Errorf("Expected log output in async mode, got: %s", output)
	}

	// Cleanup: disable async mode
	SetConfig(Config{
		Output:    sw,
		Level:     LevelTrace,
		AsyncMode: false,
	})
	time.Sleep(50 * time.Millisecond)
}

func TestMetricsCollection(t *testing.T) {
	var buf bytes.Buffer
	SetConfig(Config{
		Output:        &buf,
		Level:         LevelTrace,
		EnableMetrics: true,
	})

	// Log some messages
	LogInfo("info message")
	LogError("error message")
	LogDebug("debug message")

	metricsData := GetMetrics()

	if metricsData == nil {
		t.Fatal("Expected metrics data, got nil")
	}

	totalLogs, ok := metricsData["total_logs"].(int64)
	if !ok {
		t.Fatal("Expected total_logs in metrics")
	}

	if totalLogs < 3 {
		t.Errorf("Expected at least 3 logs, got %d", totalLogs)
	}

	// Check that level counts exist
	if _, ok := metricsData["logs_info"]; !ok {
		t.Error("Expected logs_info in metrics")
	}
	if _, ok := metricsData["logs_error"]; !ok {
		t.Error("Expected logs_error in metrics")
	}

	// Cleanup
	SetConfig(Config{
		Output:        &buf,
		Level:         LevelTrace,
		EnableMetrics: false,
	})
}

func TestShouldSampleFunction(t *testing.T) {
	tests := []struct {
		name     string
		msg      string
		rate     float64
		seed     int64
		expected bool
	}{
		{"always log", "test", 1.0, 0, true},
		{"rarely log", "test", 0.001, 0, false},
		{"deterministic same seed", "test", 0.5, 42, true},
		{"deterministic same message", "test", 0.5, 42, true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := shouldSample(tt.msg, tt.rate, tt.seed)
			if result != tt.expected {
				t.Errorf("shouldSample(%s, %f, %d) = %v, want %v",
					tt.msg, tt.rate, tt.seed, result, tt.expected)
			}
		})
	}
}

func TestRotatingWriterBackupCleanup(t *testing.T) {
	tmpDir := t.TempDir()
	logFile := filepath.Join(tmpDir, "cleanup.log")

	config := &RotationConfig{
		MaxSize:    50,
		MaxBackups: 2, // Keep only 2 backups
		Compress:   false,
	}

	writer, err := NewRotatingWriter(logFile, config)
	if err != nil {
		t.Fatalf("Failed to create rotating writer: %v", err)
	}
	defer func() { _ = writer.Close() }()

	// Write multiple times to trigger several rotations
	for i := 0; i < 5; i++ {
		data := strings.Repeat("X", 60)
		_, _ = writer.Write([]byte(data))
		time.Sleep(10 * time.Millisecond) // Give time for cleanup goroutine
	}

	// Wait for cleanup to complete
	time.Sleep(200 * time.Millisecond)

	// Count backup files
	matches, _ := filepath.Glob(logFile + ".*")

	// Should have current file + max 2 backups = at most 2 backup files
	if len(matches) > config.MaxBackups {
		t.Errorf("Expected at most %d backup files, got %d", config.MaxBackups, len(matches))
	}
}

func TestMetricsConcurrency(t *testing.T) {
	m := NewLogMetrics()

	// Simulate concurrent logging
	done := make(chan bool)
	for i := 0; i < 10; i++ {
		go func() {
			for j := 0; j < 100; j++ {
				m.RecordLog(Info)
				m.RecordLog(Error)
			}
			done <- true
		}()
	}

	// Wait for all goroutines
	for i := 0; i < 10; i++ {
		<-done
	}

	metricsData := m.GetMetrics()
	totalLogs := metricsData["total_logs"].(int64)

	expectedTotal := int64(10 * 100 * 2) // 10 goroutines * 100 iterations * 2 log levels
	if totalLogs != expectedTotal {
		t.Errorf("Expected %d total logs, got %d", expectedTotal, totalLogs)
	}
}

func TestRotatingWriterCompression(t *testing.T) {
	tmpDir := t.TempDir()
	logFile := filepath.Join(tmpDir, "compress.log")

	config := &RotationConfig{
		MaxSize:  50,
		Compress: true,
	}

	writer, err := NewRotatingWriter(logFile, config)
	if err != nil {
		t.Fatalf("Failed to create rotating writer: %v", err)
	}
	defer func() { _ = writer.Close() }()

	// Write enough to trigger rotation
	data := strings.Repeat("Y", 60)
	_, _ = writer.Write([]byte(data))

	// Wait for compression goroutine
	time.Sleep(200 * time.Millisecond)

	// Check for .gz files
	matches, _ := filepath.Glob(logFile + ".*.gz")
	if len(matches) == 0 {
		t.Error("Expected compressed backup file (.gz), but found none")
	}
}

func TestAsyncModeChannelFullFallback(t *testing.T) {
	// First ensure we're in sync mode and wait for any previous async goroutines
	sw := newSyncWriter()
	SetConfig(Config{
		Output:    sw,
		Level:     LevelTrace,
		AsyncMode: false,
	})
	time.Sleep(100 * time.Millisecond)

	// Now start async mode with small buffer
	SetConfig(Config{
		Output:       sw,
		Level:        LevelTrace,
		AsyncMode:    true,
		BufferSize:   2,               // Very small buffer
		FlushTimeout: 5 * time.Second, // Long timeout so channel fills
	})

	time.Sleep(50 * time.Millisecond)

	// Fill the channel
	for i := 0; i < 10; i++ {
		LogInfo("message %d", "n", i)
	}

	// Some messages should still be logged via fallback
	time.Sleep(200 * time.Millisecond)

	output := sw.String()
	if len(output) == 0 {
		t.Error("Expected some log output even with full channel")
	}

	// Cleanup
	SetConfig(Config{
		Output:    sw,
		Level:     LevelTrace,
		AsyncMode: false,
	})
	time.Sleep(100 * time.Millisecond)
}

func TestRotatingWriterDefaultConfig(t *testing.T) {
	tmpDir := t.TempDir()
	logFile := filepath.Join(tmpDir, "default.log")

	// Pass nil config to test defaults
	writer, err := NewRotatingWriter(logFile, nil)
	if err != nil {
		t.Fatalf("Failed to create rotating writer with nil config: %v", err)
	}
	defer func() { _ = writer.Close() }()

	// Write some data
	data := "test data"
	n, err := writer.Write([]byte(data))
	if err != nil {
		t.Fatalf("Failed to write: %v", err)
	}
	if n != len(data) {
		t.Errorf("Expected to write %d bytes, wrote %d", len(data), n)
	}

	// Check file was created
	if _, err := os.Stat(logFile); os.IsNotExist(err) {
		t.Error("Expected log file to be created")
	}
}
