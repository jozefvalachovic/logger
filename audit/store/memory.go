package store

import (
	"slices"
	"sort"
	"sync"

	"github.com/jozefvalachovic/logger/v4/audit"
)

// MemoryStore is an in-memory audit log store for testing and development.
// Uses a ring buffer for O(1) eviction when at capacity.
type MemoryStore struct {
	mu      sync.RWMutex
	ring    []audit.AuditEntry // fixed-size ring buffer
	byID    map[string]int     // maps ID → ring index
	head    int                // next write position
	size    int                // current entry count (≤ maxSize)
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
		ring:    make([]audit.AuditEntry, maxSize),
		byID:    make(map[string]int, maxSize),
		maxSize: maxSize,
	}
}

// Store stores an audit entry
func (s *MemoryStore) Store(entry *audit.AuditEntry) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	if s.size >= s.maxSize {
		// Evict the oldest entry at the current head position (O(1))
		delete(s.byID, s.ring[s.head].ID)
	} else {
		s.size++
	}

	s.ring[s.head] = *entry
	s.byID[entry.ID] = s.head
	s.head = (s.head + 1) % s.maxSize

	return nil
}

// orderedEntries returns entries oldest-first for iteration.
func (s *MemoryStore) orderedEntries() []audit.AuditEntry {
	if s.size == 0 {
		return nil
	}
	result := make([]audit.AuditEntry, s.size)
	for i := range s.size {
		idx := (s.head - s.size + i + s.maxSize) % s.maxSize
		result[i] = s.ring[idx]
	}
	return result
}

// Query queries the store
func (s *MemoryStore) Query(q audit.Query) (*audit.QueryResult, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	var matches []audit.AuditEntry

	for _, entry := range s.orderedEntries() {
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

	entry := s.ring[idx]
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
	return s.size
}

// Clear clears all entries
func (s *MemoryStore) Clear() {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.size = 0
	s.head = 0
	clear(s.byID)
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
