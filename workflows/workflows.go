package workflows

import (
	"context"
	"strings"

	"github.com/goforj/atlas/diagnostics"
	atlasdocs "github.com/goforj/atlas/docs"
	"github.com/goforj/atlas/project"
)

// Inventory contains app-aware project facts used by workflow planning.
type Inventory struct {
	Routes     map[string][]string `json:"routes,omitempty"`
	Schedules  map[string][]string `json:"schedules,omitempty"`
	Commands   map[string][]string `json:"commands,omitempty"`
	Queues     []string            `json:"queues,omitempty"`
	Caches     []string            `json:"caches,omitempty"`
	Disks      []string            `json:"disks,omitempty"`
	EventBuses []string            `json:"event_buses,omitempty"`
	Resources  []ResourceLink      `json:"resources,omitempty"`
}

// Context contains the local project facts used by workflow helpers.
type Context struct {
	Project     project.Project                  `json:"project"`
	Inventory   Inventory                        `json:"inventory,omitempty"`
	Connections []diagnostics.DatabaseConnection `json:"connections,omitempty"`
	Overlays    []string                         `json:"overlays,omitempty"`
}

// PlanRequest asks Atlas for a workflow plan for a task.
type PlanRequest struct {
	Task string `json:"task"`
	App  string `json:"app,omitempty"`
}

// PlanResult describes the framework workflow an agent should follow.
type PlanResult struct {
	WorkflowID   string             `json:"workflow_id"`
	WorkflowIDs  []string           `json:"workflow_ids,omitempty"`
	Segments     []WorkflowSegment  `json:"segments,omitempty"`
	Steps        []PlanStep         `json:"steps,omitempty"`
	App          string             `json:"app"`
	Tools        []string           `json:"tools,omitempty"`
	Commands     []string           `json:"commands,omitempty"`
	Files        []string           `json:"files,omitempty"`
	Ownership    []FilePolicyResult `json:"ownership,omitempty"`
	Docs         []DocReference     `json:"docs,omitempty"`
	Overlays     []string           `json:"overlays,omitempty"`
	Verification []ValidationStep   `json:"verification,omitempty"`
	Warnings     []string           `json:"warnings,omitempty"`
}

// WorkflowSegment preserves one concern inside a compositional application plan.
type WorkflowSegment struct {
	WorkflowID string         `json:"workflow_id"`
	Commands   []string       `json:"commands,omitempty"`
	Files      []string       `json:"files,omitempty"`
	Docs       []DocReference `json:"docs,omitempty"`
	Warnings   []string       `json:"warnings,omitempty"`
	Steps      []PlanStep     `json:"steps,omitempty"`
}

// PlanStep records one ordered prerequisite or implementation action.
type PlanStep struct {
	Order      int    `json:"order"`
	WorkflowID string `json:"workflow_id"`
	Action     string `json:"action"`
	Purpose    string `json:"purpose"`
}

// DocReference points an agent at a focused docs section.
type DocReference struct {
	Path    string `json:"path"`
	Heading string `json:"heading,omitempty"`
	Reason  string `json:"reason,omitempty"`
}

// ValidationStep describes one verification command or Atlas inspection.
type ValidationStep struct {
	Command  string `json:"command"`
	Purpose  string `json:"purpose"`
	Required bool   `json:"required"`
}

// RegistrationPoint describes one app-owned registration surface.
type RegistrationPoint struct {
	Kind              string   `json:"kind"`
	App               string   `json:"app"`
	Files             []string `json:"files"`
	ComponentRequired string   `json:"component_required,omitempty"`
}

// RegistrationResult describes registration surfaces for one app.
type RegistrationResult struct {
	App    string              `json:"app"`
	Points []RegistrationPoint `json:"points"`
}

// WireDiagnostic describes a likely Wire failure category and fix.
type WireDiagnostic struct {
	Category     string   `json:"category"`
	Message      string   `json:"message"`
	MissingType  string   `json:"missing_type,omitempty"`
	Consumer     string   `json:"consumer,omitempty"`
	ProviderSet  string   `json:"provider_set,omitempty"`
	LikelyFiles  []string `json:"likely_files,omitempty"`
	SuggestedFix string   `json:"suggested_fix"`
}

// ScenarioGuideResult points agents at verified scenario docs.
type ScenarioGuideResult struct {
	Query     string         `json:"query"`
	Scenarios []DocReference `json:"scenarios"`
}

