package sink

import (
	"bufio"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"sync"
	"time"

	"github.com/jozefvalachovic/logger/v4/audit"
)

// FileSink writes audit entries to files with optional rotation
type FileSink struct {
	mu          sync.Mutex
	path        string
	file        *os.File
	writer      *bufio.Writer
	maxSize     int64
	currentSize int64
	rotateDaily bool
	currentDate string
	closed      bool
}

// FileSinkConfig configures a file sink
type FileSinkConfig struct {
	Path        string
	MaxSize     int64
	RotateDaily bool
	BufferSize  int
}

// NewFileSink creates a new file sink
func NewFileSink(cfg FileSinkConfig) (*FileSink, error) {
	dir := filepath.Dir(cfg.Path)
	if err := os.MkdirAll(dir, 0750); err != nil {
		return nil, fmt.Errorf("sink: failed to create directory: %w", err)
	}

	file, err := os.OpenFile(cfg.Path, os.O_CREATE|os.O_APPEND|os.O_WRONLY, 0640)
	if err != nil {
		return nil, fmt.Errorf("sink: failed to open file: %w", err)
	}

	info, err := file.Stat()
	if err != nil {
		file.Close()
		return nil, fmt.Errorf("sink: failed to stat file: %w", err)
	}

	bufferSize := cfg.BufferSize
	if bufferSize == 0 {
		bufferSize = 64 * 1024
	}

	maxSize := cfg.MaxSize
	if maxSize == 0 {
		maxSize = 100 << 20
	}

	return &FileSink{
		path:        cfg.Path,
		file:        file,
		writer:      bufio.NewWriterSize(file, bufferSize),
		maxSize:     maxSize,
		currentSize: info.Size(),
		rotateDaily: cfg.RotateDaily,
		currentDate: time.Now().Format("2006-01-02"),
	}, nil
}

// Write writes an audit entry to the file
func (s *FileSink) Write(entry *audit.AuditEntry) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	if s.closed {
		return fmt.Errorf("sink: file sink is closed")
	}

	if s.shouldRotate() {
		if err := s.rotate(); err != nil {
			return err
		}
	}

	data, err := json.Marshal(entry)
	if err != nil {
		return fmt.Errorf("sink: failed to marshal entry: %w", err)
	}

	data = append(data, '\n')
	n, err := s.writer.Write(data)
	if err != nil {
		return fmt.Errorf("sink: failed to write entry: %w", err)
	}

	s.currentSize += int64(n)
	return nil
}

// Flush flushes buffered data to disk
func (s *FileSink) Flush() error {
	s.mu.Lock()
	defer s.mu.Unlock()

	if s.closed {
		return nil
	}

	if err := s.writer.Flush(); err != nil {
		return err
	}
	return s.file.Sync()
}

// Close closes the file sink
func (s *FileSink) Close() error {
	s.mu.Lock()
	defer s.mu.Unlock()

	if s.closed {
		return nil
	}

	s.closed = true

	if err := s.writer.Flush(); err != nil {
		s.file.Close()
		return err
	}

	return s.file.Close()
}

func (s *FileSink) shouldRotate() bool {
	if s.maxSize > 0 && s.currentSize >= s.maxSize {
		return true
	}

	if s.rotateDaily {
		today := time.Now().Format("2006-01-02")
		if today != s.currentDate {
			return true
		}
	}

	return false
}

func (s *FileSink) rotate() error {
	if err := s.writer.Flush(); err != nil {
		return err
	}
	if err := s.file.Close(); err != nil {
		return err
	}

	timestamp := time.Now().Format("20060102-150405")
	ext := filepath.Ext(s.path)
	base := s.path[:len(s.path)-len(ext)]
	archivePath := fmt.Sprintf("%s.%s%s", base, timestamp, ext)

	if err := os.Rename(s.path, archivePath); err != nil {
		return fmt.Errorf("sink: failed to rotate file: %w", err)
	}

	file, err := os.OpenFile(s.path, os.O_CREATE|os.O_APPEND|os.O_WRONLY, 0640)
	if err != nil {
		return fmt.Errorf("sink: failed to create new file: %w", err)
	}

	s.file = file
	s.writer = bufio.NewWriterSize(file, 64*1024)
	s.currentSize = 0
	s.currentDate = time.Now().Format("2006-01-02")

	return nil
}
