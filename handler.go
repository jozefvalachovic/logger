package logger

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"log/slog"
	"path/filepath"
	"regexp"
	"runtime"
	"time"
)

// prettyHandler is a custom slog.Handler that formats log records in a human-readable way
type prettyHandler struct {
	slog.Handler
	logger         *log.Logger
	config         Config
	redactPatterns []*regexp.Regexp
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
	LevelAudit  = slog.Level(10) // Higher than Error for security audit logs
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
		case LevelAudit:
			recordLevel = formatString("AUDIT", brightCyan, false)
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
		case LevelAudit:
			recordLevel = "AUDIT"
		default:
			recordLevel = record.Level.String()
		}
	}

	// Caller attribution
	var caller string
	if handler.config.EnableCaller && record.PC != 0 {
		fs := runtime.CallersFrames([]uintptr{record.PC})
		f, _ := fs.Next()
		caller = fmt.Sprintf("%s:%d", filepath.Base(f.File), f.Line)
		if handler.config.EnableColor {
			caller = formatString(caller, gray, false)
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
			val := a.Value.Any()
			// Apply regex redaction to string values
			if s, ok := val.(string); ok {
				for _, re := range handler.redactPatterns {
					if re.MatchString(s) {
						val = handler.config.RedactMask
						break
					}
				}
			}
			fields[a.Key] = val
		}
		return true
	})

	var jsonStr string
	if recordAttrs > 0 {
		if handler.config.CompactJSON {
			jsonData, err := json.Marshal(fields)
			if err != nil {
				return err
			}
			jsonStr = string(jsonData)
		} else {
			jsonData, err := json.MarshalIndent(fields, "", "  ")
			if err != nil {
				return err
			}
			jsonStr = string(jsonData)
		}
		if handler.config.EnableColor && handler.config.ColorizeJSON {
			jsonStr = colorizeJSONOutput(jsonStr)
		}
	}

	// Use config.TimeFormat
	timeStr := record.Time.Format(handler.config.TimeFormat)

	// Build output parts
	parts := []any{timeStr, recordLevel}
	if caller != "" {
		parts = append(parts, "["+caller+"]")
	}
	if record.Message == "" {
		if recordAttrs > 0 {
			parts = append(parts, jsonStr)
		}
	} else {
		msg := record.Message
		if handler.config.EnableColor {
			msg = formatString(record.Message, cyan, false)
		}
		parts = append(parts, msg)
		if recordAttrs > 0 {
			parts = append(parts, jsonStr)
		}
	}

	handler.logger.Println(parts...)

	return nil
}

var jsonKeyColorRe = regexp.MustCompile(`("(?:[^"\\]|\\.)*")\s*:`)

func colorizeJSONOutput(jsonStr string) string {
	return jsonKeyColorRe.ReplaceAllStringFunc(jsonStr, func(match string) string {
		// Find last quote before the colon
		for i := len(match) - 1; i >= 0; i-- {
			if match[i] == '"' {
				key := match[:i+1]
				rest := match[i+1:]
				return formatString(key, blue, false) + rest
			}
		}
		return match
	})
}

// newPrettyHandler creates a new instance of prettyHandler with the given output and options
func newPrettyHandler(out io.Writer, opts prettyHandlerOptions) *prettyHandler {
	h := &prettyHandler{
		Handler: slog.NewJSONHandler(out, &opts.SlogOpts),
		logger:  log.New(out, "", 0),
		config:  opts.Config,
	}
	for _, pattern := range opts.Config.RedactPatterns {
		if re, err := regexp.Compile(pattern); err == nil {
			h.redactPatterns = append(h.redactPatterns, re)
		}
	}
	return h
}
