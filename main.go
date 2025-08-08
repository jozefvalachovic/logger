package logger

import (
	"fmt"
	"log/slog"
	"os"
)

var defaultLogger *slog.Logger
var config Config

func init() {
	// Default configuration
	config = Config{
		Output:      os.Stdout,
		Level:       slog.LevelDebug,
		EnableColor: true,
		TimeFormat:  "2006-01-02 15:04:05",
	}
	initLogger()
}

func initLogger() {
	opts := prettyHandlerOptions{
		SlogOpts: slog.HandlerOptions{
			Level: config.Level,
		},
		Config: config,
	}
	defaultLogger = slog.New(newPrettyHandler(config.Output, opts))
}

// SetConfig configures the logger with custom settings.
// This will reinitialize the logger with the new configuration.
func SetConfig(cfg Config) {
	config = cfg
	initLogger()
}

// GetConfig returns the current logger configuration.
func GetConfig() Config {
	return config
}

// Log function - BACKWARD COMPATIBLE with v1
// Example: Log(Info, "User logged in", "username", "john", "id", 123, "rate", 3.14)
func Log(level LogLevel, message string, keyValues ...any) {
	logInternal(level, message, keyValues...)
}

// Internal logging function with modern any signature
func logInternal(level LogLevel, message string, keyValues ...any) {
	if len(keyValues)%2 != 0 {
		// Handle odd number of arguments
		keyValues = append(keyValues, "MISSING_VALUE")
	}

	attrs := make([]slog.Attr, 0, len(keyValues)/2)
	for i := 0; i < len(keyValues); i += 2 {
		if i+1 < len(keyValues) {
			key := fmt.Sprintf("%v", keyValues[i])
			value := keyValues[i+1]

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
	case Info:
		defaultLogger.Info(message, anyAttrs...)
	case Warn:
		defaultLogger.Warn(message, anyAttrs...)
	case Error:
		defaultLogger.Error(message, anyAttrs...)
	}
}

// NEW v2 Convenience functions - use modern any signature
// LogDebug logs a debug message with optional key-value pairs
func LogDebug(message string, keyValues ...any) {
	logInternal(Debug, message, keyValues...)
}

// LogInfo logs an info message with optional key-value pairs
func LogInfo(message string, keyValues ...any) {
	logInternal(Info, message, keyValues...)
}

// LogWarn logs a warning message with optional key-value pairs
func LogWarn(message string, keyValues ...any) {
	logInternal(Warn, message, keyValues...)
}

// LogError logs an error message with optional key-value pairs
func LogError(message string, keyValues ...any) {
	logInternal(Error, message, keyValues...)
}
