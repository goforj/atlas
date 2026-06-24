package mcp

import (
	"context"
	"encoding/json"
	"strings"
	"testing"

	"github.com/goforj/atlas/config"
	"github.com/goforj/atlas/diagnostics"
	atlasdocs "github.com/goforj/atlas/docs"
	"github.com/goforj/atlas/project"
	"github.com/goforj/atlas/workflows"
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

func TestRuntimeSnapshotFullEvidence(t *testing.T) {
	result, err := fixtureServer().runtimeSnapshot(context.Background(), request(map[string]any{"app": "app", "runtime": "http", "path": "/health"}))
	if err != nil {
		t.Fatalf("runtime snapshot failed: %v", err)
	}
	text := resultText(t, result)
	if !strings.Contains(text, `"absolute_url":"http://localhost:3000/health"`) ||
		!strings.Contains(text, `"confidence":"high"`) ||
		!strings.Contains(text, `"GET /health"`) ||
		!strings.Contains(text, `"browser_logs"`) ||
		!strings.Contains(text, `"targets":["`) {
		t.Fatalf("unexpected result %s", text)
	}
}

func TestRuntimeSnapshotPartialEvidenceDoesNotInventFacts(t *testing.T) {
	server := fixtureServer()
	server.Inventory.Routes = nil
	server.Inventory.Resources = nil
	server.Diagnostics = diagnostics.StaticProvider{}
	result, err := server.runtimeSnapshot(context.Background(), request(map[string]any{"app": "app", "runtime": "http", "path": "/missing"}))
	if err != nil {
		t.Fatalf("runtime snapshot failed: %v", err)
	}
	text := resultText(t, result)
	if !strings.Contains(text, `"confidence":"low"`) ||
		!strings.Contains(text, "absolute-url") ||
		!strings.Contains(text, "routes") ||
		strings.Contains(text, "localhost:3000") {
		t.Fatalf("unexpected result %s", text)
	}
}

func TestDebugPlanUsesRuntimeEvidence(t *testing.T) {
	result, err := fixtureServer().debugPlan(context.Background(), request(map[string]any{"app": "app", "runtime": "http", "path": "/health"}))
	if err != nil {
		t.Fatalf("debug plan failed: %v", err)
	}
	text := resultText(t, result)
	if !strings.Contains(text, `"tool":"route-list"`) ||
		!strings.Contains(text, `"tool":"get-absolute-url"`) ||
		!strings.Contains(text, `"tool":"read-log-entries"`) ||
		!strings.Contains(text, `"confidence":"high"`) {
		t.Fatalf("unexpected result %s", text)
	}
}

func TestWorkflowPlanNamedApp(t *testing.T) {
	result, err := fixtureServer().workflowPlan(context.Background(), request(map[string]any{
		"app":  "marketplace",
		"task": "add checkout route",
	}))
	if err != nil {
		t.Fatalf("workflow plan failed: %v", err)
	}
	text := resultText(t, result)
	if !strings.Contains(text, `"workflow_id":"goforj-add-http-route"`) || !strings.Contains(text, "forj marketplace make:controller") {
		t.Fatalf("unexpected result %s", text)
	}
}

func TestRegistrationPoints(t *testing.T) {
	result, err := fixtureServer().registrationPoints(context.Background(), request(map[string]any{"app": "marketplace"}))
	if err != nil {
		t.Fatalf("registration points failed: %v", err)
	}
	text := resultText(t, result)
	if !strings.Contains(text, "app/marketplace/routes.go") {
		t.Fatalf("unexpected result %s", text)
	}
}

func TestValidationPlan(t *testing.T) {
	result, err := fixtureServer().validationPlan(context.Background(), request(map[string]any{"task": "add schedule"}))
	if err != nil {
		t.Fatalf("validation plan failed: %v", err)
	}
	text := resultText(t, result)
	if !strings.Contains(text, "GOCACHE=/tmp/gocache") || !strings.Contains(text, "schedule-list") {
		t.Fatalf("unexpected result %s", text)
	}
}

func TestWireDiagnostics(t *testing.T) {
	result, err := fixtureServer().wireDiagnostics(context.Background(), request(map[string]any{"output": "wire: no provider found for *users.Service"}))
	if err != nil {
		t.Fatalf("wire diagnostics failed: %v", err)
	}
	text := resultText(t, result)
	if !strings.Contains(text, "missing-provider") {
		t.Fatalf("unexpected result %s", text)
	}
}

func TestScenarioGuide(t *testing.T) {
	result, err := fixtureServer().scenarioGuide(context.Background(), request(map[string]any{"query": "job"}))
	if err != nil {
		t.Fatalf("scenario guide failed: %v", err)
	}
	text := resultText(t, result)
	if !strings.Contains(text, "scenarios/reports-generate-job.md") {
		t.Fatalf("unexpected result %s", text)
	}
}

func TestResourceInventory(t *testing.T) {
	result, err := fixtureServer().resourceInventory(context.Background(), request(nil))
	if err != nil {
		t.Fatalf("resource inventory failed: %v", err)
	}
	text := resultText(t, result)
	if !strings.Contains(text, `"databases"`) || !strings.Contains(text, `"routes"`) || !strings.Contains(text, `"queues"`) || !strings.Contains(text, `"frontend_kit":"vue"`) {
		t.Fatalf("unexpected result %s", text)
	}
}

func TestGeneratedFilePolicy(t *testing.T) {
	result, err := fixtureServer().generatedFilePolicy(context.Background(), request(map[string]any{"path": "app/wire/wire_gen.go"}))
	if err != nil {
		t.Fatalf("generated file policy failed: %v", err)
	}
	text := resultText(t, result)
	if !strings.Contains(text, `"classification":"generated"`) || !strings.Contains(text, `"editable":false`) || !strings.Contains(text, `"preferred_action":"regenerate"`) {
		t.Fatalf("unexpected result %s", text)
	}
}

func TestGeneratedFilePolicyUsesProjectOwnershipOverrides(t *testing.T) {
	root := t.TempDir()
	if err := config.Save(root, config.Config{
		OwnershipRules: []config.OwnershipRuleConfig{{
			Pattern:         "docs/generated/**",
			Classification:  "generated-docs",
			Editable:        false,
			PreferredAction: "regenerate",
			ChangeThrough:   "forj docs:generate",
			Reason:          "Generated API docs are owned by the docs generator.",
		}},
	}); err != nil {
		t.Fatalf("save config: %v", err)
	}
	server := fixtureServer()
	server.Project.Root = root
	result, err := server.generatedFilePolicy(context.Background(), request(map[string]any{"path": "docs/generated/api.md"}))
	if err != nil {
		t.Fatalf("generated file policy failed: %v", err)
	}
	text := resultText(t, result)
	if !strings.Contains(text, `"classification":"generated-docs"`) || !strings.Contains(text, `"matched_rule":"docs/generated/**"`) || !strings.Contains(text, `"change_through":"forj docs:generate"`) {
		t.Fatalf("unexpected result %s", text)
	}
}

func TestCommandAdvice(t *testing.T) {
	result, err := fixtureServer().commandAdvice(context.Background(), request(map[string]any{
		"app":      "marketplace",
		"task":     "add background job",
		"resource": "sync-catalog",
	}))
	if err != nil {
		t.Fatalf("command advice failed: %v", err)
	}
	text := resultText(t, result)
	if !strings.Contains(text, "forj marketplace make:job sync-catalog") {
		t.Fatalf("unexpected result %s", text)
	}
}

func TestDocsSectionPack(t *testing.T) {
	result, err := fixtureServer().docsSectionPack(context.Background(), request(map[string]any{"workflow_id": "goforj-add-http-route"}))
	if err != nil {
		t.Fatalf("docs section pack failed: %v", err)
	}
	text := resultText(t, result)
	if !strings.Contains(text, `"workflow_id":"goforj-add-http-route"`) || !strings.Contains(text, "Where Routes Live") || !strings.Contains(text, `"alignment"`) {
		t.Fatalf("unexpected result %s", text)
	}
}

func TestVersionAlignment(t *testing.T) {
	result, err := fixtureServer().versionAlignment(context.Background(), request(nil))
	if err != nil {
		t.Fatalf("version alignment failed: %v", err)
	}
	text := resultText(t, result)
	if !strings.Contains(text, `"project_goforj_version":"0.18.0"`) || !strings.Contains(text, `"docs_version":"0.18.0"`) || !strings.Contains(text, `"aligned":`) {
		t.Fatalf("unexpected result %s", text)
	}
}

func TestWorkflowScorecard(t *testing.T) {
	result, err := fixtureServer().workflowScorecard(context.Background(), request(map[string]any{"capture_transcript": true}))
	if err != nil {
		t.Fatalf("workflow scorecard failed: %v", err)
	}
	text := resultText(t, result)
	if !strings.Contains(text, `"failed":0`) || !strings.Contains(text, `"transcript"`) {
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
			FrontendKit:   "vue",
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
				{
					Path:  "scenarios/reports-generate-job.md",
					Title: "Reports Generate Job",
					Content: `# Reports Generate Job

## What You Will Build

Dispatch durable reports:generate work from a users.created subscriber and process it with a queue worker.
`,
				},
				{
					Path:  "applications/routes.md",
					Title: "Routes",
					Content: `# Routes

## Where Routes Live

Application route composition lives in app routes.
`,
				},
				{
					Path:  "applications/controllers.md",
					Title: "Controllers",
					Content: `# Controllers

## Controller Shape

Controllers translate HTTP into service calls.
`,
				},
				{
					Path:  "core/wiring-recipes.md",
					Title: "Wiring Recipes",
					Content: `# Wiring Recipes

## HTTP Controller

Controllers belong in the HTTP controller set.
`,
				},
			},
		},
		Inventory: Inventory{
			Routes: map[string][]string{
				"app":         {"GET /health"},
				"marketplace": {"GET /checkout"},
			},
			Queues:     []string{"reports"},
			Caches:     []string{"profiles"},
			Disks:      []string{"uploads"},
			EventBuses: []string{"default"},
			Resources:  []workflows.ResourceLink{{ID: "app", Label: "App", URL: "http://localhost:3000", Category: "app", Source: "test"}},
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
					Targets: []string{"http://localhost:3000/metrics"},
				},
			},
		},
	}
}
