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

type Config struct {
	Output      io.Writer
	Level       slog.Level
	EnableColor bool
	TimeFormat  string
}
