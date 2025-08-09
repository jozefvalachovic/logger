package logger

import (
	"io"
	"log/slog"
	"net/http"
)

type LogLevel int

const (
	Debug LogLevel = iota
	Info
	Warn
	Error
)

// Config is exported for users to configure the logger
type Config struct {
	Output      io.Writer
	Level       slog.Level
	EnableColor bool
	TimeFormat  string
}

// wrappedWriter is used to capture the status code of HTTP responses
// This is used in the HTTP handler to log the status code of responses
type wrappedWriter struct {
	http.ResponseWriter
	statusCode int
}

// Internal unexported types
type color int

const (
	blue color = iota
	cyan
	green
	purple
	red
	yellow
)
