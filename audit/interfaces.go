package audit

// Sink represents an audit log destination
type Sink interface {
	Write(entry *AuditEntry) error
	Flush() error
	Close() error
}

// Store represents a queryable audit log storage
type Store interface {
	Store(entry *AuditEntry) error
	Query(q Query) (*QueryResult, error)
	Get(id string) (*AuditEntry, error)
	Close() error
}

// Query defines parameters for searching audit logs
type Query struct {
	TimeRange   TimeRange
	EventTypes  []AuditEventType
	ActorIDs    []string
	ActorTypes  []string
	ResourceIDs []string
	Actions     []string
	Outcomes    []AuditOutcome
	TraceID     string
	Limit       int
	Offset      int
	OrderBy     string
	Descending  bool
}

// QueryResult holds the results of an audit query
type QueryResult struct {
	Entries    []AuditEntry
	Total      int64
	HasMore    bool
	NextOffset int
}

// NewQuery creates a new Query with defaults
func NewQuery() Query {
	return Query{
		Limit:      100,
		Offset:     0,
		OrderBy:    "timestamp",
		Descending: true,
	}
}

// WithTimeRange sets the time range for the query
func (q Query) WithTimeRange(tr TimeRange) Query {
	q.TimeRange = tr
	return q
}

// WithEventTypes filters by event types
func (q Query) WithEventTypes(types ...AuditEventType) Query {
	q.EventTypes = types
	return q
}

// WithActorIDs filters by actor IDs
func (q Query) WithActorIDs(ids ...string) Query {
	q.ActorIDs = ids
	return q
}

// WithActions filters by actions
func (q Query) WithActions(actions ...string) Query {
	q.Actions = actions
	return q
}

// WithOutcomes filters by outcomes
func (q Query) WithOutcomes(outcomes ...AuditOutcome) Query {
	q.Outcomes = outcomes
	return q
}

// WithTraceID filters by trace ID
func (q Query) WithTraceID(traceID string) Query {
	q.TraceID = traceID
	return q
}

// WithLimit sets the result limit
func (q Query) WithLimit(limit int) Query {
	q.Limit = limit
	return q
}

// WithOffset sets the result offset
func (q Query) WithOffset(offset int) Query {
	q.Offset = offset
	return q
}
