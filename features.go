package logger

import (
	"fmt"
	"hash/fnv"
	"os"
	"path/filepath"
	"sync"
	"sync/atomic"
	"time"
)

// logEntry represents a log entry for async processing
type logEntry struct {
	level     LogLevel
	message   string
	keyValues []any
}

// LogMetrics tracks logging metrics
type LogMetrics struct {
	mu            sync.RWMutex
	TotalLogs     int64
	LogsByLevel   map[LogLevel]int64
	ErrorRate     float64
	lastErrorTime time.Time
}

// NewLogMetrics creates a new metrics instance
func NewLogMetrics() *LogMetrics {
	return &LogMetrics{
		LogsByLevel: make(map[LogLevel]int64),
	}
}

// RecordLog increments log counters
func (m *LogMetrics) RecordLog(level LogLevel) {
	atomic.AddInt64(&m.TotalLogs, 1)

	m.mu.Lock()
	m.LogsByLevel[level]++
	if level == Error {
		m.lastErrorTime = time.Now()
		// Calculate error rate (errors per second over last minute)
		totalErrors := m.LogsByLevel[Error]
		duration := time.Since(m.lastErrorTime).Seconds()
		if duration > 0 {
			m.ErrorRate = float64(totalErrors) / duration
		}
	}
	m.mu.Unlock()
}

// GetMetrics returns a snapshot of current metrics
func (m *LogMetrics) GetMetrics() map[string]interface{} {
	m.mu.RLock()
	defer m.mu.RUnlock()

	result := map[string]interface{}{
		"total_logs": atomic.LoadInt64(&m.TotalLogs),
		"error_rate": m.ErrorRate,
	}

	for level, count := range m.LogsByLevel {
		result[fmt.Sprintf("logs_%s", levelToString(level))] = count
	}

	return result
}

func levelToString(level LogLevel) string {
	switch level {
	case Trace:
		return "trace"
	case Debug:
		return "debug"
	case Info:
		return "info"
	case Notice:
		return "notice"
	case Warn:
		return "warn"
	case Error:
		return "error"
	case Audit:
		return "audit"
	default:
		return "unknown"
	}
}

// shouldSample determines if a log message should be logged based on sampling rate
func shouldSample(msg string, rate float64, seed int64) bool {
	if rate >= 1.0 {
		return true
	}
	if rate <= 0.0 {
		return false
	}

	hash := fnv.New64a()
	hash.Write([]byte(msg))
	_, _ = fmt.Fprint(hash, seed)
	hashValue := hash.Sum64()

	return float64(hashValue%10000) < rate*10000
}

// startAsyncLogger starts the async logging goroutine
func startAsyncLogger(cfg Config) {
	asyncMu.Lock()

	// If already running, stop it first to avoid race conditions
	if asyncRunning {
		// Signal stop and close channels
		asyncDone <- true
		close(logChan)
		asyncRunning = false
		asyncMu.Unlock()

		// Wait for goroutine to finish
		asyncWg.Wait()

		asyncMu.Lock()
	}

	logChan = make(chan *logEntry, cfg.BufferSize)
	asyncDone = make(chan bool, 1) // Buffered to prevent blocking
	asyncRunning = true
	asyncWg.Add(1)

	asyncMu.Unlock()

	go func() {
		defer asyncWg.Done()
		ticker := time.NewTicker(cfg.FlushTimeout)
		defer ticker.Stop()

		for {
			select {
			case entry := <-logChan:
				logInternalSync(entry.level, entry.message, entry.keyValues...)
			case <-ticker.C:
				// Flush any pending logs
				for len(logChan) > 0 {
					entry := <-logChan
					logInternalSync(entry.level, entry.message, entry.keyValues...)
				}
			case <-asyncDone:
				// Drain remaining logs
				for len(logChan) > 0 {
					entry := <-logChan
					logInternalSync(entry.level, entry.message, entry.keyValues...)
				}
				return
			}
		}
	}()
}

