package logger

import (
	"context"
	"fmt"
	"io"
	"log/slog"
	"os"
	"runtime/debug"
	"strings"
	"sync"
	"time"
)

type Config struct {
	Output      io.Writer
	Level       slog.Level
	EnableColor bool
	TimeFormat  string
	RedactKeys  []string
	RedactMask  string
	MaxBodySize int64    // Maximum size for HTTP body logging in bytes (default: 1MB)
	RedactPaths []string // URL paths to completely redact from logs

	// Sampling configuration
	SampleRate float64 // 0.0 to 1.0, where 0.1 = log 10% of messages (0 = disabled, 1.0 = all)
	SampleSeed int64   // Seed for deterministic sampling

	// Rotation configuration
	Rotation *RotationConfig

	// Async logging configuration
	AsyncMode    bool
	BufferSize   int           // Channel buffer size for async mode (default: 1000)
	FlushTimeout time.Duration // How often to flush in async mode (default: 1s)

	// Metrics configuration
	EnableMetrics bool
	MetricsPrefix string // Prefix for metric names (default: "logger")
}

// RotationConfig configures automatic log file rotation
type RotationConfig struct {
	MaxSize    int64         // Max size in bytes before rotation (default: 100MB)
	MaxAge     time.Duration // Max age before rotation (default: 7 days)
	MaxBackups int           // Number of old files to keep (default: 3)
	Compress   bool          // Compress rotated files (default: false)
}

// Validate checks if the Config has valid settings
func (c *Config) Validate() error {
	if c.Output == nil {
		return fmt.Errorf("output cannot be nil")
	}
	if c.TimeFormat == "" {
		return fmt.Errorf("TimeFormat cannot be empty")
	}
	if c.RedactMask == "" {
		return fmt.Errorf("RedactMask cannot be empty")
	}
	if c.MaxBodySize < 0 {
		return fmt.Errorf("MaxBodySize cannot be negative")
	}
	return nil
}

// Global logger instance and configuration
var (
	defaultLogger *slog.Logger
	configMu      sync.RWMutex
	globalConfig  Config

	// Async logging
	logChan      chan *logEntry
	asyncDone    chan bool
	asyncRunning bool
	asyncMu      sync.Mutex
	asyncWg      sync.WaitGroup // Tracks if async goroutine is running

	// Metrics
	metrics *LogMetrics

	defaultConfig = Config{
		Output:        os.Stdout,
		Level:         LevelTrace,
		EnableColor:   true,
		TimeFormat:    "2006-01-02 15:04:05",
		RedactKeys:    []string{"password", "secret", "token", "authorization", "bearer", "api_key", "api-key"},
		RedactMask:    "***",
		MaxBodySize:   1 << 20, // 1MB default
		RedactPaths:   []string{},
		SampleRate:    1.0, // Log everything by default
		SampleSeed:    0,
		Rotation:      nil, // No rotation by default
		AsyncMode:     false,
		BufferSize:    1000,
		FlushTimeout:  time.Second,
		EnableMetrics: false,
		MetricsPrefix: "logger",
	}
)

func init() {
	configMu.Lock()
	globalConfig = defaultConfig
	configMu.Unlock()
	initLogger()
}

// initLogger initializes the default logger based on the current configuration
func initLogger() {
	configMu.RLock()
	cfg := globalConfig
	configMu.RUnlock()

	opts := prettyHandlerOptions{
		SlogOpts: slog.HandlerOptions{
			Level: cfg.Level,
		},
		Config: cfg,
	}
	defaultLogger = slog.New(newPrettyHandler(cfg.Output, opts))
}

// logInternal is an internal function to log messages with key-value pairs
func logInternal(level LogLevel, message string, keyValues ...any) {
	// Lazy evaluation: skip expensive operations if log level doesn't match
	configMu.RLock()
	cfg := globalConfig
	configMu.RUnlock()

	if cfg.Level > slogLevelFromLogLevel(level) {
		return // Early return - don't process if we won't log anyway
	}

	// Apply sampling
	if cfg.SampleRate < 1.0 && !shouldSample(message, cfg.SampleRate, cfg.SampleSeed) {
		return
	}

	// Track metrics
	if cfg.EnableMetrics && metrics != nil {
		metrics.RecordLog(level)
	}

	// Use async logging if enabled
	if cfg.AsyncMode && asyncRunning {
		entry := &logEntry{
			level:     level,
			message:   message,
			keyValues: keyValues,
		}
		select {
		case logChan <- entry:
			// Successfully queued
		default:
			// Channel full, fall back to sync logging
			logInternalSync(level, message, keyValues...)
		}
		return
	}

	// Synchronous logging
	logInternalSync(level, message, keyValues...)
}

// logInternalSync performs synchronous logging (used by both sync and async paths)
func logInternalSync(level LogLevel, message string, keyValues ...any) {
	configMu.RLock()
	cfg := globalConfig
	configMu.RUnlock()

	if len(keyValues)%2 != 0 {
		// Handle odd number of arguments
		keyValues = append(keyValues, "MISSING_VALUE")
	}

	attrs := make([]slog.Attr, 0, len(keyValues)/2)
	for i := 0; i < len(keyValues); i += 2 {
		if i+1 < len(keyValues) {
			key := fmt.Sprintf("%v", keyValues[i])
			value := keyValues[i+1]
			value = redactValueIfNeeded(key, value, cfg)

			// Use the new convertToSlogAttr function for all types
			attrs = append(attrs, convertToSlogAttr(key, value))
		}
	}

	anyAttrs := make([]any, len(attrs))
	for i, attr := range attrs {
		anyAttrs[i] = attr
	}

	switch level {
	case Debug:
		defaultLogger.Debug(message, anyAttrs...)
	case Trace:
		defaultLogger.Log(context.Background(), LevelTrace, message, anyAttrs...)
	case Info:
		defaultLogger.Info(message, anyAttrs...)
	case Notice:
		defaultLogger.Log(context.Background(), LevelNotice, message, anyAttrs...)
	case Warn:
		defaultLogger.Warn(message, anyAttrs...)
	case Error:
		defaultLogger.Error(message, anyAttrs...)
	case Audit:
		defaultLogger.Log(context.Background(), LevelAudit, message, anyAttrs...)
	}
}

// slogLevelFromLogLevel converts LogLevel to slog.Level
func slogLevelFromLogLevel(level LogLevel) slog.Level {
	switch level {
	case Trace:
		return LevelTrace
	case Debug:
		return slog.LevelDebug
	case Info:
		return slog.LevelInfo
	case Notice:
		return LevelNotice
	case Warn:
		return slog.LevelWarn
	case Error:
		return slog.LevelError
	case Audit:
		return LevelAudit
	default:
		return slog.LevelInfo
	}
}

// GetStackTrace returns the current stack trace as a string
func GetStackTrace() string {
	return string(debug.Stack())
}

// ShouldRedactPath checks if a path should be completely redacted (exported for middleware)
func ShouldRedactPath(path string, cfg Config) bool {
	return shouldRedactPath(path, cfg)
}

// shouldRedactPath checks if a path should be completely redacted (internal)
func shouldRedactPath(path string, cfg Config) bool {
	for _, redactPath := range cfg.RedactPaths {
		if strings.Contains(path, redactPath) {
			return true
		}
	}
	return false
}
