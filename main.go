// Package logger provides a high-performance structured logger with colorized output,
// audit logging, sampling, rotation, async writes, and metrics collection.
//
// The package uses a global singleton configuration by default via SetConfig/GetConfig
// and the top-level Log* functions. For applications that need multiple independent logger
// instances with different configurations, use the [Logger] interface via [DefaultLogger]
// or implement custom instances. The global state is safe for concurrent use.
package logger

import (
	"context"
	"fmt"
	"io"
	"log/slog"
	"os"
	"regexp"
	"runtime"
	"runtime/debug"
	"strings"
	"sync"
	"sync/atomic"
	"time"

	"github.com/jozefvalachovic/logger/v4/audit"
)

type Config struct {
	Output      io.Writer
	Level       slog.Level
	LevelSet    bool // Explicitly marks Level as set (allows setting Level to 0/slog.LevelDebug)
	EnableColor bool
	TimeFormat  string
	RedactKeys  []string
	RedactMask  string
	MaxBodySize int64    // Maximum size for HTTP body logging in bytes (default: 1MB)
	RedactPaths []string // URL paths to completely redact from logs

	// Sampling configuration
	SampleRate    float64 // 0.0 to 1.0, where 0.1 = log 10% of messages (default: 1.0 = all)
	SampleRateSet bool    // Explicitly marks SampleRate as set (allows setting to 0.0)
	SampleSeed    int64   // Seed for deterministic sampling

	// Rotation configuration
	Rotation *RotationConfig

	// Async logging configuration
	AsyncMode    bool
	BufferSize   int           // Channel buffer size for async mode (default: 1000)
	FlushTimeout time.Duration // How often to flush in async mode (default: 1s)

	// Metrics configuration
	EnableMetrics bool
	MetricsPrefix string // Prefix for metric names (default: "logger")

	// Caller attribution: includes source file:line in log output
	EnableCaller bool

	// Regex-based value redaction patterns (applied to all string values)
	RedactPatterns []string

	// Output format options
	CompactJSON  bool // Single-line JSON instead of indented
	ColorizeJSON bool // Colorize JSON keys (requires EnableColor)

	// Deduplication: suppress repeated identical messages within a window
	EnableDedup bool
	DedupWindow time.Duration // Default: 5s

	// AdditionalHandlers allows sending log output to multiple destinations
	// using slog.NewMultiHandler (Go 1.26+). The prettyHandler is always included.
	AdditionalHandlers []slog.Handler

	// Enterprise Audit configuration (nil = use legacy LogAudit behavior)
	Audit *audit.Config
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
	for _, p := range c.RedactPatterns {
		if _, err := regexp.Compile(p); err != nil {
			return fmt.Errorf("invalid redact pattern %q: %w", p, err)
		}
	}
	if c.Audit != nil {
		if err := c.Audit.Validate(); err != nil {
			return fmt.Errorf("audit config: %w", err)
		}
	}
	return nil
}

// Global logger instance and configuration
var (
	defaultLogger *slog.Logger
	globalConfig  atomic.Pointer[Config]

	// configWriteMu serialises SetConfig calls so that read-modify writes
	// (e.g. comparing old vs new async/audit config) are safe.
	configWriteMu sync.Mutex

	// Async logging
	logChan      chan *logEntry
	asyncDone    chan bool
	asyncRunning bool
	asyncMu      sync.Mutex
	asyncWg      sync.WaitGroup // Tracks if async goroutine is running

	// Metrics
	metrics *LogMetrics

	// Enterprise audit logger (nil when using legacy behavior)
	auditLogger *audit.Logger

	defaultConfig = Config{
		Output:        os.Stdout,
		Level:         LevelTrace,
		EnableColor:   true,
		CompactJSON:   true, // Single-line JSON by default for production log aggregators
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
		DedupWindow:   5 * time.Second,
		Audit:         nil, // Legacy behavior by default
	}
)

func init() {
	cfg := defaultConfig
	applyEnvOverrides(&cfg)
	globalConfig.Store(&cfg)
	initLogger()
}

// initLogger initializes the default logger based on the current configuration
func initLogger() {
	cfg := *globalConfig.Load()

	opts := prettyHandlerOptions{
		SlogOpts: slog.HandlerOptions{
			Level:     cfg.Level,
			AddSource: cfg.EnableCaller,
		},
		Config: cfg,
	}

	var handler slog.Handler = newPrettyHandler(cfg.Output, opts)
	if len(cfg.AdditionalHandlers) > 0 {
		allHandlers := make([]slog.Handler, 0, len(cfg.AdditionalHandlers)+1)
		allHandlers = append(allHandlers, handler)
		allHandlers = append(allHandlers, cfg.AdditionalHandlers...)
		handler = slog.NewMultiHandler(allHandlers...)
	}
	defaultLogger = slog.New(handler)

	// Sync the stdlib log package level with our configured level
	// so log.Print/log.Printf respect the same threshold (Go 1.26+).
	slog.SetLogLoggerLevel(cfg.Level)
}

// logInternal is an internal function to log messages with key-value pairs
func logInternal(level LogLevel, message string, keyValues ...any) {
	// Lazy evaluation: skip expensive operations if log level doesn't match
	cfg := *globalConfig.Load()

	if cfg.Level > slogLevelFromLogLevel(level) {
		return // Early return - don't process if we won't log anyway
	}

	// Apply sampling
	if cfg.SampleRate < 1.0 && !shouldSample(message, cfg.SampleRate, cfg.SampleSeed) {
		return
	}

	// Apply deduplication
	if cfg.EnableDedup && dedupMgr != nil {
		if !dedupMgr.ShouldLog(level, message) {
			return
		}
	}

	// Track metrics
	if cfg.EnableMetrics && metrics != nil {
		metrics.RecordLog(level)
	}

	// Capture caller PC for source attribution
	var pc uintptr
	if cfg.EnableCaller {
		var pcs [1]uintptr
		runtime.Callers(3, pcs[:])
		pc = pcs[0]
	}

	// Use async logging if enabled
	if cfg.AsyncMode && asyncRunning {
		entry := &logEntry{
			level:     level,
			message:   message,
			keyValues: keyValues,
			pc:        pc,
		}
		select {
		case logChan <- entry:
			// Successfully queued
		default:
			// Channel full, fall back to sync logging
			logInternalSync(level, message, pc, keyValues...)
		}
		return
	}

	// Synchronous logging
	logInternalSync(level, message, pc, keyValues...)
}

// logInternalSync performs synchronous logging (used by both sync and async paths)
func logInternalSync(level LogLevel, message string, pc uintptr, keyValues ...any) {
	cfg := *globalConfig.Load()

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

	slogLevel := slogLevelFromLogLevel(level)
	record := slog.NewRecord(time.Now(), slogLevel, message, pc)
	record.AddAttrs(attrs...)
	_ = defaultLogger.Handler().Handle(context.Background(), record)
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