// ResourceInventory describes named and runtime resources visible to Atlas.
type ResourceInventory struct {
	Apps        []project.App                    `json:"apps"`
	Components  []string                         `json:"components,omitempty"`
	FrontendKit string                           `json:"frontend_kit,omitempty"`
	Routes      map[string][]string              `json:"routes,omitempty"`
	Schedules   map[string][]string              `json:"schedules,omitempty"`
	Commands    map[string][]string              `json:"commands,omitempty"`
	Queues      []string                         `json:"queues,omitempty"`
	Caches      []string                         `json:"caches,omitempty"`
	Disks       []string                         `json:"disks,omitempty"`
	EventBuses  []string                         `json:"event_buses,omitempty"`
	Resources   []ResourceLink                   `json:"resources,omitempty"`
	Databases   []diagnostics.DatabaseConnection `json:"databases,omitempty"`
	Categories  []string                         `json:"categories,omitempty"`
}

// ResourceLink describes a dashboard, app URL, or operator-facing resource.
type ResourceLink struct {
	ID       string `json:"id"`
	Label    string `json:"label"`
	URL      string `json:"url,omitempty"`
	Category string `json:"category,omitempty"`
	Source   string `json:"source,omitempty"`
	App      string `json:"app,omitempty"`
	Runtime  string `json:"runtime,omitempty"`
	Health   string `json:"health,omitempty"`
	Auth     string `json:"auth,omitempty"`
	Owner    string `json:"owner,omitempty"`
}

// Plan returns a deterministic workflow plan for a GoForj task.
func Plan(ctx Context, req PlanRequest) (PlanResult, bool) {
	p := ctx.Project.WithDiscoveredDefaults()
	app, ok := p.AppByName(req.App)
	if !ok {
		return PlanResult{}, false
	}
	workflowID := Classify(req.Task)
	workflowIDs := ClassifyAll(req.Task)
	scope := makeScope(app)
	layout := appLayout(app)

	result := PlanResult{
		WorkflowID:  workflowID,
		WorkflowIDs: workflowIDs,
		App:         app.Name,
		Files:       mergeWorkflowFiles(workflowIDs, layout),
		Docs:        mergeWorkflowDocs(workflowIDs),
		Tools:       mergeWorkflowTools(workflowIDs),
		Overlays:    mergeOverlays(productOverlays(p, req.Task), matchingOverlays(ctx.Overlays, req.Task)),
		Commands:    mergeWorkflowCommands(workflowIDs, scope),
		Warnings:    mergeWorkflowWarnings(workflowIDs),
	}
	result.Segments, result.Steps = workflowSegments(workflowIDs, scope, layout)
	result.Ownership = FilePolicies(FilePolicyRequest{Project: p, Task: req.Task, WorkflowIDs: workflowIDs}, result.Files)
	result.Verification = ValidationPlan(ctx, PlanRequest{Task: req.Task, App: app.Name}).Steps
	return result, true
}

// ClassifyAll returns every independently useful workflow present in a broad task.
func ClassifyAll(task string) []string {
	lower := strings.ToLower(task)
	ids := []string{}
	add := func(id string, matched bool) {
		if matched && !stringSliceContains(ids, id) {
			ids = append(ids, id)
		}
	}
	add("goforj-wire-repair", containsAny(lower, "wire", "missing dependency", "duplicate provider"))
	add("goforj-add-data-resource", containsAny(lower, "database", "repository", "migration", "model", "persistence", "schema", "table"))
	add("goforj-add-http-route", containsAny(lower, "route", "controller", "http", "api", "json response", "upload"))
	add("goforj-add-job", containsAny(lower, "job", "queue", "worker", "background"))
	add("goforj-add-event-workflow", containsAny(lower, "event", "subscriber", "publish"))
	add("goforj-add-schedule", containsAny(lower, "schedule", "scheduler", "cron", "recurring", "daily"))
	add("goforj-add-app-command", containsAny(lower, "command", "cli", "operator"))
	add("goforj-frontend-change", containsAny(lower, "frontend", "ui", "view", "page", "react", "vue", "templ", "htmx"))
	add("goforj-debug-runtime", containsAny(lower, "debug", "diagnose", "runtime failure", "investigate logs", "troubleshoot"))
	add("goforj-multi-app-change", containsAny(lower, "named app", "multi-app", "additional app"))
	if len(ids) == 0 {
		return []string{Classify(task)}
	}
	return ids
}

