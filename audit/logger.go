package audit

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"sync"
	"time"
)

// Logger is the main enterprise audit logger
type Logger struct {
	mu               sync.RWMutex
	cfg              Config
	hashChain        *HashChain
	wal              *WAL
	rateLimiter      *RateLimiter
	retentionManager *RetentionManager
	sinks            []Sink
	store            Store
	buffer           chan *AuditEntry
	stopCh           chan struct{}
	doneCh           chan struct{}
	closed           bool
}

// New creates a new enterprise audit logger (alias for NewLogger)
func New(cfg Config) (*Logger, error) {
	return NewLogger(cfg)
}

// NewLogger creates a new enterprise audit logger
func NewLogger(cfg Config) (*Logger, error) {
	if err := cfg.Validate(); err != nil {
		return nil, err
	}

	l := &Logger{
		cfg:    cfg,
		sinks:  cfg.Sinks,
		store:  cfg.Store,
		buffer: make(chan *AuditEntry, cfg.BufferSize),
		stopCh: make(chan struct{}),
		doneCh: make(chan struct{}),
	}

	if cfg.HashChain.Enabled {
		l.hashChain = NewHashChain(cfg.HashChain)
	}

	if cfg.WAL.Enabled {
		wal, err := NewWAL(cfg.WAL)
		if err != nil {
			return nil, err
		}
		l.wal = wal

		uncommitted, err := wal.Recover()
		if err != nil {
			wal.Close()
			return nil, err
		}

		for _, entry := range uncommitted {
			if err := l.writeToSinks(entry); err == nil {
				wal.Commit(entry.ID)
			}
		}
	}

	if cfg.RateLimit != nil {
		l.rateLimiter = NewRateLimiter(cfg.RateLimit)
	}

	go l.processBuffer()

	return l, nil
}

// Log logs an audit event asynchronously
func (l *Logger) Log(ctx context.Context, event AuditEvent) error {
	if err := event.Validate(); err != nil {
		return err
	}

	l.mu.RLock()
	if l.closed {
		l.mu.RUnlock()
		return ErrAuditLoggerClosed
	}
	l.mu.RUnlock()

	if l.rateLimiter != nil {
		if !l.rateLimiter.Allow() {
			if l.rateLimiter.DropWhenLimited() {
				return ErrRateLimited
			}
			waitTime := l.rateLimiter.Wait()
			time.Sleep(waitTime)
		}
	}

	entry := l.createEntry(ctx, event)

	select {
	case l.buffer <- entry:
		return nil
	default:
		return l.processEntry(entry)
	}
}

// LogSync logs an audit event synchronously
func (l *Logger) LogSync(ctx context.Context, event AuditEvent) error {
	if err := event.Validate(); err != nil {
		return err
	}

	l.mu.RLock()
	if l.closed {
		l.mu.RUnlock()
		return ErrAuditLoggerClosed
	}
	l.mu.RUnlock()

	if l.rateLimiter != nil {
		if !l.rateLimiter.Allow() {
			if l.rateLimiter.DropWhenLimited() {
				return ErrRateLimited
			}
			waitTime := l.rateLimiter.Wait()
			time.Sleep(waitTime)
		}
	}

	entry := l.createEntry(ctx, event)
	return l.processEntry(entry)
}

// Query executes a query against the audit store
func (l *Logger) Query(q Query) (*QueryResult, error) {
	if l.store == nil {
		return nil, ErrStoreNotConfigured
	}
	return l.store.Query(q)
}

// Get retrieves a single audit entry by ID
func (l *Logger) Get(id string) (*AuditEntry, error) {
	if l.store == nil {
		return nil, ErrStoreNotConfigured
	}
	return l.store.Get(id)
}

// Flush flushes all buffered entries
func (l *Logger) Flush() error {
	for {
		select {
		case entry := <-l.buffer:
			if err := l.processEntry(entry); err != nil {
				return err
			}
		default:
			for _, sink := range l.sinks {
				if err := sink.Flush(); err != nil {
					return err
				}
			}
			if l.wal != nil {
				return l.wal.Flush()
			}
			return nil
		}
	}
}

