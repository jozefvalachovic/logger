package audit

import (
	"encoding/json"
	"time"
)

// AuditEventType categorizes audit events for filtering and compliance
type AuditEventType string

const (
	// AuditAuth represents authentication events
	AuditAuth AuditEventType = "authentication"
	// AuditAuthz represents authorization/permission checks
	AuditAuthz AuditEventType = "authorization"
	// AuditDataAccess represents data read operations
	AuditDataAccess AuditEventType = "data_access"
	// AuditDataModify represents data create/update/delete operations
	AuditDataModify AuditEventType = "data_modification"
	// AuditConfigChange represents system configuration changes
	AuditConfigChange AuditEventType = "config_change"
	// AuditAdminAction represents administrative actions
	AuditAdminAction AuditEventType = "admin_action"
	// AuditSecurityEvent represents security-related events
	AuditSecurityEvent AuditEventType = "security_event"
	// AuditUserLifecycle represents user account lifecycle events
	AuditUserLifecycle AuditEventType = "user_lifecycle"
	// AuditAPIAccess represents API access events
	AuditAPIAccess AuditEventType = "api_access"
	// AuditSystem represents system events
	AuditSystem AuditEventType = "system"
	// AuditCustom represents custom event type
	AuditCustom AuditEventType = "custom"
)

// AuditOutcome represents the result of an audited action
type AuditOutcome string

const (
	OutcomeSuccess AuditOutcome = "success"
	OutcomeFailure AuditOutcome = "failure"
	OutcomeDenied  AuditOutcome = "denied"
	OutcomeError   AuditOutcome = "error"
	OutcomePending AuditOutcome = "pending"
	OutcomeUnknown AuditOutcome = "unknown"
)

// AuditActor represents who performed the action
type AuditActor struct {
	ID        string         `json:"id"`
	Type      string         `json:"type"`
	Name      string         `json:"name,omitempty"`
	Email     string         `json:"email,omitempty"`
	IP        string         `json:"ip,omitempty"`
	UserAgent string         `json:"user_agent,omitempty"`
	SessionID string         `json:"session_id,omitempty"`
	Metadata  map[string]any `json:"metadata,omitempty"`
}

// AuditResource represents what was affected by the action
type AuditResource struct {
	ID       string         `json:"id"`
	Type     string         `json:"type"`
	Name     string         `json:"name,omitempty"`
	Metadata map[string]any `json:"metadata,omitempty"`
}

// AuditEvent represents a single auditable action
type AuditEvent struct {
	Type        AuditEventType `json:"type"`
	Action      string         `json:"action"`
	Outcome     AuditOutcome   `json:"outcome"`
	Actor       AuditActor     `json:"actor"`
	Resource    *AuditResource `json:"resource,omitempty"`
	Description string         `json:"description,omitempty"`
	Reason      string         `json:"reason,omitempty"`
	Changes     *AuditChanges  `json:"changes,omitempty"`
	Metadata    map[string]any `json:"metadata,omitempty"`
}

// AuditChanges captures before/after state for modifications
type AuditChanges struct {
	Before map[string]any `json:"before,omitempty"`
	After  map[string]any `json:"after,omitempty"`
}

// AuditEntry is the complete audit log record with all metadata
type AuditEntry struct {
	ID            string       `json:"id"`
	Timestamp     time.Time    `json:"timestamp"`
	Event         AuditEvent   `json:"event"`
	Service       *ServiceInfo `json:"service,omitempty"`
	Trace         *TraceInfo   `json:"trace,omitempty"`
	Hash          string       `json:"hash,omitempty"`
	PreviousHash  string       `json:"previous_hash,omitempty"`
	Sequence      int64        `json:"sequence"`
	SchemaVersion string       `json:"schema_version"`
}

// ServiceInfo contains information about the originating service
type ServiceInfo struct {
	Name        string `json:"name"`
	Version     string `json:"version,omitempty"`
	Environment string `json:"environment,omitempty"`
	Region      string `json:"region,omitempty"`
	Instance    string `json:"instance,omitempty"`
	Namespace   string `json:"namespace,omitempty"`
}

// TraceInfo contains distributed tracing context
type TraceInfo struct {
	TraceID      string `json:"trace_id"`
	SpanID       string `json:"span_id,omitempty"`
	ParentSpanID string `json:"parent_span_id,omitempty"`
}

// CurrentSchemaVersion is the current audit entry schema version
const CurrentSchemaVersion = "1.0"

// MarshalJSON implements json.Marshaler for AuditEntry
func (e AuditEntry) MarshalJSON() ([]byte, error) {
	type Alias AuditEntry
	return json.Marshal(&struct {
		Alias
	}{
		Alias: (Alias)(e),
	})
}

// Validate checks if an AuditEvent has required fields
func (e *AuditEvent) Validate() error {
	if e.Type == "" {
		return ErrMissingEventType
	}
	if e.Action == "" {
		return ErrMissingAction
	}
	if e.Outcome == "" {
		e.Outcome = OutcomeUnknown
	}
	if e.Actor.ID == "" && e.Actor.Type == "" {
		return ErrMissingActor
	}
	return nil
}

// TimeRange represents a time range for queries
type TimeRange struct {
	Start time.Time
	End   time.Time
}

// NewTimeRange creates a TimeRange from start and end times
func NewTimeRange(start, end time.Time) TimeRange {
	return TimeRange{Start: start, End: end}
}

// LastDuration creates a TimeRange for the last N duration
func LastDuration(d time.Duration) TimeRange {
	now := time.Now()
	return TimeRange{
		Start: now.Add(-d),
		End:   now,
	}
}

// LastDays creates a TimeRange for the last N days
func LastDays(days int) TimeRange {
	return LastDuration(time.Duration(days) * 24 * time.Hour)
}
