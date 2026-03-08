package middleware

import (
	"fmt"
	"net/http"
	"strings"
	"sync/atomic"
	"time"

	"github.com/jozefvalachovic/logger/v4"
)

// WebSocketStats tracks WebSocket connection statistics.
type WebSocketStats struct {
	MessagesReceived atomic.Int64
	MessagesSent     atomic.Int64
	BytesReceived    atomic.Int64
	BytesSent        atomic.Int64
}

// LogWebSocketMiddleware wraps an HTTP handler to log WebSocket connection lifecycle events.
// It logs connection upgrades, message counts, and disconnections with duration.
//
// Usage:
//
//	http.Handle("/ws", middleware.LogWebSocketMiddleware(wsHandler))
func LogWebSocketMiddleware(next http.Handler, opts ...HTTPMiddlewareOption) http.Handler {
	options := DefaultHTTPMiddlewareOptions()
	for _, opt := range opts {
		opt(options)
	}

	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if !isWebSocketUpgrade(r) {
			next.ServeHTTP(w, r)
			return
		}

		start := time.Now()
		path := r.URL.Path
		remoteAddr := r.RemoteAddr

		logger.LogInfo(fmt.Sprintf("WebSocket upgrade %s", path),
			"ws.path", path,
			"ws.remote", remoteAddr,
			"ws.protocol", r.Header.Get("Sec-WebSocket-Protocol"),
		)

		wrapped := &wsWriter{ResponseWriter: w, stats: &WebSocketStats{}}

		defer func() {
			duration := time.Since(start)
			logger.LogInfo(fmt.Sprintf("WebSocket closed %s %s", path, duration),
				"ws.path", path,
				"ws.remote", remoteAddr,
				"ws.duration", duration.String(),
				"ws.messages_received", wrapped.stats.MessagesReceived.Load(),
				"ws.messages_sent", wrapped.stats.MessagesSent.Load(),
				"ws.bytes_received", wrapped.stats.BytesReceived.Load(),
				"ws.bytes_sent", wrapped.stats.BytesSent.Load(),
			)
		}()

		next.ServeHTTP(wrapped, r)
	})
}

// WebSocketStatsFromWriter returns the WebSocket stats tracker from a wrapped writer.
// Call this from your WebSocket handler to record message statistics.
func WebSocketStatsFromWriter(w http.ResponseWriter) *WebSocketStats {
	if ww, ok := w.(*wsWriter); ok {
		return ww.stats
	}
	return nil
}

type wsWriter struct {
	http.ResponseWriter
	stats *WebSocketStats
}

func (w *wsWriter) Write(b []byte) (int, error) {
	n, err := w.ResponseWriter.Write(b)
	w.stats.MessagesSent.Add(1)
	w.stats.BytesSent.Add(int64(n))
	return n, err
}

func (w *wsWriter) Flush() {
	if f, ok := w.ResponseWriter.(http.Flusher); ok {
		f.Flush()
	}
}

func isWebSocketUpgrade(r *http.Request) bool {
	for _, v := range r.Header.Values("Connection") {
		if strings.EqualFold(v, "upgrade") {
			if strings.EqualFold(r.Header.Get("Upgrade"), "websocket") {
				return true
			}
		}
	}
	return false
}