// Classify maps task text to a stable GoForj workflow id.
func Classify(task string) string {
	lower := strings.ToLower(task)
	switch {
	case containsAny(lower, "wire", "provider", "missing dependency", "duplicate provider"):
		return "goforj-wire-repair"
	case containsAny(lower, "named app", "multi-app", "app selection", "selected app"):
		return "goforj-multi-app-change"
	case containsAny(lower, "debug", "runtime", "logs", "logging", "error", "lighthouse", "browser", "metrics"):
		return "goforj-debug-runtime"
	case containsAny(lower, "schedule", "scheduler", "cron", "recurring", "daily") && containsAny(lower, "job", "queue", "worker", "background"):
		return "goforj-add-job-schedule"
	case containsAny(lower, "route", "controller", "http endpoint", "api endpoint", "json response"):
		return "goforj-add-http-route"
	case containsAny(lower, "schedule", "scheduler", "cron", "recurring", "daily"):
		return "goforj-add-schedule"
	case containsAny(lower, "job", "queue", "worker", "background"):
		return "goforj-add-job"
	case containsAny(lower, "event", "subscriber", "publish"):
		return "goforj-add-event-workflow"
	case containsAny(lower, "database", "repository", "migration", "model", "cache", "storage", "disk"):
		return "goforj-add-data-resource"
	case containsAny(lower, "command", "cli", "operator"):
		return "goforj-add-app-command"
	case containsAny(lower, "validate", "test", "verify", "review"):
		return "goforj-validate-change"
	default:
		return "goforj-add-http-route"
	}
}

// RegistrationPoints returns app-owned registration surfaces for one app.
func RegistrationPoints(p project.Project, appName string) (RegistrationResult, bool) {
	app, ok := p.WithDiscoveredDefaults().AppByName(appName)
	if !ok {
		return RegistrationResult{}, false
	}
	layout := appLayout(app)
	return RegistrationResult{
		App: app.Name,
		Points: []RegistrationPoint{
			{Kind: "routes", App: app.Name, Files: []string{layout.composition + "/routes.go", layout.wire + "/inject_http_controllers_app.go"}, ComponentRequired: "web-api"},
			{Kind: "commands", App: app.Name, Files: []string{layout.composition + "/commands.go", layout.wire + "/inject_cmd_app.go"}, ComponentRequired: "cli"},
			{Kind: "jobs", App: app.Name, Files: []string{layout.wire + "/inject_jobs_app.go"}, ComponentRequired: "jobs"},
			{Kind: "schedules", App: app.Name, Files: []string{layout.composition + "/schedules.go", layout.wire + "/inject_schedules_app.go"}, ComponentRequired: "scheduler"},
			{Kind: "subscribers", App: app.Name, Files: []string{layout.composition + "/lifecycle.go", layout.wire + "/inject_subscribers_app.go"}, ComponentRequired: "events"},
			{Kind: "services", App: app.Name, Files: []string{layout.wire + "/inject_services_app.go"}},
			{Kind: "repositories", App: app.Name, Files: []string{layout.wire + "/inject_repositories_app.go"}, ComponentRequired: "database"},
		},
	}, true
}

// ValidationResult describes the checks Atlas recommends for a task.
type ValidationResult struct {
	App      string           `json:"app"`
	Steps    []ValidationStep `json:"steps"`
	Warnings []string         `json:"warnings,omitempty"`
}

