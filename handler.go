package logger

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"log/slog"
	"time"
)

// prettyHandler is a custom slog.Handler that formats log records in a human-readable way
type prettyHandler struct {
	slog.Handler
	logger *log.Logger
	config Config
}

// prettyHandlerOptions holds configuration options for the prettyHandler
type prettyHandlerOptions struct {
	SlogOpts slog.HandlerOptions
	Config   Config
}

const (
	LevelDebug  = slog.LevelDebug
	LevelInfo   = slog.LevelInfo
	LevelNotice = slog.Level(2)
	LevelTrace  = slog.Level(-8)
	LevelWarn   = slog.LevelWarn
	LevelError  = slog.LevelError
)

// Handle formats and outputs the log record
func (handler *prettyHandler) Handle(ctx context.Context, record slog.Record) error {
	var recordLevel string

	// Use config.EnableColor to conditionally apply colors
	if handler.config.EnableColor {
		switch record.Level {
		case LevelDebug:
			recordLevel = formatString("DEBUG", purple, false)
		case LevelInfo:
			recordLevel = formatString("INFO", blue, false)
		case LevelNotice:
			recordLevel = formatString("NOTICE", green, false)
		case LevelTrace:
			recordLevel = formatString("TRACE", gray, false)
		case LevelWarn:
			recordLevel = formatString("WARN", yellow, false)
		case LevelError:
			recordLevel = formatString("ERROR", red, false)
		default:
			recordLevel = formatString(record.Level.String(), gray, false)
		}
	} else {
		switch record.Level {
		case LevelDebug:
			recordLevel = "DEBUG"
		case LevelInfo:
			recordLevel = "INFO"
		case LevelNotice:
			recordLevel = "NOTICE"
		case LevelTrace:
			recordLevel = "TRACE"
		case LevelWarn:
			recordLevel = "WARN"
		case LevelError:
			recordLevel = "ERROR"
		default:
			recordLevel = record.Level.String()
		}
	}

	recordAttrs := record.NumAttrs()

	fields := make(map[string]any, recordAttrs)
	record.Attrs(func(a slog.Attr) bool {
		if a.Key == "duration" {
			if duration, ok := a.Value.Any().(time.Duration); ok {
				fields[a.Key] = fmt.Sprintf("%.9fs", duration.Seconds())
			} else {
				fields[a.Key] = a.Value.Any()
			}
		} else {
			fields[a.Key] = a.Value.Any()
		}
		return true
	})

	jsonData, err := json.MarshalIndent(fields, "", "  ")
	if err != nil {
		return err
	}

	// Use config.TimeFormat
	timeStr := record.Time.Format(handler.config.TimeFormat)

	if record.Message == "" {
		if recordAttrs > 0 {
			handler.logger.Println(timeStr, recordLevel, string(jsonData))
		} else {
			handler.logger.Println(timeStr, recordLevel)
		}
	} else {
		msg := record.Message
		if handler.config.EnableColor {
			msg = formatString(record.Message, cyan, false)
		}
		if recordAttrs > 0 {
			handler.logger.Println(timeStr, recordLevel, msg, string(jsonData))
		} else {
			handler.logger.Println(timeStr, recordLevel, msg)
		}
	}

	return nil
}

// newPrettyHandler creates a new instance of prettyHandler with the given output and options
func newPrettyHandler(out io.Writer, opts prettyHandlerOptions) *prettyHandler {
	h := &prettyHandler{
		Handler: slog.NewJSONHandler(out, &opts.SlogOpts),
		logger:  log.New(out, "", 0),
		config:  opts.Config,
	}
	return h
}
