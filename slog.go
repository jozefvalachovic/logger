package logger

import (
	"context"
	"log/slog"
	"os"
	"slices"
	"sync"
)

// forwardOp records a WithAttrs or WithGroup call so it can be replayed against
// whichever handler is active when a record is actually written.
type forwardOp struct {
	group string
	attrs []slog.Attr
}

// forwardHandler resolves the active pipeline handler per record instead of
// capturing it, so a cached *slog.Logger still observes later SetConfig calls.
type forwardHandler struct {
	ops []forwardOp
}

var fallbackHandler = sync.OnceValue(func() slog.Handler {
	return slog.NewJSONHandler(os.Stderr, nil)
})

func (h *forwardHandler) current() slog.Handler {
	root := defaultLogger.Load()
	if root == nil {
		return fallbackHandler()
	}
	handler := root.Handler()
	for _, op := range h.ops {
		if op.group != "" {
			handler = handler.WithGroup(op.group)
			continue
		}
		handler = handler.WithAttrs(op.attrs)
	}
	return handler
}

func (h *forwardHandler) Enabled(ctx context.Context, level slog.Level) bool {
	return h.current().Enabled(ctx, level)
}

func (h *forwardHandler) Handle(ctx context.Context, record slog.Record) error {
	return h.current().Handle(ctx, record)
}

func (h *forwardHandler) WithAttrs(attrs []slog.Attr) slog.Handler {
	if len(attrs) == 0 {
		return h
	}
	return &forwardHandler{ops: append(h.clonedOps(), forwardOp{attrs: slices.Clone(attrs)})}
}

func (h *forwardHandler) WithGroup(name string) slog.Handler {
	if name == "" {
		return h
	}
	return &forwardHandler{ops: append(h.clonedOps(), forwardOp{group: name})}
}

func (h *forwardHandler) clonedOps() []forwardOp {
	return slices.Clip(slices.Clone(h.ops))
}

// Slog returns a *slog.Logger backed by this pipeline, for dependencies that
// accept a *slog.Logger and cannot be given anything else.
//
// The returned logger resolves the active handler on every record, so it keeps
// working across SetConfig calls and is safe to cache for the process lifetime.
// Records written through it get the configured level, output and rotation,
// AdditionalHandlers, and both key and pattern redaction. Sampling,
// deduplication, metrics and async buffering are applied by the Log* functions
// and do NOT apply to this path.
func Slog() *slog.Logger {
	return slog.New(&forwardHandler{})
}

// SetAsSlogDefault routes log/slog's package-level default through this
// pipeline, so a dependency that reaches for slog.Default() is covered too.
//
// This reassigns a process-wide global and is therefore opt-in; the package
// never calls it on import.
func SetAsSlogDefault() {
	slog.SetDefault(Slog())
}
