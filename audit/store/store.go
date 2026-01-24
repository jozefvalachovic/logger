package store

import (
	"github.com/jozefvalachovic/logger/v4/audit"
)

// Store represents a queryable audit log storage
type Store = audit.Store

// Query defines parameters for searching audit logs
type Query = audit.Query

// QueryResult holds the results of an audit query
type QueryResult = audit.QueryResult

// NewQuery creates a new Query with defaults
var NewQuery = audit.NewQuery
