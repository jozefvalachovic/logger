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
	"slices"
	"time"
)

// prettyHandler is a custom slog.Handler that formats log records in a human-readable way
type prettyHandler struct {
	logger *log.Logger
	config Config
	attrs  []slog.Attr
	groups []string
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

	recordAttrs := make([]slog.Attr, 0, record.NumAttrs())
	record.Attrs(func(a slog.Attr) bool {
		recordAttrs = append(recordAttrs, a)
		return true
	})

	fields := make(map[string]any, len(handler.attrs)+len(recordAttrs))
	mergeAttrsInto(fields, handler.attrs)
	mergeAttrsInto(fields, wrapGroups(handler.groups, recordAttrs))

	var jsonStr string
	if len(fields) > 0 {
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
		if jsonStr != "" {
			parts = append(parts, jsonStr)
		}
	} else {
		msg := record.Message
		if handler.config.EnableColor {
			msg = formatString(record.Message, cyan, false)
		}
		parts = append(parts, msg)
		if jsonStr != "" {
			parts = append(parts, jsonStr)
		}
	}

	handler.logger.Println(parts...)

	return nil
}

func (handler *prettyHandler) Enabled(_ context.Context, level slog.Level) bool {
	return level >= handler.config.Level
}

func (handler *prettyHandler) WithAttrs(attrs []slog.Attr) slog.Handler {
	if len(attrs) == 0 {
		return handler
	}
	next := handler.clone()
	next.attrs = append(next.attrs, wrapGroups(handler.groups, attrs)...)
	return next
}

func (handler *prettyHandler) WithGroup(name string) slog.Handler {
	if name == "" {
		return handler
	}
	next := handler.clone()
	next.groups = append(next.groups, name)
	return next
}

// clone clips the copied slices so sibling handlers never share a backing array.
func (handler *prettyHandler) clone() *prettyHandler {
	return &prettyHandler{
		logger: handler.logger,
		config: handler.config,
		attrs:  slices.Clip(slices.Clone(handler.attrs)),
		groups: slices.Clip(slices.Clone(handler.groups)),
	}
}

// wrapGroups nests attrs under the open group chain, outermost group first.
func wrapGroups(groups []string, attrs []slog.Attr) []slog.Attr {
	if len(attrs) == 0 {
		return nil
	}
	for _, group := range slices.Backward(groups) {
		attrs = []slog.Attr{{Key: group, Value: slog.GroupValue(attrs...)}}
	}
	return attrs
}

// mergeAttrsInto flattens attrs into dst, resolving LogValuers and nesting groups.
func mergeAttrsInto(dst map[string]any, attrs []slog.Attr) {
	for _, a := range attrs {
		value := a.Value.Resolve()

		if value.Kind() == slog.KindGroup {
			group := value.Group()
			if len(group) == 0 {
				continue
			}
			if a.Key == "" {
				mergeAttrsInto(dst, group)
				continue
			}
			sub, ok := dst[a.Key].(map[string]any)
			if !ok {
				sub = make(map[string]any, len(group))
				dst[a.Key] = sub
			}
			mergeAttrsInto(sub, group)
			continue
		}

		if a.Key == "" {
			continue
		}

		if a.Key == "duration" {
			if duration, ok := value.Any().(time.Duration); ok {
				dst[a.Key] = fmt.Sprintf("%.9fs", duration.Seconds())
				continue
			}
		}

		dst[a.Key] = value.Any()
	}
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
func newPrettyHandler(out io.Writer, cfg Config) *prettyHandler {
	return &prettyHandler{
		logger: log.New(out, "", 0),
		config: cfg,
	}
}
