package logger

import (
	"context"
	"log/slog"
)

type OTelBridgeHandler struct {
	inner       slog.Handler
	serviceName string
	version     string
	attrs       []slog.Attr
}

func NewOTelBridgeHandler(inner slog.Handler, serviceName, version string) *OTelBridgeHandler {
	return &OTelBridgeHandler{
		inner:       inner,
		serviceName: serviceName,
		version:     version,
		attrs: []slog.Attr{
			slog.String("service.name", serviceName),
			slog.String("service.version", version),
		},
	}
}

func (h *OTelBridgeHandler) Enabled(ctx context.Context, level slog.Level) bool {
	return h.inner.Enabled(ctx, h.mapLevel(level))
}

func (h *OTelBridgeHandler) Handle(ctx context.Context, record slog.Record) error {
	// Pick the severity from the original level: after mapLevel only Trace
	// still matched, so Notice and Audit lost their severity attribute.
	severity := customSeverity(record.Level)
	// Clone before adding attrs so sibling handlers sharing the record's
	// backing array never see ours.
	record = record.Clone()
	record.Level = h.mapLevel(record.Level)
	record.AddAttrs(h.attrs...)
	if severity != "" {
		record.AddAttrs(slog.String("severity", severity))
	}
	return h.inner.Handle(ctx, record)
}

// customSeverity names the levels that have no slog equivalent.
func customSeverity(level slog.Level) string {
	switch level {
	case LevelTrace:
		return "TRACE"
	case LevelNotice:
		return "NOTICE"
	case LevelAudit:
		return "AUDIT"
	default:
		return ""
	}
}

func (h *OTelBridgeHandler) WithAttrs(attrs []slog.Attr) slog.Handler {
	return &OTelBridgeHandler{
		inner:       h.inner.WithAttrs(attrs),
		serviceName: h.serviceName,
		version:     h.version,
		attrs:       h.attrs,
	}
}

func (h *OTelBridgeHandler) WithGroup(name string) slog.Handler {
	return &OTelBridgeHandler{
		inner:       h.inner.WithGroup(name),
		serviceName: h.serviceName,
		version:     h.version,
		attrs:       h.attrs,
	}
}

func (h *OTelBridgeHandler) mapLevel(level slog.Level) slog.Level {
	switch level {
	case LevelTrace:
		return slog.LevelDebug - 4
	case LevelNotice:
		return slog.LevelInfo + 1
	case LevelAudit:
		return slog.LevelError + 4
	default:
		return level
	}
}

type LevelFilterHandler struct {
	minLevel slog.Level
	inner    slog.Handler
}

func NewLevelFilterHandler(minLevel slog.Level, inner slog.Handler) *LevelFilterHandler {
	return &LevelFilterHandler{minLevel: minLevel, inner: inner}
}

func (h *LevelFilterHandler) Enabled(_ context.Context, level slog.Level) bool {
	return level >= h.minLevel
}

func (h *LevelFilterHandler) Handle(ctx context.Context, record slog.Record) error {
	if record.Level < h.minLevel {
		return nil
	}
	return h.inner.Handle(ctx, record)
}

func (h *LevelFilterHandler) WithAttrs(attrs []slog.Attr) slog.Handler {
	return &LevelFilterHandler{minLevel: h.minLevel, inner: h.inner.WithAttrs(attrs)}
}

func (h *LevelFilterHandler) WithGroup(name string) slog.Handler {
	return &LevelFilterHandler{minLevel: h.minLevel, inner: h.inner.WithGroup(name)}
}