// ValidationPlan returns task-aware verification commands.
func ValidationPlan(ctx Context, req PlanRequest) ValidationResult {
	p := ctx.Project.WithDiscoveredDefaults()
	app, _ := p.AppByName(req.App)
	workflowID := Classify(req.Task)
	workflowIDs := ClassifyAll(req.Task)
	scope := makeScope(app)
	steps := []ValidationStep{
		{Command: "GOCACHE=/tmp/gocache GOMODCACHE=/tmp/gomodcache go test ./...", Purpose: "run Go tests with repo cache policy", Required: true},
		{Command: "test -z \"$(gofmt -l $(find . -type f -name '*.go' -not -path './vendor/*'))\"", Purpose: "reject unformatted Go source before accepting the change", Required: true},
		{Command: scope.build, Purpose: "refresh generated code, Wire, API index, and binary", Required: true},
	}
	switch workflowID {
	case "goforj-add-http-route":
		steps = append(steps, ValidationStep{Command: scope.command("route:list"), Purpose: "confirm the route is registered", Required: true})
	case "goforj-add-app-command":
		steps = append(steps, ValidationStep{Command: "Atlas command-list", Purpose: "confirm the command is registered for the selected app", Required: true})
	case "goforj-add-job":
		steps = append(steps, ValidationStep{Command: scope.command("worker"), Purpose: "smoke worker startup when jobs are enabled", Required: false})
	case "goforj-add-job-schedule":
		steps = append(steps,
			ValidationStep{Command: "Atlas schedule-list", Purpose: "confirm the schedule is registered for the selected app", Required: true},
			ValidationStep{Command: scope.command("worker"), Purpose: "smoke worker startup for the scheduled durable job", Required: false},
		)
	case "goforj-add-schedule":
		steps = append(steps, ValidationStep{Command: "Atlas schedule-list", Purpose: "confirm the schedule is registered for the selected app", Required: true})
	case "goforj-debug-runtime":
		steps = append(steps, ValidationStep{Command: "Atlas read-log-entries and last-error", Purpose: "inspect recent runtime failures before guessing", Required: true})
	}
	warnings := []string{}
	if stringSliceContains(workflowIDs, "goforj-add-data-resource") {
		warnings = append(warnings, "Verify persisted models and repository constructors came from forj make:model and are registered in the selected App's repository Wire set; application-specific repository methods may be added afterward.")
	}
	return ValidationResult{App: app.Name, Steps: steps, Warnings: warnings}
}

// DiagnoseWire classifies common Wire error output.
func DiagnoseWire(output string) []WireDiagnostic {
	lower := strings.ToLower(output)
	files := []string{"app/wire/inject_services_app.go", "app/wire/inject_http_controllers_app.go", "app/wire/inject_cmd_app.go", "app/wire/inject_jobs_app.go", "app/wire/inject_schedules_app.go"}
	switch {
	case containsAny(lower, "no provider found", "missing provider"):
		return []WireDiagnostic{enrichWireDiagnostic(output, WireDiagnostic{Category: "missing-provider", Message: "Wire cannot construct a required dependency.", LikelyFiles: files, SuggestedFix: "Add the constructor or provider function to the most specific app Wire set that owns the dependency."})}
	case containsAny(lower, "multiple bindings", "multiple providers", "duplicate provider"):
		return []WireDiagnostic{enrichWireDiagnostic(output, WireDiagnostic{Category: "duplicate-provider", Message: "Wire found more than one provider for the same type.", LikelyFiles: files, SuggestedFix: "Remove the duplicate provider or introduce a domain-specific adapter type so the graph is unambiguous."})}
	case containsAny(lower, "does not implement", "cannot use", "wrong type"):
		return []WireDiagnostic{enrichWireDiagnostic(output, WireDiagnostic{Category: "type-mismatch", Message: "A provider returns a type that does not match the consumer boundary.", LikelyFiles: files, SuggestedFix: "Align the constructor parameter, provider return type, or Wire interface binding."})}
	case containsAny(lower, "wire_gen.go", "stale"):
		return []WireDiagnostic{enrichWireDiagnostic(output, WireDiagnostic{Category: "stale-generated-output", Message: "Generated Wire output may be stale.", LikelyFiles: []string{"app/wire/wire_gen.go"}, SuggestedFix: "Run forj build and do not edit wire_gen.go by hand."})}
	default:
		return []WireDiagnostic{enrichWireDiagnostic(output, WireDiagnostic{Category: "unknown", Message: "The Wire error did not match a known Atlas category.", LikelyFiles: files, SuggestedFix: "Read the provider chain from the first missing or duplicate type and fix the owning app Wire set."})}
	}
}

// ScenarioGuide returns verified scenario references relevant to a query.
func ScenarioGuide(ctx context.Context, provider atlasdocs.Provider, query string) (ScenarioGuideResult, error) {
	refs := workflowScenarioRefs(ClassifyAll(query))
	if len(refs) > 0 {
		return ScenarioGuideResult{Query: query, Scenarios: refs}, nil
	}
	results, err := atlasdocs.Search(ctx, provider, atlasdocs.SearchOptions{Query: "scenarios " + query, Limit: 5, TokenLimit: 40})
	if err != nil {
		return ScenarioGuideResult{}, err
	}
	refs = []DocReference{}
	for _, result := range results {
		if strings.HasPrefix(result.Path, "scenarios/") {
			refs = append(refs, DocReference{Path: result.Path, Heading: result.Heading, Reason: result.Snippet})
		}
	}
	if len(refs) == 0 {
		refs = defaultScenarioRefs(query)
	}
	return ScenarioGuideResult{Query: query, Scenarios: refs}, nil
}

