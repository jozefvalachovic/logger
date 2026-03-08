package store

import (
	"database/sql"
	"encoding/json"
	"fmt"
	"strings"
	"unicode"

	"github.com/jozefvalachovic/logger/v4/audit"
)

// SQLStore implements the audit.Store interface using database/sql.
// It works with any SQL driver (e.g., pgx for PostgreSQL, go-sqlite3, mysql).
// Users must provide their own driver import and database connection.
type SQLStore struct {
	db    *sql.DB
	table string
}

// NewSQLStore creates a new SQL-backed audit store.
// The table is created automatically if it doesn't exist.
// The table name must contain only letters, digits, and underscores.
func NewSQLStore(db *sql.DB, table string) (*SQLStore, error) {
	if table == "" {
		table = "audit_log"
	}
	if !isValidTableName(table) {
		return nil, fmt.Errorf("audit sql store: invalid table name %q", table)
	}

	s := &SQLStore{db: db, table: table}
	if err := s.createTable(); err != nil {
		return nil, fmt.Errorf("audit sql store: create table: %w", err)
	}
	return s, nil
}

func isValidTableName(name string) bool {
	if len(name) == 0 || len(name) > 64 {
		return false
	}
	for _, c := range name {
		if !unicode.IsLetter(c) && !unicode.IsDigit(c) && c != '_' {
			return false
		}
	}
	return true
}

func (s *SQLStore) createTable() error {
	query := `CREATE TABLE IF NOT EXISTS ` + s.table + ` (
		id TEXT PRIMARY KEY,
		timestamp TEXT NOT NULL,
		event_type TEXT NOT NULL,
		action TEXT NOT NULL,
		outcome TEXT NOT NULL,
		actor_id TEXT,
		actor_type TEXT,
		resource_id TEXT,
		resource_type TEXT,
		hash TEXT,
		previous_hash TEXT,
		sequence BIGINT,
		data TEXT
	)`
	_, err := s.db.Exec(query)
	return err
}

// Store persists an audit entry.
func (s *SQLStore) Store(entry *audit.AuditEntry) error {
	data, err := json.Marshal(entry)
	if err != nil {
		return err
	}

	query := `INSERT INTO ` + s.table +
		` (id, timestamp, event_type, action, outcome, actor_id, actor_type, resource_id, resource_type, hash, previous_hash, sequence, data)
		VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)`

	var resourceID, resourceType string
	if entry.Event.Resource != nil {
		resourceID = entry.Event.Resource.ID
		resourceType = entry.Event.Resource.Type
	}

	_, err = s.db.Exec(query,
		entry.ID,
		entry.Timestamp.Format("2006-01-02T15:04:05.999999999Z07:00"),
		string(entry.Event.Type),
		entry.Event.Action,
		string(entry.Event.Outcome),
		entry.Event.Actor.ID,
		entry.Event.Actor.Type,
		resourceID,
		resourceType,
		entry.Hash,
		entry.PreviousHash,
		entry.Sequence,
		string(data),
	)
	return err
}

// Get retrieves a single audit entry by ID.
func (s *SQLStore) Get(id string) (*audit.AuditEntry, error) {
	query := `SELECT data FROM ` + s.table + ` WHERE id = ?`
	row := s.db.QueryRow(query, id)

	var data string
	if err := row.Scan(&data); err != nil {
		if err == sql.ErrNoRows {
			return nil, audit.ErrEntryNotFound
		}
		return nil, err
	}

	var entry audit.AuditEntry
	if err := json.Unmarshal([]byte(data), &entry); err != nil {
		return nil, err
	}
	return &entry, nil
}

// Query searches audit entries based on the given parameters.
func (s *SQLStore) Query(q audit.Query) (*audit.QueryResult, error) {
	var conditions []string
	var args []any

	if !q.TimeRange.Start.IsZero() {
		conditions = append(conditions, "timestamp >= ?")
		args = append(args, q.TimeRange.Start.Format("2006-01-02T15:04:05.999999999Z07:00"))
	}
	if !q.TimeRange.End.IsZero() {
		conditions = append(conditions, "timestamp <= ?")
		args = append(args, q.TimeRange.End.Format("2006-01-02T15:04:05.999999999Z07:00"))
	}
	if len(q.EventTypes) > 0 {
		placeholders := make([]string, len(q.EventTypes))
		for i, et := range q.EventTypes {
			placeholders[i] = "?"
			args = append(args, string(et))
		}
		conditions = append(conditions, "event_type IN ("+strings.Join(placeholders, ",")+")")
	}
	if len(q.ActorIDs) > 0 {
		placeholders := make([]string, len(q.ActorIDs))
		for i, id := range q.ActorIDs {
			placeholders[i] = "?"
			args = append(args, id)
		}
		conditions = append(conditions, "actor_id IN ("+strings.Join(placeholders, ",")+")")
	}
	if len(q.Actions) > 0 {
		placeholders := make([]string, len(q.Actions))
		for i, a := range q.Actions {
			placeholders[i] = "?"
			args = append(args, a)
		}
		conditions = append(conditions, "action IN ("+strings.Join(placeholders, ",")+")")
	}

	where := ""
	if len(conditions) > 0 {
		where = "WHERE " + strings.Join(conditions, " AND ")
	}

	// Count total
	countQuery := `SELECT COUNT(*) FROM ` + s.table + ` ` + where
	var total int64
	if err := s.db.QueryRow(countQuery, args...).Scan(&total); err != nil {
		return nil, err
	}

	// Fetch entries
	order := "ASC"
	if q.Descending {
		order = "DESC"
	}
	orderBy := "timestamp"
	if q.OrderBy != "" {
		// Only allow known column names for ORDER BY to prevent injection
		switch q.OrderBy {
		case "timestamp", "event_type", "action", "outcome", "actor_id", "sequence":
			orderBy = q.OrderBy
		}
	}

	selectQuery := fmt.Sprintf(`SELECT data FROM %s %s ORDER BY %s %s LIMIT ? OFFSET ?`,
		s.table, where, orderBy, order)

	args = append(args, q.Limit, q.Offset)

	rows, err := s.db.Query(selectQuery, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var entries []audit.AuditEntry
	for rows.Next() {
		var data string
		if err := rows.Scan(&data); err != nil {
			return nil, err
		}
		var entry audit.AuditEntry
		if err := json.Unmarshal([]byte(data), &entry); err != nil {
			return nil, err
		}
		entries = append(entries, entry)
	}

	return &audit.QueryResult{
		Entries:    entries,
		Total:      total,
		HasMore:    int64(q.Offset+len(entries)) < total,
		NextOffset: q.Offset + len(entries),
	}, rows.Err()
}

// Close closes the database connection.
func (s *SQLStore) Close() error {
	return s.db.Close()
}

// Ensure SQLStore implements the Store interface.
var _ audit.Store = (*SQLStore)(nil)
