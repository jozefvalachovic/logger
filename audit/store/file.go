package store

import (
	"bufio"
	"encoding/json"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"sync"

	"github.com/jozefvalachovic/logger/v4/audit"
)

// FileStore is a file-based audit log store
type FileStore struct {
	mu       sync.RWMutex
	basePath string
	pattern  string
}

// FileStoreConfig configures a file store
type FileStoreConfig struct {
	BasePath string
	Pattern  string
}

// NewFileStore creates a new file-based store
func NewFileStore(cfg FileStoreConfig) (*FileStore, error) {
	if err := os.MkdirAll(cfg.BasePath, 0750); err != nil {
		return nil, err
	}

	pattern := cfg.Pattern
	if pattern == "" {
		pattern = "*.jsonl"
	}

	return &FileStore{
		basePath: cfg.BasePath,
		pattern:  pattern,
	}, nil
}

// Store stores an audit entry (appends to current log file)
func (s *FileStore) Store(entry *audit.AuditEntry) error {
	return nil
}

// Query queries the file store
func (s *FileStore) Query(q audit.Query) (*audit.QueryResult, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	files, err := filepath.Glob(filepath.Join(s.basePath, s.pattern))
	if err != nil {
		return nil, err
	}

	var matches []audit.AuditEntry

	for _, file := range files {
		entries, err := s.readFile(file, q)
		if err != nil {
			continue
		}
		matches = append(matches, entries...)
	}

	if q.Descending {
		sort.Slice(matches, func(i, j int) bool {
			return matches[i].Timestamp.After(matches[j].Timestamp)
		})
	} else {
		sort.Slice(matches, func(i, j int) bool {
			return matches[i].Timestamp.Before(matches[j].Timestamp)
		})
	}

	total := len(matches)

	start := q.Offset
	if start > total {
		start = total
	}

	end := start + q.Limit
	if end > total {
		end = total
	}

	result := &audit.QueryResult{
		Entries:    matches[start:end],
		Total:      int64(total),
		HasMore:    end < total,
		NextOffset: end,
	}

	return result, nil
}

// Get retrieves an entry by ID
func (s *FileStore) Get(id string) (*audit.AuditEntry, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	files, err := filepath.Glob(filepath.Join(s.basePath, s.pattern))
	if err != nil {
		return nil, err
	}

	for _, file := range files {
		entry, err := s.findInFile(file, id)
		if err == nil && entry != nil {
			return entry, nil
		}
	}

	return nil, audit.ErrEntryNotFound
}

// Close closes the store
func (s *FileStore) Close() error {
	return nil
}

func (s *FileStore) readFile(path string, q audit.Query) ([]audit.AuditEntry, error) {
	file, err := os.Open(path)
	if err != nil {
		return nil, err
	}
	defer func() { _ = file.Close() }()

	var entries []audit.AuditEntry
	scanner := bufio.NewScanner(file)
	scanner.Buffer(make([]byte, 1024*1024), 10*1024*1024)

	for scanner.Scan() {
		line := scanner.Text()
		if strings.TrimSpace(line) == "" {
			continue
		}

		var entry audit.AuditEntry
		if err := json.Unmarshal([]byte(line), &entry); err != nil {
			continue
		}

		if s.matchesQuery(entry, q) {
			entries = append(entries, entry)
		}
	}

	return entries, scanner.Err()
}

func (s *FileStore) findInFile(path string, id string) (*audit.AuditEntry, error) {
	file, err := os.Open(path)
	if err != nil {
		return nil, err
	}
	defer func() { _ = file.Close() }()

	scanner := bufio.NewScanner(file)
	scanner.Buffer(make([]byte, 1024*1024), 10*1024*1024)

	for scanner.Scan() {
		line := scanner.Text()
		if strings.TrimSpace(line) == "" {
			continue
		}

		if !strings.Contains(line, id) {
			continue
		}

		var entry audit.AuditEntry
		if err := json.Unmarshal([]byte(line), &entry); err != nil {
			continue
		}

		if entry.ID == id {
			return &entry, nil
		}
	}

	return nil, scanner.Err()
}

func (s *FileStore) matchesQuery(entry audit.AuditEntry, q audit.Query) bool {
	if !q.TimeRange.Start.IsZero() && entry.Timestamp.Before(q.TimeRange.Start) {
		return false
	}
	if !q.TimeRange.End.IsZero() && entry.Timestamp.After(q.TimeRange.End) {
		return false
	}

	if len(q.EventTypes) > 0 && !containsEventType(q.EventTypes, entry.Event.Type) {
		return false
	}

	if len(q.ActorIDs) > 0 && !containsString(q.ActorIDs, entry.Event.Actor.ID) {
		return false
	}

	if len(q.ActorTypes) > 0 && !containsString(q.ActorTypes, entry.Event.Actor.Type) {
		return false
	}

	if len(q.ResourceIDs) > 0 {
		if entry.Event.Resource == nil || !containsString(q.ResourceIDs, entry.Event.Resource.ID) {
			return false
		}
	}

	if len(q.Actions) > 0 && !containsString(q.Actions, entry.Event.Action) {
		return false
	}

	if len(q.Outcomes) > 0 && !containsOutcome(q.Outcomes, entry.Event.Outcome) {
		return false
	}

	if q.TraceID != "" && (entry.Trace == nil || entry.Trace.TraceID != q.TraceID) {
		return false
	}

	return true
}
