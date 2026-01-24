package store

import (
	"encoding/csv"
	"encoding/json"
	"fmt"
	"io"
	"time"

	"github.com/jozefvalachovic/logger/v4/audit"
)

// ExportFormat represents the export format
type ExportFormat string

const (
	FormatJSON  ExportFormat = "json"
	FormatJSONL ExportFormat = "jsonl"
	FormatCSV   ExportFormat = "csv"
)

// Export exports audit entries to a writer in the specified format
func Export(w io.Writer, entries []audit.AuditEntry, format ExportFormat) error {
	switch format {
	case FormatJSON:
		return exportJSON(w, entries)
	case FormatJSONL:
		return exportJSONL(w, entries)
	case FormatCSV:
		return exportCSV(w, entries)
	default:
		return fmt.Errorf("store: unsupported export format: %s", format)
	}
}

// ExportQuery exports query results directly
func ExportQuery(w io.Writer, store audit.Store, q audit.Query, format ExportFormat) error {
	var allEntries []audit.AuditEntry
	offset := 0

	for {
		q.Offset = offset
		result, err := store.Query(q)
		if err != nil {
			return err
		}

		allEntries = append(allEntries, result.Entries...)

		if !result.HasMore {
			break
		}
		offset = result.NextOffset
	}

	return Export(w, allEntries, format)
}

func exportJSON(w io.Writer, entries []audit.AuditEntry) error {
	encoder := json.NewEncoder(w)
	encoder.SetIndent("", "  ")
	return encoder.Encode(entries)
}

func exportJSONL(w io.Writer, entries []audit.AuditEntry) error {
	for _, entry := range entries {
		data, err := json.Marshal(entry)
		if err != nil {
			return err
		}
		if _, err := w.Write(data); err != nil {
			return err
		}
		if _, err := w.Write([]byte("\n")); err != nil {
			return err
		}
	}
	return nil
}

func exportCSV(w io.Writer, entries []audit.AuditEntry) error {
	cw := csv.NewWriter(w)
	defer cw.Flush()

	headers := []string{
		"id", "timestamp", "type", "action", "outcome",
		"actor_id", "actor_type", "actor_name", "actor_ip",
		"resource_id", "resource_type", "resource_name",
		"description", "trace_id", "sequence", "hash",
	}

	if err := cw.Write(headers); err != nil {
		return err
	}

	for _, entry := range entries {
		resourceID := ""
		resourceType := ""
		resourceName := ""
		if entry.Event.Resource != nil {
			resourceID = entry.Event.Resource.ID
			resourceType = entry.Event.Resource.Type
			resourceName = entry.Event.Resource.Name
		}

		traceID := ""
		if entry.Trace != nil {
			traceID = entry.Trace.TraceID
		}

		row := []string{
			entry.ID,
			entry.Timestamp.Format(time.RFC3339Nano),
			string(entry.Event.Type),
			entry.Event.Action,
			string(entry.Event.Outcome),
			entry.Event.Actor.ID,
			entry.Event.Actor.Type,
			entry.Event.Actor.Name,
			entry.Event.Actor.IP,
			resourceID,
			resourceType,
			resourceName,
			entry.Event.Description,
			traceID,
			fmt.Sprintf("%d", entry.Sequence),
			entry.Hash,
		}

		if err := cw.Write(row); err != nil {
			return err
		}
	}

	return nil
}

// AggregateBy aggregates entries by a field
type AggregateResult struct {
	Field string
	Value string
	Count int64
	First time.Time
	Last  time.Time
}

// AggregateBy aggregates entries by a field
func AggregateBy(entries []audit.AuditEntry, field string) []AggregateResult {
	counts := make(map[string]*AggregateResult)

	for _, entry := range entries {
		var value string
		switch field {
		case "type":
			value = string(entry.Event.Type)
		case "action":
			value = entry.Event.Action
		case "outcome":
			value = string(entry.Event.Outcome)
		case "actor_id":
			value = entry.Event.Actor.ID
		case "actor_type":
			value = entry.Event.Actor.Type
		case "resource_type":
			if entry.Event.Resource != nil {
				value = entry.Event.Resource.Type
			}
		default:
			continue
		}

		if value == "" {
			value = "(empty)"
		}

		if agg, ok := counts[value]; ok {
			agg.Count++
			if entry.Timestamp.Before(agg.First) {
				agg.First = entry.Timestamp
			}
			if entry.Timestamp.After(agg.Last) {
				agg.Last = entry.Timestamp
			}
		} else {
			counts[value] = &AggregateResult{
				Field: field,
				Value: value,
				Count: 1,
				First: entry.Timestamp,
				Last:  entry.Timestamp,
			}
		}
	}

	results := make([]AggregateResult, 0, len(counts))
	for _, agg := range counts {
		results = append(results, *agg)
	}

	return results
}
