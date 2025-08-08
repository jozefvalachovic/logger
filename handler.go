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

type prettyHandler struct {
	slog.Handler
	logger *log.Logger
	config Config
}

type prettyHandlerOptions struct {
	SlogOpts slog.HandlerOptions
	Config   Config
}

func (handler *prettyHandler) Handle(ctx context.Context, record slog.Record) error {
	var recordLevel = record.Level.String()

	// Use config.EnableColor to conditionally apply colors
	if handler.config.EnableColor {
		switch record.Level {
		case slog.LevelDebug:
			recordLevel = FormatString(recordLevel, Purple, false)
		case slog.LevelInfo:
			recordLevel = FormatString(recordLevel, Blue, false)
		case slog.LevelWarn:
			recordLevel = FormatString(recordLevel, Yellow, false)
		case slog.LevelError:
			recordLevel = FormatString(recordLevel, Red, false)
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
			msg = FormatString(record.Message, Cyan, false)
		}
		if recordAttrs > 0 {
			handler.logger.Println(timeStr, recordLevel, msg, string(jsonData))
		} else {
			handler.logger.Println(timeStr, recordLevel, msg)
		}
	}

	return nil
}

func newPrettyHandler(out io.Writer, opts prettyHandlerOptions) *prettyHandler {
	h := &prettyHandler{
		Handler: slog.NewJSONHandler(out, &opts.SlogOpts),
		logger:  log.New(out, "", 0),
		config:  opts.Config,
	}
	return h
}
