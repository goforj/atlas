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
		mcpgo.WithNumber("limit", mcpgo.Description("Maximum number of results.")),
		mcpgo.WithNumber("token_limit", mcpgo.Description("Maximum words per snippet.")),
	)
}

// toolReadDocSection defines the bounded section reader tool.
func toolReadDocSection() mcpgo.Tool {
	return baseTool("read-doc-section", "Reads one bounded Markdown section.",
		mcpgo.WithString("path", mcpgo.Description("Document path."), mcpgo.Required()),
		mcpgo.WithString("heading", mcpgo.Description("Heading to read.")),
		mcpgo.WithNumber("token_limit", mcpgo.Description("Maximum words to return.")),
	)
}

// toolReadDocNeighborhood defines the nearby-section reader tool.
func toolReadDocNeighborhood() mcpgo.Tool {
	return baseTool("read-doc-neighborhood", "Reads one Markdown section plus nearby sections.",
		mcpgo.WithString("path", mcpgo.Description("Document path."), mcpgo.Required()),
		mcpgo.WithString("heading", mcpgo.Description("Heading to read."), mcpgo.Required()),
		mcpgo.WithNumber("before", mcpgo.Description("Sections before.")),
		mcpgo.WithNumber("after", mcpgo.Description("Sections after.")),
		mcpgo.WithNumber("token_limit", mcpgo.Description("Maximum words per section.")),
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
		mcpgo.WithNumber("limit", mcpgo.Description("Maximum rows to return.")),
	)
}

// toolReadLogEntries defines the recent log reader tool.
func toolReadLogEntries() mcpgo.Tool {
	return baseTool("read-log-entries", "Returns recent framework log entries.",
		appArg(),
		mcpgo.WithNumber("limit", mcpgo.Description("Maximum entries to return.")),
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
		mcpgo.WithNumber("limit", mcpgo.Description("Maximum entries to return.")),
	)
}

// toolMetricsMetadata defines the app/runtime metrics metadata tool.
func toolMetricsMetadata() mcpgo.Tool {
	return baseTool("metrics-metadata", "Returns app/runtime-aware metrics labels and targets.",
		appArg(),
		mcpgo.WithString("runtime", mcpgo.Description("Runtime name.")),
	)
}

// toolRuntimeSnapshot defines the composed runtime evidence tool.
func toolRuntimeSnapshot() mcpgo.Tool {
	return baseTool("runtime-snapshot", "Combines safe local runtime evidence for an app, runtime, path, or route.",
		appArg(),
		mcpgo.WithString("runtime", mcpgo.Description("Runtime name such as http, jobs, scheduler, or cli.")),
		mcpgo.WithString("path", mcpgo.Description("App-relative path to resolve when debugging an HTTP surface.")),
		mcpgo.WithString("route_name", mcpgo.Description("Route or route group name being investigated.")),
		mcpgo.WithString("time_window", mcpgo.Description("Human time window hint for log interpretation.")),
		mcpgo.WithNumber("limit", mcpgo.Description("Maximum log and browser entries to include.")),
	)
}

// toolDebugPlan defines the runtime evidence planning tool.
func toolDebugPlan() mcpgo.Tool {
	return baseTool("debug-plan", "Builds read-only runtime debugging steps from the available Atlas evidence.",
		appArg(),
		mcpgo.WithString("runtime", mcpgo.Description("Runtime name such as http, jobs, scheduler, or cli.")),
		mcpgo.WithString("path", mcpgo.Description("App-relative path to resolve when debugging an HTTP surface.")),
		mcpgo.WithString("route_name", mcpgo.Description("Route or route group name being investigated.")),
		mcpgo.WithString("time_window", mcpgo.Description("Human time window hint for log interpretation.")),
		mcpgo.WithNumber("limit", mcpgo.Description("Maximum log and browser entries to include.")),
	)
}

// toolWorkflowPlan defines the framework workflow planning tool.
func toolWorkflowPlan() mcpgo.Tool {
	return baseTool("workflow-plan", "Returns a deterministic GoForj workflow plan for a development task.",
		appArg(),
		mcpgo.WithString("task", mcpgo.Description("Task the agent is planning."), mcpgo.Required()),
	)
}

// toolRegistrationPoints defines the app registration surface tool.
func toolRegistrationPoints() mcpgo.Tool {
	return baseTool("registration-points", "Returns app-owned registration and Wire surfaces for the selected app.", appArg())
}

// toolValidationPlan defines the task-aware validation tool.
func toolValidationPlan() mcpgo.Tool {
	return baseTool("validation-plan", "Returns task-aware build, test, and inspection checks.",
		appArg(),
		mcpgo.WithString("task", mcpgo.Description("Task or workflow to validate."), mcpgo.Required()),
	)
}

// toolWireDiagnostics defines the Wire error classifier tool.
func toolWireDiagnostics() mcpgo.Tool {
	return baseTool("wire-diagnostics", "Classifies common GoForj Wire failures and suggests provider-set fixes.",
		mcpgo.WithString("output", mcpgo.Description("Wire or build error output."), mcpgo.Required()),
	)
}

// toolScenarioGuide defines the verified scenario reference tool.
func toolScenarioGuide() mcpgo.Tool {
	return baseTool("scenario-guide", "Returns verified GoForj scenario references for a task.",
		mcpgo.WithString("query", mcpgo.Description("Scenario or workflow query."), mcpgo.Required()),
	)
}

// toolResourceInventory defines the resource inventory tool.
func toolResourceInventory() mcpgo.Tool {
	return baseTool("resource-inventory", "Returns app, component, registration, and safe runtime resource inventory.")
}

// toolGeneratedFilePolicy defines the generated-file ownership classifier.
func toolGeneratedFilePolicy() mcpgo.Tool {
	return baseTool("generated-file-policy", "Classifies whether a path is generated, app-owned, user-owned, or should be changed through GoForj commands.",
		mcpgo.WithString("path", mcpgo.Description("Project-relative path to classify."), mcpgo.Required()),
	)
}

// toolCommandAdvice defines the preferred GoForj command advisor.
func toolCommandAdvice() mcpgo.Tool {
	return baseTool("command-advice", "Returns the preferred GoForj command for a task, app, and resource name.",
		appArg(),
		mcpgo.WithString("task", mcpgo.Description("Task the agent is planning."), mcpgo.Required()),
		mcpgo.WithString("resource", mcpgo.Description("Resource name to place into the command.")),
	)
}

// toolDocsSectionPack defines the workflow docs section reader.
func toolDocsSectionPack() mcpgo.Tool {
	return baseTool("docs-section-pack", "Returns bounded docs sections in workflow reading order.",
		mcpgo.WithString("workflow_id", mcpgo.Description("Workflow id such as goforj-add-http-route.")),
		mcpgo.WithString("task", mcpgo.Description("Task to classify when workflow_id is omitted.")),
		mcpgo.WithNumber("token_limit", mcpgo.Description("Maximum words per docs section.")),
	)
}

// toolVersionAlignment defines the docs and project version alignment tool.
func toolVersionAlignment() mcpgo.Tool {
	return baseTool("version-alignment", "Compares project GoForj version, Atlas version, and active docs bundle metadata.")
}

// toolWorkflowScorecard defines the deterministic workflow fixture scorecard tool.
func toolWorkflowScorecard() mcpgo.Tool {
	return baseTool("workflow-scorecard", "Runs deterministic workflow fixtures and returns scorecard output.",
		mcpgo.WithBoolean("capture_transcript", mcpgo.Description("Include compact fixture transcript entries.")),
	)
}
