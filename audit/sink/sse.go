package sink

import (
	"encoding/json"
	"fmt"
	"net/http"
	"sync"

	"github.com/jozefvalachovic/logger/v4/audit"
)

// SSESink streams audit events to connected Server-Sent Events (SSE) clients.
// Use Handler() to get an http.Handler for the SSE endpoint.
type SSESink struct {
	mu      sync.RWMutex
	clients map[chan []byte]struct{}
	closed  bool
}

// NewSSESink creates a new SSE streaming sink.
func NewSSESink() *SSESink {
	return &SSESink{
		clients: make(map[chan []byte]struct{}),
	}
}

// Write sends an audit entry to all connected SSE clients.
func (s *SSESink) Write(entry *audit.AuditEntry) error {
	data, err := json.Marshal(entry)
	if err != nil {
		return err
	}

	s.mu.RLock()
	defer s.mu.RUnlock()

	if s.closed {
		return fmt.Errorf("sse sink is closed")
	}

	for ch := range s.clients {
		select {
		case ch <- data:
		default:
			// Client is slow, skip this event
		}
	}
	return nil
}

// Flush is a no-op for SSE (events are pushed immediately).
func (s *SSESink) Flush() error { return nil }

// Close disconnects all SSE clients and closes the sink.
func (s *SSESink) Close() error {
	s.mu.Lock()
	defer s.mu.Unlock()

	s.closed = true
	for ch := range s.clients {
		close(ch)
		delete(s.clients, ch)
	}
	return nil
}

// Handler returns an http.Handler that serves the SSE event stream.
// Each connected client receives real-time audit events as JSON.
//
// Usage:
//
//	sseSink := sink.NewSSESink()
//	http.Handle("/audit/stream", sseSink.Handler())
func (s *SSESink) Handler() http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		flusher, ok := w.(http.Flusher)
		if !ok {
			http.Error(w, "streaming not supported", http.StatusInternalServerError)
			return
		}

		w.Header().Set("Content-Type", "text/event-stream")
		w.Header().Set("Cache-Control", "no-cache")
		w.Header().Set("Connection", "keep-alive")

		ch := make(chan []byte, 64)

		s.mu.Lock()
		if s.closed {
			s.mu.Unlock()
			http.Error(w, "sink closed", http.StatusServiceUnavailable)
			return
		}
		s.clients[ch] = struct{}{}
		s.mu.Unlock()

		defer func() {
			s.mu.Lock()
			delete(s.clients, ch)
			s.mu.Unlock()
		}()

		for {
			select {
			case data, ok := <-ch:
				if !ok {
					return
				}
				fmt.Fprintf(w, "data: %s\n\n", data)
				flusher.Flush()
			case <-r.Context().Done():
				return
			}
		}
	})
}

// Ensure SSESink implements the Sink interface.
var _ audit.Sink = (*SSESink)(nil)
