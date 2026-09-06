package logger

import (
	"context"
	"log/slog"
	"regexp"
)

// redactHandler sanitises records before they fan out to the pretty handler and
// any AdditionalHandlers, so every destination sees the same redacted values.
type redactHandler struct {
	inner      slog.Handler
	redactKeys []string
	mask       string
	patterns   []*regexp.Regexp
}

func newRedactHandler(inner slog.Handler, cfg Config, patterns []*regexp.Regexp) *redactHandler {
	return &redactHandler{
		inner:      inner,
		redactKeys: cfg.RedactKeys,
		mask:       cfg.RedactMask,
		patterns:   patterns,
	}
}

func (h *redactHandler) Enabled(ctx context.Context, level slog.Level) bool {
	return h.inner.Enabled(ctx, level)
}

func (h *redactHandler) Handle(ctx context.Context, record slog.Record) error {
	out := slog.NewRecord(record.Time, record.Level, record.Message, record.PC)
	record.Attrs(func(a slog.Attr) bool {
		out.AddAttrs(h.redactAttr(a))
		return true
	})
	return h.inner.Handle(ctx, out)
}

func (h *redactHandler) WithAttrs(attrs []slog.Attr) slog.Handler {
	redacted := make([]slog.Attr, 0, len(attrs))
	for _, a := range attrs {
		redacted = append(redacted, h.redactAttr(a))
	}
	return &redactHandler{
		inner:      h.inner.WithAttrs(redacted),
		redactKeys: h.redactKeys,
		mask:       h.mask,
		patterns:   h.patterns,
	}
}

func (h *redactHandler) WithGroup(name string) slog.Handler {
	return &redactHandler{
		inner:      h.inner.WithGroup(name),
		redactKeys: h.redactKeys,
		mask:       h.mask,
		patterns:   h.patterns,
	}
}

// redactAttr resolves LogValuers and walks groups so nested secrets are masked too.
func (h *redactHandler) redactAttr(a slog.Attr) slog.Attr {
	value := a.Value.Resolve()

	if value.Kind() == slog.KindGroup {
		group := value.Group()
		redacted := make([]slog.Attr, 0, len(group))
		for _, ga := range group {
			redacted = append(redacted, h.redactAttr(ga))
		}
		return slog.Attr{Key: a.Key, Value: slog.GroupValue(redacted...)}
	}

	if isSensitiveKey(a.Key, h.redactKeys) {
		return slog.String(a.Key, h.mask)
	}

	if value.Kind() == slog.KindString {
		s := value.String()
		for _, re := range h.patterns {
			if re.MatchString(s) {
				return slog.String(a.Key, h.mask)
			}
		}
	}

	return slog.Attr{Key: a.Key, Value: value}
}

func compileRedactPatterns(patterns []string) []*regexp.Regexp {
	var compiled []*regexp.Regexp
	for _, pattern := range patterns {
		if re, err := regexp.Compile(pattern); err == nil {
			compiled = append(compiled, re)
		}
	}
	return compiled
}