// Close closes the audit logger
func (l *Logger) Close() error {
	l.mu.Lock()
	if l.closed {
		l.mu.Unlock()
		return nil
	}
	l.closed = true
	l.mu.Unlock()

	close(l.stopCh)
	<-l.doneCh

	l.Flush()

	var errs []error

	for _, sink := range l.sinks {
		if err := sink.Close(); err != nil {
			errs = append(errs, err)
		}
	}

	if l.wal != nil {
		if err := l.wal.Close(); err != nil {
			errs = append(errs, err)
		}
	}

	if l.store != nil {
		if err := l.store.Close(); err != nil {
			errs = append(errs, err)
		}
	}

	if l.retentionManager != nil {
		l.retentionManager.Stop()
	}

	if len(errs) > 0 {
		return fmt.Errorf("audit: close errors: %v", errs)
	}

	return nil
}

func (l *Logger) createEntry(ctx context.Context, event AuditEvent) *AuditEntry {
	entry := &AuditEntry{
		ID:            generateUUID(),
		Timestamp:     time.Now().UTC(),
		Event:         event,
		SchemaVersion: CurrentSchemaVersion,
	}

	if l.cfg.Service != nil {
		entry.Service = &ServiceInfo{
			Name:        l.cfg.Service.Name,
			Version:     l.cfg.Service.Version,
			Environment: l.cfg.Service.Environment,
			Region:      l.cfg.Service.Region,
			Instance:    l.cfg.Service.Instance,
			Namespace:   l.cfg.Service.Namespace,
		}
	}

	if l.cfg.Tracing.Enabled {
		if trace := TraceFromContext(ctx); trace != nil {
			entry.Trace = trace
		}
	}

	return entry
}

func (l *Logger) processEntry(entry *AuditEntry) error {
	if l.hashChain != nil {
		l.hashChain.Chain(entry)
	}

	if l.wal != nil {
		if err := l.wal.Write(entry); err != nil {
			return err
		}
	}

	if err := l.writeToSinks(entry); err != nil {
		return err
	}

	if l.wal != nil {
		if err := l.wal.Commit(entry.ID); err != nil {
			return err
		}
	}

	if l.store != nil {
		if err := l.store.Store(entry); err != nil {
			return err
		}
	}

	return nil
}

func (l *Logger) writeToSinks(entry *AuditEntry) error {
	if l.cfg.Output != nil && len(l.sinks) == 0 {
		return l.writeToOutput(entry)
	}

	var lastErr error
	for _, sink := range l.sinks {
		if err := sink.Write(entry); err != nil {
			lastErr = err
		}
	}
	return lastErr
}

func (l *Logger) writeToOutput(entry *AuditEntry) error {
	data, err := json.Marshal(entry)
	if err != nil {
		return err
	}
	data = append(data, '\n')

	l.mu.Lock()
	defer l.mu.Unlock()

	_, err = l.cfg.Output.Write(data)
	return err
}

func (l *Logger) processBuffer() {
	defer close(l.doneCh)

	flushInterval := l.cfg.FlushInterval
	if flushInterval == 0 {
		flushInterval = time.Second
	}

	ticker := time.NewTicker(flushInterval)
	defer ticker.Stop()

	for {
		select {
		case entry := <-l.buffer:
			l.processEntry(entry)
		case <-ticker.C:
			l.Flush()
		case <-l.stopCh:
			return
		}
	}
}

// SetRetentionManager sets the retention manager for log cleanup
func (l *Logger) SetRetentionManager(rm *RetentionManager) {
	l.retentionManager = rm
	if rm != nil {
		rm.Start()
	}
}

// LoggerStats contains statistics about the audit logger
type LoggerStats struct {
	Sequence   int64
	BufferSize int
	BufferUsed int
	Closed     bool
}

// GetStats returns current logger statistics
func (l *Logger) GetStats() LoggerStats {
	l.mu.RLock()
	defer l.mu.RUnlock()

	var sequence int64
	if l.hashChain != nil {
		sequence = l.hashChain.GetSequence()
	}

	return LoggerStats{
		Sequence:   sequence,
		BufferSize: cap(l.buffer),
		BufferUsed: len(l.buffer),
		Closed:     l.closed,
	}
}

// WriterSink creates a simple sink that writes JSON to an io.Writer
func WriterSink(w io.Writer) Sink {
	return &writerSink{w: w}
}

type writerSink struct {
	mu sync.Mutex
	w  io.Writer
}

func (s *writerSink) Write(entry *AuditEntry) error {
	data, err := json.Marshal(entry)
	if err != nil {
		return err
	}
	data = append(data, '\n')

	s.mu.Lock()
	defer s.mu.Unlock()

	_, err = s.w.Write(data)
	return err
}

func (s *writerSink) Flush() error {
	return nil
}

func (s *writerSink) Close() error {
	return nil
}
