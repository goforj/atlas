package mcp

import mcpgo "github.com/mark3labs/mcp-go/mcp"

// baseTool applies Atlas's read-only safety annotations to every MCP tool.
func baseTool(name string, description string, opts ...mcpgo.ToolOption) mcpgo.Tool {
	base := []mcpgo.ToolOption{
		mcpgo.WithDescription(description),
		mcpgo.WithReadOnlyHintAnnotation(true),
		mcpgo.WithDestructiveHintAnnotation(false),
		mcpgo.WithIdempotentHintAnnotation(true),
		mcpgo.WithOpenWorldHintAnnotation(false),
	}
	base = append(base, opts...)
	return mcpgo.NewTool(name, base...)
}

// appArg defines the shared optional app selector for app-aware tools.
func appArg() mcpgo.ToolOption {
	return mcpgo.WithString("app", mcpgo.Description("GoForj app name. Defaults to the default app."))
}

// toolApplicationInfo defines the project summary tool.
func toolApplicationInfo() mcpgo.Tool {
	return baseTool("application-info", "Returns project, app, runtime, component, and docs version information.")
}

// toolProjectLayout defines the app-aware layout tool.
func toolProjectLayout() mcpgo.Tool {
	return baseTool("project-layout", "Returns app-aware GoForj file layout and registration points.", appArg())
}

// toolSearchDocs defines the docs search tool.
func toolSearchDocs() mcpgo.Tool {
	return baseTool("search-docs", "Searches version-aware GoForj documentation.",
		mcpgo.WithString("query", mcpgo.Description("Search query."), mcpgo.Required()),
		mcpgo.WithInteger("limit", mcpgo.Description("Maximum number of results.")),
		mcpgo.WithInteger("token_limit", mcpgo.Description("Maximum words per snippet.")),
	)
}

// toolReadDocSection defines the bounded section reader tool.
func toolReadDocSection() mcpgo.Tool {
	return baseTool("read-doc-section", "Reads one bounded Markdown section.",
		mcpgo.WithString("path", mcpgo.Description("Document path."), mcpgo.Required()),
		mcpgo.WithString("heading", mcpgo.Description("Heading to read.")),
		mcpgo.WithInteger("token_limit", mcpgo.Description("Maximum words to return.")),
	)
}

// toolReadDocNeighborhood defines the nearby-section reader tool.
func toolReadDocNeighborhood() mcpgo.Tool {
	return baseTool("read-doc-neighborhood", "Reads one Markdown section plus nearby sections.",
		mcpgo.WithString("path", mcpgo.Description("Document path."), mcpgo.Required()),
		mcpgo.WithString("heading", mcpgo.Description("Heading to read."), mcpgo.Required()),
		mcpgo.WithInteger("before", mcpgo.Description("Sections before.")),
		mcpgo.WithInteger("after", mcpgo.Description("Sections after.")),
		mcpgo.WithInteger("token_limit", mcpgo.Description("Maximum words per section.")),
	)
}

// toolListDocHeadings defines the cheap document outline tool.
func toolListDocHeadings() mcpgo.Tool {
	return baseTool("list-doc-headings", "Lists headings for one Markdown document.",
		mcpgo.WithString("path", mcpgo.Description("Document path."), mcpgo.Required()),
	)
}

// toolExplainAPI defines the command/path-to-docs mapping tool.
func toolExplainAPI() mcpgo.Tool {
	return baseTool("explain-api", "Maps a GoForj command, path, symbol, or concept to useful docs.",
		mcpgo.WithString("query", mcpgo.Description("Command, path, symbol, or concept."), mcpgo.Required()),
	)
}

// toolRouteList defines the app route inventory tool.
func toolRouteList() mcpgo.Tool {
	return baseTool("route-list", "Returns routes for the selected app.", appArg())
}

// toolScheduleList defines the app schedule inventory tool.
func toolScheduleList() mcpgo.Tool {
	return baseTool("schedule-list", "Returns schedules for the selected app.", appArg())
}

// toolCommandList defines the app command inventory tool.
func toolCommandList() mcpgo.Tool {
	return baseTool("command-list", "Returns commands for the selected app.", appArg())
}

// toolDatabaseConnections defines the safe database metadata tool.
func toolDatabaseConnections() mcpgo.Tool {
	return baseTool("database-connections", "Returns safe database connection metadata.")
}

// toolDatabaseSchema defines the safe schema metadata tool.
func toolDatabaseSchema() mcpgo.Tool {
	return baseTool("database-schema", "Returns table, column, and index metadata for a connection.",
		mcpgo.WithString("connection", mcpgo.Description("Database connection name."), mcpgo.Required()),
	)
}

// toolDatabaseQuery defines the bounded read-only query tool.
func toolDatabaseQuery() mcpgo.Tool {
	return baseTool("database-query", "Runs a bounded read-only database query.",
		mcpgo.WithString("connection", mcpgo.Description("Database connection name."), mcpgo.Required()),
		mcpgo.WithString("sql", mcpgo.Description("Read-only SQL query."), mcpgo.Required()),
		mcpgo.WithInteger("limit", mcpgo.Description("Maximum rows to return.")),
	)
}

// toolReadLogEntries defines the recent log reader tool.
func toolReadLogEntries() mcpgo.Tool {
	return baseTool("read-log-entries", "Returns recent framework log entries.",
		appArg(),
		mcpgo.WithInteger("limit", mcpgo.Description("Maximum entries to return.")),
	)
}

// toolLastError defines the latest application error tool.
func toolLastError() mcpgo.Tool {
	return baseTool("last-error", "Returns the most recent application error.")
}

// toolGetAbsoluteURL defines the local URL resolver tool.
func toolGetAbsoluteURL() mcpgo.Tool {
	return baseTool("get-absolute-url", "Converts an app-relative path into a local absolute URL.",
		appArg(),
		mcpgo.WithString("path", mcpgo.Description("App-relative URL path.")),
	)
}

// toolBrowserLogs defines the browser console log reader tool.
func toolBrowserLogs() mcpgo.Tool {
	return baseTool("browser-logs", "Returns recent browser console logs captured during local development.",
		appArg(),
		mcpgo.WithInteger("limit", mcpgo.Description("Maximum entries to return.")),
	)
}

// toolMetricsMetadata defines the app/runtime metrics metadata tool.
func toolMetricsMetadata() mcpgo.Tool {
	return baseTool("metrics-metadata", "Returns app/runtime-aware metrics labels and targets.",
		appArg(),
		mcpgo.WithString("runtime", mcpgo.Description("Runtime name.")),
	)
}
