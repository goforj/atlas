package mcp

import (
	"context"
	"fmt"

	"github.com/goforj/atlas/project"
	mcpgo "github.com/mark3labs/mcp-go/mcp"
)

// applicationInfoResult is the structured payload returned by `application-info`.
type applicationInfoResult struct {
	Project       string        `json:"project"`
	GoForjVersion string        `json:"goforj_version"`
	GoVersion     string        `json:"go_version"`
	DocsRevision  string        `json:"docs_revision,omitempty"`
	Apps          []project.App `json:"apps"`
	Components    []string      `json:"components,omitempty"`
}

// projectLayoutResult is the structured payload returned by `project-layout`.
type projectLayoutResult struct {
	App              string   `json:"app"`
	Default          bool     `json:"default"`
	Entrypoint       string   `json:"entrypoint"`
	Composition      string   `json:"composition"`
	Wire             string   `json:"wire"`
	SharedCode       string   `json:"shared_code"`
	Registration     []string `json:"registration"`
	MakeCommandScope string   `json:"make_command_scope"`
}

// applicationInfo reports the project facts agents should request before guessing.
func (s Server) applicationInfo(ctx context.Context, _ mcpgo.CallToolRequest) (*mcpgo.CallToolResult, error) {
	ctx = _context(ctx)
	p := s.projectWithDefaults()
	manifest, _ := s.docsProvider().Manifest(ctx)
	return jsonResult(applicationInfoResult{
		Project:       p.Name,
		GoForjVersion: p.GoForjVersion,
		GoVersion:     p.GoVersion,
		DocsRevision:  manifest.Revision,
		Apps:          p.Apps,
		Components:    p.Components,
	})
}

// projectLayout explains where the selected app starts, composes, and registers behavior.
func (s Server) projectLayout(_ context.Context, request mcpgo.CallToolRequest) (*mcpgo.CallToolResult, error) {
	p := s.projectWithDefaults()
	app, ok := p.AppByName(appName(request))
	if !ok {
		return toolError(fmt.Errorf("app not found: %s", appName(request)))
	}

	composition := "app"
	entrypoint := "cmd/app/main.go"
	wire := "app/wire"
	scope := "forj make:*"
	if !app.Default && app.Name != project.DefaultAppName {
		composition = "app/" + app.Name
		entrypoint = "cmd/" + app.Name + "/main.go"
		wire = "app/" + app.Name + "/wire"
		scope = "forj " + app.Name + " make:*"
	}

	return jsonResult(projectLayoutResult{
		App:         app.Name,
		Default:     app.Default,
		Entrypoint:  entrypoint,
		Composition: composition,
		Wire:        wire,
		SharedCode:  "internal",
		Registration: []string{
			composition + "/commands.go",
			composition + "/routes.go",
			composition + "/lifecycle.go",
			composition + "/schedules.go",
			wire + "/inject_cmd_app.go",
			wire + "/inject_http_controllers_app.go",
			wire + "/inject_jobs_app.go",
			wire + "/inject_repositories_app.go",
			wire + "/inject_schedules_app.go",
			wire + "/inject_services_app.go",
			wire + "/inject_subscribers_app.go",
		},
		MakeCommandScope: scope,
	})
}

// routeList returns routes from the injected inventory provider.
func (s Server) routeList(_ context.Context, request mcpgo.CallToolRequest) (*mcpgo.CallToolResult, error) {
	return s.appInventory(request, "routes", s.Inventory.Routes)
}

// scheduleList returns schedules from the injected inventory provider.
func (s Server) scheduleList(_ context.Context, request mcpgo.CallToolRequest) (*mcpgo.CallToolResult, error) {
	return s.appInventory(request, "schedules", s.Inventory.Schedules)
}

// commandList returns commands from the injected inventory provider.
func (s Server) commandList(_ context.Context, request mcpgo.CallToolRequest) (*mcpgo.CallToolResult, error) {
	return s.appInventory(request, "commands", s.Inventory.Commands)
}

// appInventory centralizes app selection and unknown-app errors for inventory tools.
func (s Server) appInventory(request mcpgo.CallToolRequest, key string, values map[string][]string) (*mcpgo.CallToolResult, error) {
	p := s.projectWithDefaults()
	app, ok := p.AppByName(appName(request))
	if !ok {
		return toolError(fmt.Errorf("app not found: %s", appName(request)))
	}
	items := []string{}
	if values != nil {
		items = append(items, values[app.Name]...)
	}
	return jsonResult(map[string]any{
		"app":     app.Name,
		"default": app.Default,
		key:       items,
	})
}
