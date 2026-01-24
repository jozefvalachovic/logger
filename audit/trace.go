package audit

import (
	"context"
	"encoding/hex"
	"strings"
)

type contextKey string

const (
	TraceContextKey contextKey = "audit_trace_context"
)

// ExtractTraceContext extracts trace context from headers based on format
func ExtractTraceContext(cfg TracingConfig, getHeader func(string) string) *TraceInfo {
	if !cfg.Enabled {
		return nil
	}

	switch strings.ToLower(cfg.PropagationFormat) {
	case "w3c":
		return extractW3CTrace(getHeader)
	case "b3":
		return extractB3Trace(getHeader)
	case "b3-single":
		return extractB3SingleTrace(getHeader)
	case "jaeger":
		return extractJaegerTrace(getHeader)
	default:
		if cfg.TraceIDHeader != "" {
			return &TraceInfo{
				TraceID: getHeader(cfg.TraceIDHeader),
				SpanID:  getHeader(cfg.SpanIDHeader),
			}
		}
		return extractW3CTrace(getHeader)
	}
}

// extractW3CTrace extracts W3C Trace Context format
func extractW3CTrace(getHeader func(string) string) *TraceInfo {
	traceparent := getHeader("traceparent")
	if traceparent == "" {
		return nil
	}

	parts := strings.Split(traceparent, "-")
	if len(parts) < 4 {
		return nil
	}

	traceID := parts[1]
	spanID := parts[2]

	if len(traceID) != 32 || len(spanID) != 16 {
		return nil
	}

	if _, err := hex.DecodeString(traceID); err != nil {
		return nil
	}
	if _, err := hex.DecodeString(spanID); err != nil {
		return nil
	}

	return &TraceInfo{
		TraceID: traceID,
		SpanID:  spanID,
	}
}

// extractB3Trace extracts B3 multi-header format
func extractB3Trace(getHeader func(string) string) *TraceInfo {
	traceID := getHeader("X-B3-TraceId")
	if traceID == "" {
		return nil
	}

	return &TraceInfo{
		TraceID:      traceID,
		SpanID:       getHeader("X-B3-SpanId"),
		ParentSpanID: getHeader("X-B3-ParentSpanId"),
	}
}

// extractB3SingleTrace extracts B3 single-header format
func extractB3SingleTrace(getHeader func(string) string) *TraceInfo {
	b3 := getHeader("b3")
	if b3 == "" {
		return nil
	}

	if b3 == "0" {
		return nil
	}

	parts := strings.Split(b3, "-")
	if len(parts) < 2 {
		return nil
	}

	info := &TraceInfo{
		TraceID: parts[0],
		SpanID:  parts[1],
	}

	if len(parts) >= 4 {
		info.ParentSpanID = parts[3]
	}

	return info
}

// extractJaegerTrace extracts Jaeger format (uber-trace-id)
func extractJaegerTrace(getHeader func(string) string) *TraceInfo {
	uberTraceID := getHeader("uber-trace-id")
	if uberTraceID == "" {
		return nil
	}

	parts := strings.Split(uberTraceID, ":")
	if len(parts) < 4 {
		return nil
	}

	return &TraceInfo{
		TraceID:      parts[0],
		SpanID:       parts[1],
		ParentSpanID: parts[2],
	}
}

// WithTraceContext adds trace context to a context
func WithTraceContext(ctx context.Context, trace *TraceInfo) context.Context {
	if trace == nil {
		return ctx
	}
	return context.WithValue(ctx, TraceContextKey, trace)
}

// TraceFromContext extracts trace context from a context
func TraceFromContext(ctx context.Context) *TraceInfo {
	if ctx == nil {
		return nil
	}
	if trace, ok := ctx.Value(TraceContextKey).(*TraceInfo); ok {
		return trace
	}
	return nil
}