// Resources returns project resources visible to Atlas planning.
func Resources(ctx Context) ResourceInventory {
	p := ctx.Project.WithDiscoveredDefaults()
	categories := []string{"apps"}
	if len(ctx.Inventory.Routes) > 0 {
		categories = append(categories, "routes")
	}
	if len(ctx.Inventory.Commands) > 0 {
		categories = append(categories, "commands")
	}
	if len(ctx.Inventory.Schedules) > 0 {
		categories = append(categories, "schedules")
	}
	if len(ctx.Connections) > 0 {
		categories = append(categories, "databases")
	}
	if len(ctx.Inventory.Queues) > 0 {
		categories = append(categories, "queues")
	}
	if len(ctx.Inventory.Caches) > 0 {
		categories = append(categories, "caches")
	}
	if len(ctx.Inventory.Disks) > 0 {
		categories = append(categories, "disks")
	}
	if len(ctx.Inventory.EventBuses) > 0 {
		categories = append(categories, "event_buses")
	}
	if len(ctx.Inventory.Resources) > 0 {
		categories = append(categories, "resources")
	}
	return ResourceInventory{
		Apps:        p.Apps,
		Components:  append([]string(nil), p.Components...),
		FrontendKit: p.FrontendKit,
		Routes:      cloneMap(ctx.Inventory.Routes),
		Schedules:   cloneMap(ctx.Inventory.Schedules),
		Commands:    cloneMap(ctx.Inventory.Commands),
		Queues:      append([]string(nil), ctx.Inventory.Queues...),
		Caches:      append([]string(nil), ctx.Inventory.Caches...),
		Disks:       append([]string(nil), ctx.Inventory.Disks...),
		EventBuses:  append([]string(nil), ctx.Inventory.EventBuses...),
		Resources:   append([]ResourceLink(nil), ctx.Inventory.Resources...),
		Databases:   append([]diagnostics.DatabaseConnection(nil), ctx.Connections...),
		Categories:  categories,
	}
}

type appScope struct {
	make   string
	build  string
	prefix string
}

type layout struct {
	composition string
	wire        string
}

func makeScope(app project.App) appScope {
	if app.Name == "" || app.Default || app.Name == project.DefaultAppName {
		return appScope{make: "forj make:*", build: "forj build"}
	}
	return appScope{make: "forj " + app.Name + " make:*", build: "forj " + app.Name + " build", prefix: "forj " + app.Name + " "}
}

func (s appScope) command(name string) string {
	if s.prefix == "" {
		return "forj " + name
	}
	return s.prefix + name
}

func appLayout(app project.App) layout {
	if app.Name == "" || app.Default || app.Name == project.DefaultAppName {
		return layout{composition: "app", wire: "app/wire"}
	}
	return layout{composition: "app/" + app.Name, wire: "app/" + app.Name + "/wire"}
}

func workflowCommands(workflowID string, scope appScope) []string {
	switch workflowID {
	case "goforj-add-http-route":
		return []string{strings.Replace(scope.make, "*", "controller <name>", 1), scope.build, scope.command("route:list")}
	case "goforj-add-app-command":
		return []string{strings.Replace(scope.make, "*", "command <name>", 1), scope.build}
	case "goforj-add-job":
		return []string{strings.Replace(scope.make, "*", "job <name>", 1), scope.build, scope.command("worker")}
	case "goforj-add-job-schedule":
		return []string{strings.Replace(scope.make, "*", "job <name>", 1), strings.Replace(scope.make, "*", "schedule <name> --every <duration>", 1), scope.build, scope.command("worker")}
	case "goforj-add-schedule":
		return []string{strings.Replace(scope.make, "*", "schedule <name> --every <duration>", 1), scope.build}
	case "goforj-add-event-workflow":
		return []string{strings.Replace(scope.make, "*", "event <name>", 1), strings.Replace(scope.make, "*", "subscriber <name>", 1), scope.build}
	case "goforj-add-data-resource":
		return []string{strings.Replace(scope.make, "*", "migration <name>", 1), scope.command("migrate"), "Atlas database-schema", strings.Replace(scope.make, "*", "model <table> --package <package>", 1), scope.build}
	case "goforj-frontend-change":
		return []string{scope.build}
	default:
		return []string{scope.build}
	}
}

