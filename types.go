package logger

import (
	"io"
	"log/slog"
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
