package mcp

import (
	"context"
	"encoding/json"
	"strings"
	"testing"

	"github.com/goforj/atlas/diagnostics"
	atlasdocs "github.com/goforj/atlas/docs"
	"github.com/goforj/atlas/project"
	mcpgo "github.com/mark3labs/mcp-go/mcp"
)

func TestApplicationInfo(t *testing.T) {
	result, err := fixtureServer().applicationInfo(context.Background(), request(nil))
	if err != nil {
		t.Fatalf("application info failed: %v", err)
	}
	text := resultText(t, result)
	if !strings.Contains(text, `"project":"demo"`) || !strings.Contains(text, `"docs_revision":"rev1"`) {
		t.Fatalf("unexpected result %s", text)
	}
}

func TestProjectLayoutNamedApp(t *testing.T) {
	result, err := fixtureServer().projectLayout(context.Background(), request(map[string]any{"app": "marketplace"}))
	if err != nil {
		t.Fatalf("project layout failed: %v", err)
	}
	text := resultText(t, result)
	if !strings.Contains(text, `"entrypoint":"cmd/marketplace/main.go"`) {
		t.Fatalf("unexpected result %s", text)
	}
}

func TestSearchDocsTool(t *testing.T) {
	result, err := fixtureServer().searchDocs(context.Background(), request(map[string]any{"query": "make controller marketplace"}))
	if err != nil {
		t.Fatalf("search docs failed: %v", err)
	}
	text := resultText(t, result)
	if !strings.Contains(text, "Make Commands") {
		t.Fatalf("unexpected result %s", text)
	}
}

func TestRouteListUsesDefaultApp(t *testing.T) {
	result, err := fixtureServer().routeList(context.Background(), request(nil))
	if err != nil {
		t.Fatalf("route list failed: %v", err)
	}
	text := resultText(t, result)
	if !strings.Contains(text, "GET /health") {
		t.Fatalf("unexpected result %s", text)
	}
}

func TestUnknownAppReturnsToolError(t *testing.T) {
	result, err := fixtureServer().routeList(context.Background(), request(map[string]any{"app": "missing"}))
	if err != nil {
		t.Fatalf("route list failed: %v", err)
	}
	if !result.IsError {
		t.Fatalf("expected tool error")
	}
}

func TestDatabaseQueryRejectsMutation(t *testing.T) {
	result, err := fixtureServer().databaseQuery(context.Background(), request(map[string]any{
		"connection": "default",
		"sql":        "delete from users",
	}))
	if err != nil {
		t.Fatalf("database query failed: %v", err)
	}
	if !result.IsError {
		t.Fatalf("expected tool error")
	}
}

func TestGetAbsoluteURL(t *testing.T) {
	result, err := fixtureServer().getAbsoluteURL(context.Background(), request(map[string]any{
		"path": "/health",
	}))
	if err != nil {
		t.Fatalf("get absolute url failed: %v", err)
	}
	text := resultText(t, result)
	if !strings.Contains(text, "http://localhost:3000/health") {
		t.Fatalf("unexpected result %s", text)
	}
}

func TestBrowserLogsDoesNotRequireLighthouse(t *testing.T) {
	result, err := fixtureServer().browserLogs(context.Background(), request(map[string]any{"limit": 1}))
	if err != nil {
		t.Fatalf("browser logs failed: %v", err)
	}
	text := resultText(t, result)
	if !strings.Contains(text, "hydrated") {
		t.Fatalf("unexpected result %s", text)
	}
}

func TestMetricsMetadata(t *testing.T) {
	result, err := fixtureServer().metricsMetadata(context.Background(), request(map[string]any{
		"app":     "app",
		"runtime": "http",
	}))
	if err != nil {
		t.Fatalf("metrics metadata failed: %v", err)
	}
	text := resultText(t, result)
	if !strings.Contains(text, "http_requests_total") {
		t.Fatalf("unexpected result %s", text)
	}
}

func request(args map[string]any) mcpgo.CallToolRequest {
	return mcpgo.CallToolRequest{
		Params: mcpgo.CallToolParams{
			Arguments: args,
		},
	}
}

func resultText(t *testing.T, result *mcpgo.CallToolResult) string {
	t.Helper()
	content, err := json.Marshal(result.StructuredContent)
	if err != nil {
		t.Fatalf("marshal result content: %v", err)
	}
	return string(content)
}

func fixtureServer() Server {
	return Server{
		Version: "test",
		Project: project.Project{
			Name:          "demo",
			GoVersion:     "1.24",
			GoForjVersion: "0.18.0",
			Components:    []string{"web-api", "jobs"},
			Apps: []project.App{
				{Name: "app", Default: true, Runtimes: []string{"http", "jobs"}},
				{Name: "marketplace", Runtimes: []string{"http"}},
			},
		},
		Docs: atlasdocs.StaticProvider{
			DocsMeta: atlasdocs.Manifest{Version: "0.18.0", Revision: "rev1"},
			Docs: []atlasdocs.Document{
				{
					Path:  "apps.md",
					Title: "Apps",
					Content: `# Apps

## Make Commands

Use forj marketplace make:controller checkout.
`,
				},
			},
		},
		Inventory: Inventory{
			Routes: map[string][]string{
				"app":         {"GET /health"},
				"marketplace": {"GET /checkout"},
			},
		},
		Diagnostics: diagnostics.StaticProvider{
			Connections: []diagnostics.DatabaseConnection{{Name: "default", Driver: "sqlite"}},
			Queries: map[string]diagnostics.QueryResult{
				"select * from users": {Connection: "default", Columns: []string{"id"}},
			},
			Logs: []diagnostics.LogEntry{
				{App: "app", Runtime: "http", Level: "info", Message: "started"},
				{App: "app", Runtime: "http", Level: "error", Message: "failed"},
			},
			BaseURLs: map[string]string{"app": "http://localhost:3000"},
			Browser: []diagnostics.BrowserLogEntry{
				{App: "app", Level: "info", Message: "hydrated"},
			},
			Metrics: map[string]diagnostics.MetricsMetadata{
				"app/http": {
					App:     "app",
					Runtime: "http",
					Labels:  map[string]string{"app": "app", "runtime": "http"},
					Counters: []string{
						"http_requests_total",
					},
				},
			},
		},
	}
}
