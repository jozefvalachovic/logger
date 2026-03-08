package store

import (
	"slices"
	"sort"
	"sync"

	"github.com/jozefvalachovic/logger/v4/audit"
)

// MemoryStore is an in-memory audit log store for testing and development
type MemoryStore struct {
	mu      sync.RWMutex
	entries []audit.AuditEntry
	byID    map[string]int
	maxSize int
}

// MemoryStoreConfig configures a memory store
type MemoryStoreConfig struct {
	MaxSize int
}

// NewMemoryStore creates a new in-memory store
func NewMemoryStore(cfg MemoryStoreConfig) *MemoryStore {
	maxSize := cfg.MaxSize
	if maxSize == 0 {
		maxSize = 10000
	}

	return &MemoryStore{
		entries: make([]audit.AuditEntry, 0, maxSize),
		byID:    make(map[string]int),
		maxSize: maxSize,
	}
}

// Store stores an audit entry
func (s *MemoryStore) Store(entry *audit.AuditEntry) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	if len(s.entries) >= s.maxSize {
		oldest := s.entries[0]
		delete(s.byID, oldest.ID)
		s.entries = s.entries[1:]

		for id, idx := range s.byID {
			s.byID[id] = idx - 1
		}
	}

	s.byID[entry.ID] = len(s.entries)
	s.entries = append(s.entries, *entry)

	return nil
}

// Query queries the store
func (s *MemoryStore) Query(q audit.Query) (*audit.QueryResult, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	var matches []audit.AuditEntry

	for _, entry := range s.entries {
		if s.matchesQuery(entry, q) {
			matches = append(matches, entry)
		}
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

	start := min(q.Offset, total)

	end := min(start+q.Limit, total)

	result := &audit.QueryResult{
		Entries:    matches[start:end],
		Total:      int64(total),
		HasMore:    end < total,
		NextOffset: end,
	}

	return result, nil
}

// Get retrieves an entry by ID
func (s *MemoryStore) Get(id string) (*audit.AuditEntry, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	idx, ok := s.byID[id]
	if !ok {
		return nil, audit.ErrEntryNotFound
	}

	entry := s.entries[idx]
	return &entry, nil
}

// Close closes the store
func (s *MemoryStore) Close() error {
	return nil
}

// Count returns the number of entries
func (s *MemoryStore) Count() int {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return len(s.entries)
}

// Clear clears all entries
func (s *MemoryStore) Clear() {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.entries = s.entries[:0]
	s.byID = make(map[string]int)
}

func (s *MemoryStore) matchesQuery(entry audit.AuditEntry, q audit.Query) bool {
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

func containsString(slice []string, s string) bool {
	return slices.Contains(slice, s)
}

func containsEventType(slice []audit.AuditEventType, t audit.AuditEventType) bool {
	return slices.Contains(slice, t)
}

func containsOutcome(slice []audit.AuditOutcome, o audit.AuditOutcome) bool {
	return slices.Contains(slice, o)
}
