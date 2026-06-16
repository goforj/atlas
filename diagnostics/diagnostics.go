package diagnostics

import (
	"context"
	"errors"
	"regexp"
	"strings"
)

// Provider supplies read-only runtime diagnostics to Atlas.
type Provider interface {
	// DatabaseConnections returns safe connection metadata without secrets.
	DatabaseConnections(context.Context) ([]DatabaseConnection, error)
	// DatabaseSchema returns table, column, and index metadata for a connection.
	DatabaseSchema(context.Context, string) (DatabaseSchema, error)
	// DatabaseQuery runs a bounded read-only query.
	DatabaseQuery(context.Context, QueryRequest) (QueryResult, error)
	// LogEntries returns recent framework log entries.
	LogEntries(context.Context, LogRequest) ([]LogEntry, error)
	// LastError returns the latest error-level log entry when one is available.
	LastError(context.Context) (LogEntry, bool, error)
	// AbsoluteURL resolves an app-relative path to a local absolute URL.
	AbsoluteURL(context.Context, URLRequest) (string, error)
	// BrowserLogs returns recent browser console entries captured during local development.
	BrowserLogs(context.Context, BrowserLogRequest) ([]BrowserLogEntry, error)
	// MetricsMetadata returns app/runtime-aware metrics labels and targets.
	MetricsMetadata(context.Context, MetricsMetadataRequest) (MetricsMetadata, error)
}

// DatabaseConnection describes safe database connection metadata.
type DatabaseConnection struct {
	Name     string `json:"name"`
	Driver   string `json:"driver"`
	Database string `json:"database,omitempty"`
	App      string `json:"app,omitempty"`
}

// DatabaseSchema describes safe schema metadata.
type DatabaseSchema struct {
	Connection string  `json:"connection"`
	Tables     []Table `json:"tables,omitempty"`
}

// Table describes one database table.
type Table struct {
	Name    string   `json:"name"`
	Columns []Column `json:"columns,omitempty"`
	Indexes []Index  `json:"indexes,omitempty"`
}

// Column describes one database column.
type Column struct {
	Name     string `json:"name"`
	Type     string `json:"type"`
	Nullable bool   `json:"nullable"`
	Primary  bool   `json:"primary,omitempty"`
}

// Index describes one database index.
type Index struct {
	Name    string   `json:"name"`
	Columns []string `json:"columns"`
	Unique  bool     `json:"unique,omitempty"`
}

// QueryRequest describes a bounded read-only database query.
type QueryRequest struct {
	Connection string `json:"connection"`
	SQL        string `json:"sql"`
	Limit      int    `json:"limit,omitempty"`
}

// QueryResult describes read-only query output.
type QueryResult struct {
	Connection string           `json:"connection"`
	Columns    []string         `json:"columns,omitempty"`
	Rows       []map[string]any `json:"rows,omitempty"`
	Limit      int              `json:"limit,omitempty"`
}

// LogRequest describes a bounded log read.
type LogRequest struct {
	App   string `json:"app,omitempty"`
	Limit int    `json:"limit,omitempty"`
}

// LogEntry describes one runtime log entry.
type LogEntry struct {
	Time    string         `json:"time,omitempty"`
	App     string         `json:"app,omitempty"`
	Runtime string         `json:"runtime,omitempty"`
	Level   string         `json:"level,omitempty"`
	Message string         `json:"message"`
	Fields  map[string]any `json:"fields,omitempty"`
}

// URLRequest describes a local app URL lookup.
type URLRequest struct {
	App  string `json:"app,omitempty"`
	Path string `json:"path,omitempty"`
}

// BrowserLogRequest describes a bounded browser log read.
type BrowserLogRequest struct {
	App   string `json:"app,omitempty"`
	Limit int    `json:"limit,omitempty"`
}

// BrowserLogEntry describes one browser console log entry.
type BrowserLogEntry struct {
	Time    string `json:"time,omitempty"`
	App     string `json:"app,omitempty"`
	Level   string `json:"level,omitempty"`
	Message string `json:"message"`
	URL     string `json:"url,omitempty"`
}

// MetricsMetadataRequest describes app/runtime metrics metadata lookup.
type MetricsMetadataRequest struct {
	App     string `json:"app,omitempty"`
	Runtime string `json:"runtime,omitempty"`
}

// MetricsMetadata describes known app/runtime metric labels and targets.
type MetricsMetadata struct {
	App      string            `json:"app,omitempty"`
	Runtime  string            `json:"runtime,omitempty"`
	Labels   map[string]string `json:"labels,omitempty"`
	Targets  []string          `json:"targets,omitempty"`
	Counters []string          `json:"counters,omitempty"`
}

// StaticProvider is an in-memory diagnostics provider for tests and adapters.
type StaticProvider struct {
	Connections []DatabaseConnection
	Schemas     map[string]DatabaseSchema
	Queries     map[string]QueryResult
	Logs        []LogEntry
	BaseURLs    map[string]string
	Browser     []BrowserLogEntry
	Metrics     map[string]MetricsMetadata
}

