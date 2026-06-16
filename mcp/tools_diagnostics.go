package mcp

import (
	"context"

	"github.com/goforj/atlas/diagnostics"
	mcpgo "github.com/mark3labs/mcp-go/mcp"
)

// databaseConnections returns safe connection metadata without secrets.
func (s Server) databaseConnections(ctx context.Context, _ mcpgo.CallToolRequest) (*mcpgo.CallToolResult, error) {
	connections, err := s.diagnosticsProvider().DatabaseConnections(_context(ctx))
	if err != nil {
		return nil, err
	}
	return jsonResult(connections)
}

// databaseSchema returns safe schema metadata for one connection.
func (s Server) databaseSchema(ctx context.Context, request mcpgo.CallToolRequest) (*mcpgo.CallToolResult, error) {
	connection, err := request.RequireString("connection")
	if err != nil {
		return toolError(err)
	}
	schema, err := s.diagnosticsProvider().DatabaseSchema(_context(ctx), connection)
	if err != nil {
		return nil, err
	}
	return jsonResult(schema)
}

// databaseQuery delegates to a provider that must enforce read-only query policy.
func (s Server) databaseQuery(ctx context.Context, request mcpgo.CallToolRequest) (*mcpgo.CallToolResult, error) {
	connection, err := request.RequireString("connection")
	if err != nil {
		return toolError(err)
	}
	sql, err := request.RequireString("sql")
	if err != nil {
		return toolError(err)
	}
	result, err := s.diagnosticsProvider().DatabaseQuery(_context(ctx), diagnostics.QueryRequest{
		Connection: connection,
		SQL:        sql,
		Limit:      request.GetInt("limit", 100),
	})
	if err != nil {
		return toolError(err)
	}
	return jsonResult(result)
}

// readLogEntries returns bounded runtime log entries.
func (s Server) readLogEntries(ctx context.Context, request mcpgo.CallToolRequest) (*mcpgo.CallToolResult, error) {
	entries, err := s.diagnosticsProvider().LogEntries(_context(ctx), diagnostics.LogRequest{
		App:   appName(request),
		Limit: request.GetInt("limit", 100),
	})
	if err != nil {
		return nil, err
	}
	return jsonResult(entries)
}

// lastError returns the latest error while preserving a "not found" success shape.
func (s Server) lastError(ctx context.Context, _ mcpgo.CallToolRequest) (*mcpgo.CallToolResult, error) {
	entry, ok, err := s.diagnosticsProvider().LastError(_context(ctx))
	if err != nil {
		return nil, err
	}
	return jsonResult(map[string]any{
		"found": ok,
		"entry": entry,
	})
}

// getAbsoluteURL resolves local URLs so agents do not guess ports.
func (s Server) getAbsoluteURL(ctx context.Context, request mcpgo.CallToolRequest) (*mcpgo.CallToolResult, error) {
	url, err := s.diagnosticsProvider().AbsoluteURL(_context(ctx), diagnostics.URLRequest{
		App:  appName(request),
		Path: request.GetString("path", ""),
	})
	if err != nil {
		return toolError(err)
	}
	return jsonResult(map[string]string{"url": url})
}

// browserLogs returns captured browser logs without requiring Lighthouse UI.
func (s Server) browserLogs(ctx context.Context, request mcpgo.CallToolRequest) (*mcpgo.CallToolResult, error) {
	entries, err := s.diagnosticsProvider().BrowserLogs(_context(ctx), diagnostics.BrowserLogRequest{
		App:   appName(request),
		Limit: request.GetInt("limit", 100),
	})
	if err != nil {
		return nil, err
	}
	return jsonResult(entries)
}

// metricsMetadata returns app/runtime labels and metric hints for observability work.
func (s Server) metricsMetadata(ctx context.Context, request mcpgo.CallToolRequest) (*mcpgo.CallToolResult, error) {
	metadata, err := s.diagnosticsProvider().MetricsMetadata(_context(ctx), diagnostics.MetricsMetadataRequest{
		App:     appName(request),
		Runtime: request.GetString("runtime", ""),
	})
	if err != nil {
		return nil, err
	}
	return jsonResult(metadata)
}
