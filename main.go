package logger

import (
	"log/slog"
	"os"
)

// Log function
// Example: Log(helpers.Error, "Error occured in", "func", "error message")
func Log(level LogLevel, message string, keyValues ...string) {
	// Init logger
	opts := prettyHandlerOptions{
		SlogOpts: slog.HandlerOptions{
			Level: slog.LevelDebug,
		},
	}

	logger := slog.New(newPrettyHandler(os.Stdout, opts))

	attrs := make([]slog.Attr, 0, len(keyValues)/2)
	for i := 0; i < len(keyValues); i += 2 {
		if i+1 < len(keyValues) {
			attrs = append(attrs, slog.String(keyValues[i], keyValues[i+1]))
		}
	}

	anyAttrs := make([]any, len(attrs))
	for i, attr := range attrs {
		anyAttrs[i] = attr
	}

	switch level {
	case Debug:
		logger.Debug(message, anyAttrs...)
	case Error:
		logger.Error(message, anyAttrs...)
	case Info:
		logger.Info(message, anyAttrs...)
	case Warn:
		logger.Warn(message, anyAttrs...)
	}

}