func workflowFiles(workflowID string, layout layout) []string {
	switch workflowID {
	case "goforj-add-http-route":
		return []string{"internal/<domain>/controller.go", "internal/<domain>/service.go", "internal/<domain>/repository.go", layout.composition + "/routes.go", layout.wire + "/inject_http_controllers_app.go", layout.wire + "/inject_services_app.go", layout.wire + "/inject_repositories_app.go"}
	case "goforj-add-app-command":
		return []string{"internal/<domain>/<name>_cmd.go", layout.composition + "/commands.go", layout.wire + "/inject_cmd_app.go"}
	case "goforj-add-job":
		return []string{"internal/<domain>/<name>_job.go", layout.wire + "/inject_jobs_app.go"}
	case "goforj-add-job-schedule":
		return []string{"internal/<domain>/<name>_job.go", "internal/<domain>/<name>_schedule.go", layout.composition + "/schedules.go", layout.wire + "/inject_jobs_app.go", layout.wire + "/inject_schedules_app.go"}
	case "goforj-add-schedule":
		return []string{"internal/<domain>/<name>_schedule.go", layout.composition + "/schedules.go", layout.wire + "/inject_schedules_app.go"}
	case "goforj-add-event-workflow":
		return []string{"internal/<domain>/<name>_event.go", "internal/<domain>/<name>_subscriber.go", layout.composition + "/lifecycle.go", layout.wire + "/inject_subscribers_app.go"}
	case "goforj-add-data-resource":
		return []string{"migrations/<app>/<connection>/", ".db-relationships.yaml", "internal/<package>/<model>.go", "internal/<package>/repository.go", layout.wire + "/inject_repositories_app.go", layout.wire + "/inject_services_app.go"}
	case "goforj-frontend-change":
		return []string{"cmd/<app>/frontend/"}
	case "goforj-wire-repair":
		return []string{layout.wire + "/inject_services_app.go", layout.wire + "/inject_http_controllers_app.go", layout.wire + "/inject_cmd_app.go", layout.wire + "/inject_jobs_app.go", layout.wire + "/inject_schedules_app.go"}
	default:
		return []string{layout.composition, layout.wire, "internal"}
	}
}

func workflowTools(workflowID string) []string {
	tools := []string{"application-info", "project-layout", "workflow-plan", "registration-points", "docs-section-pack", "generated-file-policy", "validation-plan"}
	switch workflowID {
	case "goforj-add-http-route":
		return append(tools, "route-list", "resource-inventory")
	case "goforj-add-app-command":
		return append(tools, "command-list")
	case "goforj-add-job":
		return append(tools, "scenario-guide", "resource-inventory")
	case "goforj-add-job-schedule":
		return append(tools, "scenario-guide", "schedule-list", "resource-inventory")
	case "goforj-add-schedule":
		return append(tools, "schedule-list")
	case "goforj-add-event-workflow":
		return append(tools, "scenario-guide")
	case "goforj-add-data-resource":
		return append(tools, "database-connections", "database-schema", "resource-inventory")
	case "goforj-frontend-change":
		return append(tools, "resource-inventory")
	case "goforj-wire-repair":
		return append(tools, "wire-diagnostics")
	case "goforj-debug-runtime":
		return append(tools, "read-log-entries", "last-error", "get-absolute-url", "browser-logs", "metrics-metadata", "resource-inventory")
	default:
		return tools
	}
}

func workflowDocs(workflowID string) []DocReference {
	if refs := WorkflowDocsMap[workflowID]; len(refs) > 0 {
		return append([]DocReference(nil), refs...)
	}
	return []DocReference{{Path: "core/make-commands.md", Heading: "Command Map"}}
}

func workflowWarnings(workflowID string) []string {
	switch workflowID {
	case "goforj-add-job":
		return []string{"Keep queue payloads small and typed.", "Delegate business behavior to services from the job handler."}
	case "goforj-add-job-schedule":
		return []string{"Use the schedule to dispatch durable queue work instead of doing long-running work inline.", "Keep the job payload small and typed."}
	case "goforj-add-schedule":
		return []string{"Schedules are not durable queues.", "Do not hide production schedules behind anonymous callbacks."}
	case "goforj-wire-repair":
		return []string{"Do not edit wire_gen.go by hand.", "Do not add nil guards around required constructor dependencies."}
	case "goforj-debug-runtime":
		return []string{"Inspect app/runtime identity and recent logs before changing code."}
	case "goforj-add-data-resource":
		return []string{
			"Never hand-create GORM models or persistence repositories when make:model applies. If the table does not exist, create and apply its migration first.",
			"Use domain-native table names. Do not add a project-name prefix unless an existing schema or explicit requirement requires it.",
			"Keep application-specific data access methods in the generated repository's package after generation.",
		}
	default:
		return nil
	}
}

