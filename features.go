package logger

import (
	"compress/gzip"
	"fmt"
	"hash/fnv"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"sort"
	"sync"
	"sync/atomic"
	"time"
)

// logEntry represents a log entry for async processing
type logEntry struct {
	level     LogLevel
	message   string
	keyValues []any
	pc        uintptr
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
		now := time.Now()
		// Calculate error rate (errors per second since first error)
		totalErrors := m.LogsByLevel[Error]
		if m.lastErrorTime.IsZero() {
			m.lastErrorTime = now
			m.ErrorRate = 0
		} else {
			duration := now.Sub(m.lastErrorTime).Seconds()
			if duration > 0 {
				m.ErrorRate = float64(totalErrors) / duration
			}
		}
	}
	m.mu.Unlock()
}

// GetMetrics returns a snapshot of current metrics
func (m *LogMetrics) GetMetrics() map[string]any {
	m.mu.RLock()
	defer m.mu.RUnlock()

	result := map[string]any{
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
				logInternalSync(entry.level, entry.message, entry.pc, entry.keyValues...)
			case <-ticker.C:
				// Flush any pending logs
				for len(logChan) > 0 {
					entry := <-logChan
					logInternalSync(entry.level, entry.message, entry.pc, entry.keyValues...)
				}
			case <-asyncDone:
				// Drain remaining logs
				for len(logChan) > 0 {
					entry := <-logChan
					logInternalSync(entry.level, entry.message, entry.pc, entry.keyValues...)
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
		// Sort by modification time (oldest first) to handle clock adjustments
		sort.Slice(matches, func(i, j int) bool {
			fi, erri := os.Stat(matches[i])
			fj, errj := os.Stat(matches[j])
			if erri != nil || errj != nil {
				return matches[i] < matches[j]
			}
			return fi.ModTime().Before(fj.ModTime())
		})
		// Remove oldest files
		for i := 0; i < len(matches)-w.config.MaxBackups; i++ {
			_ = os.Remove(matches[i])
		}
	}
}

func compressFile(filename string) {
	src, err := os.Open(filename)
	if err != nil {
		return
	}
	defer func() { _ = src.Close() }()

	dst, err := os.Create(filename + ".gz")
	if err != nil {
		return
	}

	gw := gzip.NewWriter(dst)
	if _, err := io.Copy(gw, src); err != nil {
		_ = gw.Close()
		_ = dst.Close()
		_ = os.Remove(filename + ".gz")
		return
	}

	if err := gw.Close(); err != nil {
		_ = dst.Close()
		_ = os.Remove(filename + ".gz")
		return
	}
	if err := dst.Close(); err != nil {
		_ = os.Remove(filename + ".gz")
		return
	}

	_ = src.Close()
	_ = os.Remove(filename)
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
func GetMetrics() map[string]any {
	if metrics == nil {
		return map[string]any{}
	}
	return metrics.GetMetrics()
}

// MetricsHandler returns an http.Handler that serves metrics in Prometheus exposition format.
// Enable metrics with logger.SetConfig(logger.Config{EnableMetrics: true}).
func MetricsHandler() http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/plain; version=0.0.4; charset=utf-8")

		if metrics == nil {
			_, _ = fmt.Fprintln(w, "# No metrics collected (EnableMetrics is false)")
			return
		}

		m := metrics.GetMetrics()

		configMu.RLock()
		prefix := globalConfig.MetricsPrefix
		configMu.RUnlock()
		if prefix == "" {
			prefix = "logger"
		}

		_, _ = fmt.Fprintf(w, "# HELP %s_logs_total Total number of log entries\n", prefix)
		_, _ = fmt.Fprintf(w, "# TYPE %s_logs_total counter\n", prefix)
		if total, ok := m["total_logs"].(int64); ok {
			_, _ = fmt.Fprintf(w, "%s_logs_total %d\n", prefix, total)
		}

		_, _ = fmt.Fprintf(w, "# HELP %s_logs_by_level Log entries by level\n", prefix)
		_, _ = fmt.Fprintf(w, "# TYPE %s_logs_by_level counter\n", prefix)
		for _, level := range []string{"trace", "debug", "info", "notice", "warn", "error", "audit"} {
			key := "logs_" + level
			if count, ok := m[key]; ok {
				_, _ = fmt.Fprintf(w, "%s_logs_by_level{level=%q} %v\n", prefix, level, count)
			}
		}

		_, _ = fmt.Fprintf(w, "# HELP %s_error_rate Errors per second\n", prefix)
		_, _ = fmt.Fprintf(w, "# TYPE %s_error_rate gauge\n", prefix)
		if rate, ok := m["error_rate"].(float64); ok {
			_, _ = fmt.Fprintf(w, "%s_error_rate %f\n", prefix, rate)
		}
	})
}