// DatabaseConnections returns static connection metadata.
func (p StaticProvider) DatabaseConnections(context.Context) ([]DatabaseConnection, error) {
	return append([]DatabaseConnection(nil), p.Connections...), nil
}

// DatabaseSchema returns static schema metadata.
func (p StaticProvider) DatabaseSchema(_ context.Context, connection string) (DatabaseSchema, error) {
	if p.Schemas == nil {
		return DatabaseSchema{Connection: connection}, nil
	}
	schema := p.Schemas[connection]
	if schema.Connection == "" {
		schema.Connection = connection
	}
	return schema, nil
}

// DatabaseQuery returns a static query result after enforcing read-only SQL.
func (p StaticProvider) DatabaseQuery(_ context.Context, req QueryRequest) (QueryResult, error) {
	if err := ValidateReadOnlySQL(req.SQL); err != nil {
		return QueryResult{}, err
	}
	if p.Queries == nil {
		return QueryResult{Connection: req.Connection, Limit: req.Limit}, nil
	}
	result := p.Queries[req.SQL]
	if result.Connection == "" {
		result.Connection = req.Connection
	}
	if result.Limit == 0 {
		result.Limit = req.Limit
	}
	return result, nil
}

// LogEntries returns bounded static log entries.
func (p StaticProvider) LogEntries(_ context.Context, req LogRequest) ([]LogEntry, error) {
	entries := append([]LogEntry(nil), p.Logs...)
	if req.App != "" {
		filtered := entries[:0]
		for _, entry := range entries {
			if entry.App == req.App {
				filtered = append(filtered, entry)
			}
		}
		entries = filtered
	}
	if req.Limit > 0 && len(entries) > req.Limit {
		entries = entries[len(entries)-req.Limit:]
	}
	return entries, nil
}

// LastError returns the latest error-level log entry.
func (p StaticProvider) LastError(context.Context) (LogEntry, bool, error) {
	for i := len(p.Logs) - 1; i >= 0; i-- {
		if strings.EqualFold(p.Logs[i].Level, "error") {
			return p.Logs[i], true, nil
		}
	}
	return LogEntry{}, false, nil
}

// AbsoluteURL resolves a local app URL.
func (p StaticProvider) AbsoluteURL(_ context.Context, req URLRequest) (string, error) {
	app := req.App
	if app == "" {
		app = "app"
	}
	base := p.BaseURLs[app]
	if base == "" {
		return "", errors.New("base URL not found")
	}
	path := req.Path
	if path == "" {
		return base, nil
	}
	return strings.TrimRight(base, "/") + "/" + strings.TrimLeft(path, "/"), nil
}

// BrowserLogs returns bounded static browser log entries.
func (p StaticProvider) BrowserLogs(_ context.Context, req BrowserLogRequest) ([]BrowserLogEntry, error) {
	entries := append([]BrowserLogEntry(nil), p.Browser...)
	if req.App != "" {
		filtered := entries[:0]
		for _, entry := range entries {
			if entry.App == req.App {
				filtered = append(filtered, entry)
			}
		}
		entries = filtered
	}
	if req.Limit > 0 && len(entries) > req.Limit {
		entries = entries[len(entries)-req.Limit:]
	}
	return entries, nil
}

// MetricsMetadata returns static metrics metadata.
func (p StaticProvider) MetricsMetadata(_ context.Context, req MetricsMetadataRequest) (MetricsMetadata, error) {
	key := req.App + "/" + req.Runtime
	if p.Metrics != nil {
		if metadata, ok := p.Metrics[key]; ok {
			return metadata, nil
		}
		if metadata, ok := p.Metrics[req.App]; ok {
			return metadata, nil
		}
	}
	return MetricsMetadata{
		App:     req.App,
		Runtime: req.Runtime,
		Labels: map[string]string{
			"app":     req.App,
			"runtime": req.Runtime,
		},
	}, nil
}

// mutationPattern rejects obvious database mutations before an adapter sees the query.
var mutationPattern = regexp.MustCompile(`(?i)\b(insert|update|delete|drop|alter|truncate|create|replace|merge|grant|revoke|vacuum|call|execute)\b`)

// ValidateReadOnlySQL rejects SQL that is not obviously read-only.
func ValidateReadOnlySQL(sql string) error {
	trimmed := strings.TrimSpace(sql)
	if trimmed == "" {
		return errors.New("query is required")
	}
	lower := strings.ToLower(trimmed)
	if mutationPattern.MatchString(lower) {
		return errors.New("query must be read-only")
	}
	if strings.HasPrefix(lower, "select ") ||
		strings.HasPrefix(lower, "show ") ||
		strings.HasPrefix(lower, "describe ") ||
		strings.HasPrefix(lower, "explain ") ||
		strings.HasPrefix(lower, "with ") {
		return nil
	}
	return errors.New("query must start with SELECT, WITH, SHOW, DESCRIBE, or EXPLAIN")
}