func defaultScenarioRefs(query string) []DocReference {
	workflowID := Classify(query)
	switch workflowID {
	case "goforj-add-job":
		return []DocReference{{Path: "scenarios/reports-generate-job.md", Heading: "What You Will Build"}}
	case "goforj-add-schedule":
		return []DocReference{{Path: "scenarios/reports-daily-schedule.md", Heading: "What You Will Build"}}
	case "goforj-add-data-resource":
		return []DocReference{{Path: "scenarios/cached-user-profile.md", Heading: "What You Will Build"}, {Path: "scenarios/file-upload-storage.md", Heading: "What You Will Build"}}
	case "goforj-debug-runtime":
		return []DocReference{{Path: "scenarios/runtime-observability.md", Heading: "What You Will Observe"}}
	default:
		return []DocReference{{Path: "scenarios/json-api-route.md", Heading: "What You Will Build"}}
	}
}

// workflowScenarioRefs returns verified scenarios in workflow order before fuzzy search can introduce unrelated results.
func workflowScenarioRefs(workflowIDs []string) []DocReference {
	refs := []DocReference{}
	for _, workflowID := range workflowIDs {
		var candidates []DocReference
		switch workflowID {
		case "goforj-add-data-resource":
			candidates = []DocReference{{Path: "scenarios/create-user-model.md", Heading: "What You Will Build"}, {Path: "scenarios/create-photo-data-resource.md", Heading: "Starting State"}, {Path: "scenarios/user-post-relationships.md", Heading: "What You Will Build"}}
		case "goforj-add-http-route":
			candidates = []DocReference{{Path: "scenarios/json-api-route.md", Heading: "What You Will Build"}}
		case "goforj-add-job":
			candidates = []DocReference{{Path: "scenarios/reports-generate-job.md", Heading: "What You Will Build"}}
		case "goforj-add-event-workflow":
			candidates = []DocReference{{Path: "scenarios/users-created-event.md", Heading: "What You Will Build"}}
		case "goforj-add-schedule":
			candidates = []DocReference{{Path: "scenarios/reports-daily-schedule.md", Heading: "What You Will Build"}}
		case "goforj-debug-runtime":
			candidates = []DocReference{{Path: "scenarios/runtime-observability.md", Heading: "What You Will Observe"}}
		}
		for _, candidate := range candidates {
			if !docRefsContain(refs, candidate.Path) {
				refs = append(refs, candidate)
			}
		}
	}
	return refs
}

// mergeWorkflowFiles keeps a broad plan flat and deduplicated while segments retain concern boundaries.
func mergeWorkflowFiles(workflowIDs []string, layout layout) []string {
	values := []string{}
	for _, workflowID := range workflowIDs {
		values = appendUniqueStrings(values, workflowFiles(workflowID, layout)...)
	}
	return values
}

// mergeWorkflowDocs keeps docs in workflow order without repeating shared references.
func mergeWorkflowDocs(workflowIDs []string) []DocReference {
	refs := []DocReference{}
	for _, workflowID := range workflowIDs {
		for _, ref := range workflowDocs(workflowID) {
			if !docRefsContain(refs, ref.Path) {
				refs = append(refs, ref)
			}
		}
	}
	return refs
}

// mergeWorkflowTools keeps tool selection stable for deterministic agent plans.
func mergeWorkflowTools(workflowIDs []string) []string {
	values := []string{}
	for _, workflowID := range workflowIDs {
		values = appendUniqueStrings(values, workflowTools(workflowID)...)
	}
	return values
}

// mergeWorkflowCommands preserves prerequisite order and runs shared build commands once.
func mergeWorkflowCommands(workflowIDs []string, scope appScope) []string {
	values := []string{}
	postBuild := []string{}
	buildRequired := false
	for _, workflowID := range workflowIDs {
		for _, command := range workflowCommands(workflowID, scope) {
			if command == scope.build {
				buildRequired = true
				continue
			}
			if isPostBuildCommand(command) {
				postBuild = appendUniqueStrings(postBuild, command)
				continue
			}
			values = appendUniqueStrings(values, command)
		}
	}
	if buildRequired {
		values = append(values, scope.build)
	}
	return append(values, postBuild...)
}

