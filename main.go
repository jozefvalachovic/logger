package logger

import (
	"context"
	"fmt"
	"io"
	"log/slog"
	"os"
)

type Config struct {
	Output      io.Writer
	Level       slog.Level
	EnableColor bool
	TimeFormat  string
	RedactKeys  []string
	RedactMask  string
}

// Global logger instance and configuration
var (
	defaultLogger *slog.Logger
	config        Config

	defaultConfig = Config{
		Output:      os.Stdout,
		Level:       LevelTrace,
		EnableColor: true,
		TimeFormat:  "2006-01-02 15:04:05",
		RedactKeys:  []string{"password", "secret", "token"},
		RedactMask:  "***",
	}
)

func init() {
	config = defaultConfig
	initLogger()
}

// initLogger initializes the default logger based on the current configuration
func initLogger() {
	opts := prettyHandlerOptions{
		SlogOpts: slog.HandlerOptions{
			Level: config.Level,
		},
		Config: config,
	}
	defaultLogger = slog.New(newPrettyHandler(config.Output, opts))
}

// logInternal is an internal function to log messages with key-value pairs
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
			value = redactValueIfNeeded(key, value, config)

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
	}
}
