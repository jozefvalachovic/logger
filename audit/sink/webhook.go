package sink

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"sync"
	"time"

	"github.com/jozefvalachovic/logger/v4/audit"
)

// WebhookSink sends audit entries to a webhook endpoint
type WebhookSink struct {
	mu          sync.Mutex
	endpoint    string
	client      *http.Client
	headers     map[string]string
	maxRetries  int
	retryDelay  time.Duration
	batchSize   int
	buffer      []*audit.AuditEntry
	flushTicker *time.Ticker
	stopCh      chan struct{}
	doneCh      chan struct{}
	closed      bool
}

// WebhookSinkConfig configures a webhook sink
type WebhookSinkConfig struct {
	Endpoint      string
	Headers       map[string]string
	Timeout       time.Duration
	MaxRetries    int
	RetryDelay    time.Duration
	BatchSize     int
	FlushInterval time.Duration
}

// NewWebhookSink creates a new webhook sink
func NewWebhookSink(cfg WebhookSinkConfig) *WebhookSink {
	timeout := cfg.Timeout
	if timeout == 0 {
		timeout = 30 * time.Second
	}

	maxRetries := cfg.MaxRetries
	if maxRetries == 0 {
		maxRetries = 3
	}

	retryDelay := cfg.RetryDelay
	if retryDelay == 0 {
		retryDelay = time.Second
	}

	batchSize := cfg.BatchSize
	if batchSize == 0 {
		batchSize = 100
	}

	flushInterval := cfg.FlushInterval
	if flushInterval == 0 {
		flushInterval = 5 * time.Second
	}

	ws := &WebhookSink{
		endpoint:   cfg.Endpoint,
		client:     &http.Client{Timeout: timeout},
		headers:    cfg.Headers,
		maxRetries: maxRetries,
		retryDelay: retryDelay,
		batchSize:  batchSize,
		buffer:     make([]*audit.AuditEntry, 0, batchSize),
		stopCh:     make(chan struct{}),
		doneCh:     make(chan struct{}),
	}

	ws.flushTicker = time.NewTicker(flushInterval)
	go ws.flushLoop()

	return ws
}

// Write adds an entry to the buffer
func (w *WebhookSink) Write(entry *audit.AuditEntry) error {
	w.mu.Lock()
	defer w.mu.Unlock()

	if w.closed {
		return fmt.Errorf("sink: webhook sink is closed")
	}

	w.buffer = append(w.buffer, entry)

	if len(w.buffer) >= w.batchSize {
		return w.flushLocked()
	}

	return nil
}

// Flush sends all buffered entries
func (w *WebhookSink) Flush() error {
	w.mu.Lock()
	defer w.mu.Unlock()
	return w.flushLocked()
}

func (w *WebhookSink) flushLocked() error {
	if len(w.buffer) == 0 {
		return nil
	}

	entries := w.buffer
	w.buffer = make([]*audit.AuditEntry, 0, w.batchSize)

	return w.sendWithRetry(entries)
}

func (w *WebhookSink) sendWithRetry(entries []*audit.AuditEntry) error {
	var lastErr error

	for attempt := 0; attempt <= w.maxRetries; attempt++ {
		if attempt > 0 {
			time.Sleep(w.retryDelay * time.Duration(attempt))
		}

		if err := w.send(entries); err != nil {
			lastErr = err
			continue
		}

		return nil
	}

	return fmt.Errorf("sink: webhook failed after %d retries: %w", w.maxRetries, lastErr)
}

func (w *WebhookSink) send(entries []*audit.AuditEntry) error {
	payload := struct {
		Entries []*audit.AuditEntry `json:"entries"`
		Count   int                 `json:"count"`
		SentAt  time.Time           `json:"sent_at"`
	}{
		Entries: entries,
		Count:   len(entries),
		SentAt:  time.Now().UTC(),
	}

	data, err := json.Marshal(payload)
	if err != nil {
		return fmt.Errorf("sink: failed to marshal payload: %w", err)
	}

	ctx, cancel := context.WithTimeout(context.Background(), w.client.Timeout)
	defer cancel()

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, w.endpoint, bytes.NewReader(data))
	if err != nil {
		return fmt.Errorf("sink: failed to create request: %w", err)
	}

	req.Header.Set("Content-Type", "application/json")
	for k, v := range w.headers {
		req.Header.Set(k, v)
	}

	resp, err := w.client.Do(req)
	if err != nil {
		return fmt.Errorf("sink: request failed: %w", err)
	}
	defer func() { _ = resp.Body.Close() }()

	if resp.StatusCode >= 400 {
		return fmt.Errorf("sink: webhook returned status %d", resp.StatusCode)
	}

	return nil
}

func (w *WebhookSink) flushLoop() {
	defer close(w.doneCh)

	for {
		select {
		case <-w.flushTicker.C:
			_ = w.Flush()
		case <-w.stopCh:
			return
		}
	}
}

// Close closes the webhook sink
func (w *WebhookSink) Close() error {
	w.mu.Lock()
	if w.closed {
		w.mu.Unlock()
		return nil
	}
	w.closed = true
	w.mu.Unlock()

	w.flushTicker.Stop()
	close(w.stopCh)
	<-w.doneCh

	return w.Flush()
}