// mergeWorkflowWarnings retains each workflow's guardrails without duplicating common advice.
func mergeWorkflowWarnings(workflowIDs []string) []string {
	values := []string{}
	for _, workflowID := range workflowIDs {
		values = appendUniqueStrings(values, workflowWarnings(workflowID)...)
	}
	if stringSliceContains(workflowIDs, "goforj-add-job") && stringSliceContains(workflowIDs, "goforj-add-schedule") {
		values = appendUniqueStrings(values, workflowWarnings("goforj-add-job-schedule")...)
	}
	return values
}

// workflowSegments gives broad tasks explicit concern boundaries and one global ordered step list.
func workflowSegments(workflowIDs []string, scope appScope, layout layout) ([]WorkflowSegment, []PlanStep) {
	segments := make([]WorkflowSegment, 0, len(workflowIDs))
	steps := []PlanStep{}
	postBuild := []PlanStep{}
	for _, workflowID := range workflowIDs {
		segmentSteps := workflowSteps(workflowID, scope)
		keptSteps := segmentSteps[:0]
		for _, step := range segmentSteps {
			if isPostBuildCommand(step.Action) {
				postBuild = append(postBuild, step)
				continue
			}
			keptSteps = append(keptSteps, step)
		}
		segmentSteps = keptSteps
		for index := range segmentSteps {
			segmentSteps[index].Order = len(steps) + index + 1
		}
		steps = append(steps, segmentSteps...)
		segments = append(segments, WorkflowSegment{
			WorkflowID: workflowID,
			Commands:   workflowCommands(workflowID, scope),
			Files:      workflowFiles(workflowID, layout),
			Docs:       workflowDocs(workflowID),
			Warnings:   workflowWarnings(workflowID),
			Steps:      segmentSteps,
		})
	}
	steps = append(steps, PlanStep{Order: len(steps) + 1, WorkflowID: "goforj-validate-change", Action: scope.build, Purpose: "regenerate shared outputs and verify the complete App after all workflow segments"})
	for _, step := range postBuild {
		step.Order = len(steps) + 1
		steps = append(steps, step)
	}
	return segments, steps
}

// workflowSteps makes data prerequisites explicit while other workflows preserve their command order.
func workflowSteps(workflowID string, scope appScope) []PlanStep {
	if workflowID == "goforj-add-data-resource" {
		return []PlanStep{
			{WorkflowID: workflowID, Action: strings.Replace(scope.make, "*", "migration <name>", 1), Purpose: "create the connection-specific up/down migration pair"},
			{WorkflowID: workflowID, Action: "edit the generated migration SQL", Purpose: "define a domain-native schema without unnecessary project-name prefixes"},
			{WorkflowID: workflowID, Action: scope.command("migrate"), Purpose: "apply the schema before model introspection"},
			{WorkflowID: workflowID, Action: "Atlas database-schema", Purpose: "inspect the applied table and confirm its columns"},
			{WorkflowID: workflowID, Action: "declare relationships in .db-relationships.yaml when needed", Purpose: "make relationship generation explicit"},
			{WorkflowID: workflowID, Action: strings.Replace(scope.make, "*", "model <table> --package <package>", 1), Purpose: "derive the model, repository, and Wire registration from the live schema"},
			{WorkflowID: workflowID, Action: "extend the generated repository package", Purpose: "keep application-specific data access behind the repository boundary"},
		}
	}
	commands := workflowCommands(workflowID, scope)
	steps := make([]PlanStep, 0, len(commands))
	for _, command := range commands {
		if command == scope.build {
			continue
		}
		steps = append(steps, PlanStep{WorkflowID: workflowID, Action: command, Purpose: "complete the " + workflowID + " workflow"})
	}
	return steps
}

// appendUniqueStrings adds values once while preserving their first-seen order.
func appendUniqueStrings(existing []string, values ...string) []string {
	for _, value := range values {
		if !stringSliceContains(existing, value) {
			existing = append(existing, value)
		}
	}
	return existing
}

// isPostBuildCommand keeps runtime and registration inspection behind the shared generation pass.
func isPostBuildCommand(command string) bool {
	return strings.HasSuffix(command, "route:list") || strings.HasSuffix(command, " worker") || command == "forj worker"
}

func containsAny(value string, needles ...string) bool {
	for _, needle := range needles {
		if strings.Contains(value, needle) {
			return true
		}
	}
	return false
}

func cloneMap(values map[string][]string) map[string][]string {
	if len(values) == 0 {
		return nil
	}
	out := make(map[string][]string, len(values))
	for key, value := range values {
		out[key] = append([]string(nil), value...)
	}
	return out
}