// stopAsyncLogger stops the async logging goroutine
func stopAsyncLogger() {
	asyncMu.Lock()

	if !asyncRunning {
		asyncMu.Unlock()
		return
	}

	asyncDone <- true
	close(logChan)
	asyncRunning = false
	asyncMu.Unlock()

	// Wait for goroutine to finish
	asyncWg.Wait()
}

// RotatingWriter wraps an io.Writer with rotation capabilities
type RotatingWriter struct {
	mu        sync.Mutex
	filename  string
	file      *os.File
	size      int64
	config    *RotationConfig
	openTime  time.Time
	backupNum int
}

// NewRotatingWriter creates a new rotating file writer
func NewRotatingWriter(filename string, config *RotationConfig) (*RotatingWriter, error) {
	if config == nil {
		config = &RotationConfig{
			MaxSize:    100 << 20, // 100MB
			MaxAge:     7 * 24 * time.Hour,
			MaxBackups: 3,
			Compress:   false,
		}
	}

	w := &RotatingWriter{
		filename: filename,
		config:   config,
		openTime: time.Now(),
	}

	if err := w.openFile(); err != nil {
		return nil, err
	}

	return w, nil
}

func (w *RotatingWriter) openFile() error {
	info, err := os.Stat(w.filename)
	if err == nil {
		w.size = info.Size()
	}

	file, err := os.OpenFile(w.filename, os.O_CREATE|os.O_WRONLY|os.O_APPEND, 0644)
	if err != nil {
		return err
	}

	w.file = file
	w.openTime = time.Now()
	return nil
}

func (w *RotatingWriter) Write(p []byte) (n int, err error) {
	w.mu.Lock()
	defer w.mu.Unlock()

	// Check if rotation is needed
	if w.shouldRotate(int64(len(p))) {
		if err := w.rotate(); err != nil {
			return 0, err
		}
	}

	n, err = w.file.Write(p)
	w.size += int64(n)
	return n, err
}

func (w *RotatingWriter) shouldRotate(writeSize int64) bool {
	if w.config.MaxSize > 0 && w.size+writeSize > w.config.MaxSize {
		return true
	}
	if w.config.MaxAge > 0 && time.Since(w.openTime) > w.config.MaxAge {
		return true
	}
	return false
}

func (w *RotatingWriter) rotate() error {
	if w.file != nil {
		_ = w.file.Close()
	} // Create backup filename
	backupName := fmt.Sprintf("%s.%s.%d",
		w.filename,
		time.Now().Format("20060102-150405"),
		w.backupNum,
	)
	w.backupNum++

	// Rename current file
	if err := os.Rename(w.filename, backupName); err != nil {
		return err
	}

	// Compress if needed
	if w.config.Compress {
		go compressFile(backupName)
	}

	// Clean old backups
	go w.cleanOldBackups()

	// Open new file
	w.size = 0
	return w.openFile()
}

func (w *RotatingWriter) cleanOldBackups() {
	if w.config.MaxBackups <= 0 {
		return
	}

	pattern := w.filename + ".*"
	matches, err := filepath.Glob(pattern)
	if err != nil {
		return
	}

	if len(matches) > w.config.MaxBackups {
		// Remove oldest files
		for i := 0; i < len(matches)-w.config.MaxBackups; i++ {
			_ = os.Remove(matches[i])
		}
	}
}

func compressFile(filename string) {
	// Simple placeholder - in production, use gzip
	// For now, just rename with .gz extension as a marker
	_ = os.Rename(filename, filename+".gz")
}

// Close closes the rotating writer
func (w *RotatingWriter) Close() error {
	w.mu.Lock()
	defer w.mu.Unlock()

	if w.file != nil {
		return w.file.Close()
	}
	return nil
}

// GetMetrics returns the current logger metrics
func GetMetrics() map[string]interface{} {
	if metrics == nil {
		return map[string]interface{}{}
	}
	return metrics.GetMetrics()
}
