package logger

import "context"

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

// LogDebug logs a debug message with optional key-value pairs
func LogDebug(message string, keyValues ...any) {
	logInternal(Debug, message, keyValues...)
}

// LogInfo logs an info message with optional key-value pairs
func LogInfo(message string, keyValues ...any) {
	logInternal(Info, message, keyValues...)
}

// LogInfo logs an info message with optional key-value pairs
func LogInfoWithContext(ctx context.Context, message string, keyValues ...any) {
	// Extract trace ID, request ID from context if available
	if traceID := ctx.Value("trace_id"); traceID != nil {
		keyValues = append(keyValues, "trace_id", traceID)
	}
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

// Log function - BACKWARD COMPATIBLE with v1
// Example: Log(Info, "User logged in", "username", "john", "id", 123, "rate", 3.14)
func Log(level LogLevel, message string, keyValues ...any) {
	logInternal(level, message, keyValues...)
}
