package mcp

import (
	"context"

	"github.com/goforj/atlas/diagnostics"
	atlasdocs "github.com/goforj/atlas/docs"
	"github.com/goforj/atlas/project"
	"github.com/goforj/atlas/skills"
	"github.com/goforj/atlas/workflows"
	mcpgo "github.com/mark3labs/mcp-go/mcp"
	"github.com/mark3labs/mcp-go/server"
)

// Inventory contains app-aware read-only project facts exposed by Atlas.
type Inventory struct {
	Routes     map[string][]string
	Schedules  map[string][]string
	Commands   map[string][]string
	Queues     []string
	Caches     []string
	Disks      []string
	EventBuses []string
	Resources  []workflows.ResourceLink
}

// Server exposes Atlas MCP tools for one GoForj project.
type Server struct {
	Project     project.Project
	Docs        atlasdocs.Provider
	Diagnostics diagnostics.Provider
	Inventory   Inventory
	Version     string
}

// New creates a mark3labs MCP server with Atlas tools registered.
func New(cfg Server) *server.MCPServer {
	cfg = cfg.withDefaults()
	s := server.NewMCPServer(
		"goforj-atlas",
		cfg.Version,
		server.WithToolCapabilities(false),
		server.WithRecovery(),
	)
	cfg.register(s)
	return s
}

// ServeStdio starts the Atlas MCP server over stdio.
func ServeStdio(cfg Server) error {
	cfg = cfg.withDefaults()
	if err := cfg.warmDocs(context.Background()); err != nil {
		return err
	}
	return server.ServeStdio(New(cfg))
}

// withDefaults keeps constructor and stdio startup behavior aligned.
func (s Server) withDefaults() Server {
	if s.Version == "" {
		s.Version = "dev"
	}
	if s.Docs == nil {
		s.Docs = atlasdocs.DefaultProvider(s.Version)
	}
	return s
}

// warmDocs pays docs loading cost before MCP begins serving requests.
func (s Server) warmDocs(ctx context.Context) error {
	_, err := s.docsProvider().Documents(ctx)
	return err
}

// register keeps tool registration explicit so MCP capabilities remain auditable.
func (s Server) register(mcpServer *server.MCPServer) {
	tools := []struct {
		tool    mcpgo.Tool
		handler server.ToolHandlerFunc
	}{
		{toolApplicationInfo(), s.applicationInfo},
		{toolProjectLayout(), s.projectLayout},
		{toolSearchDocs(), s.searchDocs},
		{toolReadDocSection(), s.readDocSection},
		{toolReadDocNeighborhood(), s.readDocNeighborhood},
		{toolListDocHeadings(), s.listDocHeadings},
		{toolExplainAPI(), s.explainAPI},
		{toolRouteList(), s.routeList},
		{toolScheduleList(), s.scheduleList},
		{toolCommandList(), s.commandList},
		{toolDatabaseConnections(), s.databaseConnections},
		{toolDatabaseSchema(), s.databaseSchema},
		{toolDatabaseQuery(), s.databaseQuery},
		{toolReadLogEntries(), s.readLogEntries},
		{toolLastError(), s.lastError},
		{toolGetAbsoluteURL(), s.getAbsoluteURL},
		{toolBrowserLogs(), s.browserLogs},
		{toolMetricsMetadata(), s.metricsMetadata},
		{toolRuntimeSnapshot(), s.runtimeSnapshot},
		{toolDebugPlan(), s.debugPlan},
		{toolWorkflowPlan(), s.workflowPlan},
		{toolRegistrationPoints(), s.registrationPoints},
		{toolValidationPlan(), s.validationPlan},
		{toolWireDiagnostics(), s.wireDiagnostics},
		{toolScenarioGuide(), s.scenarioGuide},
		{toolResourceInventory(), s.resourceInventory},
		{toolGeneratedFilePolicy(), s.generatedFilePolicy},
		{toolCommandAdvice(), s.commandAdvice},
		{toolDocsSectionPack(), s.docsSectionPack},
		{toolVersionAlignment(), s.versionAlignment},
		{toolWorkflowScorecard(), s.workflowScorecard},
	}

	for _, tool := range tools {
		mcpServer.AddTool(tool.tool, tool.handler)
	}
}

// jsonResult returns both structured content and a text fallback for broad client compatibility.
func jsonResult[T any](value T) (*mcpgo.CallToolResult, error) {
	return mcpgo.NewToolResultJSON(value)
}

// toolError reports user-correctable tool errors without failing the MCP transport.
func toolError(err error) (*mcpgo.CallToolResult, error) {
	return mcpgo.NewToolResultError(err.Error()), nil
}

// appName centralizes optional app argument handling for app-aware tools.
func appName(request mcpgo.CallToolRequest) string {
	return request.GetString("app", "")
}

// docsProvider returns the configured docs provider or Atlas' local cached docs.
func (s Server) docsProvider() atlasdocs.Provider {
	if s.Docs != nil {
		return s.Docs
	}
	return atlasdocs.StaticProvider{}
}

// diagnosticsProvider returns an empty provider so read-only tools are safe by default.
func (s Server) diagnosticsProvider() diagnostics.Provider {
	if s.Diagnostics != nil {
		return s.Diagnostics
	}
	return diagnostics.StaticProvider{}
}

// projectWithDefaults keeps tool handlers from repeating default app synthesis.
func (s Server) projectWithDefaults() project.Project {
	return s.Project.WithDiscoveredDefaults()
}

// workflowContext adapts MCP server state into deterministic workflow helpers.
func (s Server) workflowContext(ctx context.Context) workflows.Context {
	connections, _ := s.diagnosticsProvider().DatabaseConnections(_context(ctx))
	overlays := []string{}
	if s.Project.Root != "" {
		projectSkills, _ := skills.ProjectSkills(s.Project.Root)
		for _, projectSkill := range projectSkills {
			overlays = append(overlays, projectSkill.Name)
		}
	}
	return workflows.Context{
		Project: s.projectWithDefaults(),
		Inventory: workflows.Inventory{
			Routes:     s.Inventory.Routes,
			Schedules:  s.Inventory.Schedules,
			Commands:   s.Inventory.Commands,
			Queues:     s.Inventory.Queues,
			Caches:     s.Inventory.Caches,
			Disks:      s.Inventory.Disks,
			EventBuses: s.Inventory.EventBuses,
			Resources:  s.Inventory.Resources,
		},
		Connections: connections,
		Overlays:    overlays,
	}
}

// _context normalizes nil contexts for direct handler tests and non-transport callers.
func _context(ctx context.Context) context.Context {
	if ctx == nil {
		return context.Background()
	}
	return ctx
}
