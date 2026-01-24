package audit

import (
	"bufio"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"sync"
	"time"
)

// WAL provides write-ahead logging for guaranteed delivery
type WAL struct {
	mu          sync.Mutex
	path        string
	file        *os.File
	writer      *bufio.Writer
	syncOnWrite bool
	maxSize     int64
	currentSize int64
	closed      bool
}

// NewWAL creates a new write-ahead log
func NewWAL(cfg WALConfig) (*WAL, error) {
	if cfg.Path == "" {
		return nil, ErrWALPathRequired
	}

	dir := filepath.Dir(cfg.Path)
	if err := os.MkdirAll(dir, 0750); err != nil {
		return nil, fmt.Errorf("audit: failed to create WAL directory: %w", err)
	}

	file, err := os.OpenFile(cfg.Path, os.O_CREATE|os.O_APPEND|os.O_WRONLY, 0640)
	if err != nil {
		return nil, fmt.Errorf("audit: failed to open WAL file: %w", err)
	}

	info, err := file.Stat()
	if err != nil {
		file.Close()
		return nil, fmt.Errorf("audit: failed to stat WAL file: %w", err)
	}

	maxSize := cfg.MaxSize
	if maxSize == 0 {
		maxSize = 100 << 20
	}

	wal := &WAL{
		path:        cfg.Path,
		file:        file,
		writer:      bufio.NewWriterSize(file, 64*1024),
		syncOnWrite: cfg.SyncOnWrite,
		maxSize:     maxSize,
		currentSize: info.Size(),
	}

	return wal, nil
}

// Write writes an entry to the WAL
func (w *WAL) Write(entry *AuditEntry) error {
	w.mu.Lock()
	defer w.mu.Unlock()

	if w.closed {
		return ErrAuditLoggerClosed
	}

	if w.currentSize >= w.maxSize {
		if err := w.rotate(); err != nil {
			return err
		}
	}

	record := walRecord{
		Timestamp: time.Now().UnixNano(),
		Entry:     entry,
		Committed: false,
	}

	data, err := json.Marshal(record)
	if err != nil {
		return fmt.Errorf("audit: failed to marshal WAL record: %w", err)
	}

	data = append(data, '\n')
	n, err := w.writer.Write(data)
	if err != nil {
		return fmt.Errorf("audit: failed to write to WAL: %w", err)
	}
	w.currentSize += int64(n)

	if w.syncOnWrite {
		if err := w.writer.Flush(); err != nil {
			return err
		}
		if err := w.file.Sync(); err != nil {
			return err
		}
	}

	return nil
}

// Commit marks an entry as committed (successfully written to sinks)
func (w *WAL) Commit(entryID string) error {
	w.mu.Lock()
	defer w.mu.Unlock()

	if w.closed {
		return ErrAuditLoggerClosed
	}

	record := walCommit{
		Timestamp: time.Now().UnixNano(),
		EntryID:   entryID,
	}

	data, err := json.Marshal(record)
	if err != nil {
		return fmt.Errorf("audit: failed to marshal commit record: %w", err)
	}

	data = append(data, '\n')
	n, err := w.writer.Write(data)
	if err != nil {
		return fmt.Errorf("audit: failed to write commit to WAL: %w", err)
	}
	w.currentSize += int64(n)

	if w.syncOnWrite {
		if err := w.writer.Flush(); err != nil {
			return err
		}
		return w.file.Sync()
	}

	return nil
}

// Flush flushes buffered data to disk
func (w *WAL) Flush() error {
	w.mu.Lock()
	defer w.mu.Unlock()

	if w.closed {
		return nil
	}

	if err := w.writer.Flush(); err != nil {
		return err
	}
	return w.file.Sync()
}

// Close closes the WAL file
func (w *WAL) Close() error {
	w.mu.Lock()
	defer w.mu.Unlock()

	if w.closed {
		return nil
	}

	w.closed = true

	if err := w.writer.Flush(); err != nil {
		w.file.Close()
		return err
	}

	return w.file.Close()
}

// Recover reads uncommitted entries from the WAL for replay
func (w *WAL) Recover() ([]*AuditEntry, error) {
	w.mu.Lock()
	defer w.mu.Unlock()

	file, err := os.Open(w.path)
	if err != nil {
		if os.IsNotExist(err) {
			return nil, nil
		}
		return nil, fmt.Errorf("audit: failed to open WAL for recovery: %w", err)
	}
	defer file.Close()

	uncommitted := make(map[string]*AuditEntry)
	scanner := bufio.NewScanner(file)
	scanner.Buffer(make([]byte, 1024*1024), 10*1024*1024)

	for scanner.Scan() {
		line := scanner.Bytes()
		if len(line) == 0 {
			continue
		}

		var record walRecord
		if err := json.Unmarshal(line, &record); err == nil && record.Entry != nil {
			uncommitted[record.Entry.ID] = record.Entry
			continue
		}

		var commit walCommit
		if err := json.Unmarshal(line, &commit); err == nil && commit.EntryID != "" {
			delete(uncommitted, commit.EntryID)
		}
	}

	if err := scanner.Err(); err != nil {
		return nil, fmt.Errorf("audit: failed to read WAL: %w", err)
	}

	result := make([]*AuditEntry, 0, len(uncommitted))
	for _, entry := range uncommitted {
		result = append(result, entry)
	}

	return result, nil
}

func (w *WAL) rotate() error {
	if err := w.writer.Flush(); err != nil {
		return err
	}
	if err := w.file.Sync(); err != nil {
		return err
	}
	if err := w.file.Close(); err != nil {
		return err
	}

	timestamp := time.Now().Format("20060102-150405")
	archivePath := fmt.Sprintf("%s.%s", w.path, timestamp)
	if err := os.Rename(w.path, archivePath); err != nil {
		return fmt.Errorf("audit: failed to rotate WAL: %w", err)
	}

	file, err := os.OpenFile(w.path, os.O_CREATE|os.O_APPEND|os.O_WRONLY, 0640)
	if err != nil {
		return fmt.Errorf("audit: failed to create new WAL file: %w", err)
	}

	w.file = file
	w.writer = bufio.NewWriterSize(file, 64*1024)
	w.currentSize = 0

	return nil
}

type walRecord struct {
	Timestamp int64       `json:"ts"`
	Entry     *AuditEntry `json:"entry,omitempty"`
	Committed bool        `json:"committed,omitempty"`
}

type walCommit struct {
	Timestamp int64  `json:"ts"`
	EntryID   string `json:"commit,omitempty"`
}
