package workflows

import (
	"context"
	"regexp"
	"strings"

	atlasdocs "github.com/goforj/atlas/docs"
	"github.com/goforj/atlas/project"
)

// WorkflowDocsMap maps workflow ids to the docs sections agents should read first.
var WorkflowDocsMap = map[string][]DocReference{
	"goforj-catalog-search":        {{Path: "applications/library-resources.md", Heading: "Declare and Provision a Search Index"}, {Path: "libraries/search.md", Heading: "Quick Start {#quick-start}"}},
	"goforj-notification-delivery": {{Path: "applications/library-resources.md", Heading: "Configure a Notification Sender"}, {Path: "libraries/notification.md", Heading: "Composition {#composition}"}, {Path: "libraries/notification.md", Heading: "Custom senders {#custom-senders}"}, {Path: "async/events-vs-queues.md", Heading: "Default Recommendation"}},
	"goforj-feature-rollouts": {
		{Path: "applications/library-resources.md", Heading: "Configure a Feature Evaluator"},
		{Path: "libraries/feature.md", Heading: "Features.Snapshot {#features.snapshot}"},
	},
	"goforj-add-http-route": {
		{Path: "applications/routes.md", Heading: "Where Routes Live"},
		{Path: "applications/controllers.md", Heading: "Responsibilities"},
		{Path: "developer-tools/wiring-recipes.md", Heading: "HTTP Controller"},
	},
	"goforj-add-app-command": {
		{Path: "applications/commands.md", Heading: "Create a Command"},
		{Path: "developer-tools/wiring-recipes.md", Heading: "Command"},
	},
	"goforj-add-job": {
		{Path: "async/jobs.md", Heading: "Generate a Job"},
		{Path: "async/workers.md", Heading: "Execution Lifecycle"},
	},
	"goforj-add-job-schedule": {
		{Path: "async/jobs.md", Heading: "Generate a Job"},
		{Path: "async/scheduler.md", Heading: "Generate a Schedule"},
		{Path: "async/events-vs-queues.md", Heading: "Default Recommendation"},
	},
	"goforj-add-schedule": {
		{Path: "async/scheduler.md", Heading: "Generate a Schedule"},
		{Path: "operations/scheduler-processes.md", Heading: "Singleton Behavior"},
	},
	"goforj-add-event-workflow": {
		{Path: "async/events-vs-queues.md", Heading: "Default Recommendation"},
		{Path: "async/event-subscribers.md", Heading: "Registration Timing"},
	},
	"goforj-add-data-resource": {
		{Path: "data/repositories.md", Heading: "Repository Shape"},
		{Path: "data/migrations.md", Heading: "Create a Migration"},
		{Path: "data/driver-selection.md", Heading: "Two Decisions"},
	},
	"goforj-wire-repair": {
		{Path: "developer-tools/reading-wire-errors.md", Heading: "Fast Checklist"},
		{Path: "developer-tools/wiring-recipes.md", Heading: "Quick Map"},
	},
	"goforj-debug-runtime": {
		{Path: "operations/logging.md", Heading: "Good Default Logs"},
		{Path: "operations/metrics.md", Heading: "Labels"},
		{Path: "operations/inspects.md", Heading: "What Inspects Capture"},
	},
	"goforj-multi-app-change": {
		{Path: "core/apps.md", Heading: "App Identity and Command Selection"},
		{Path: "core/apps.md", Heading: "App-Owned Composition"},
	},
	"goforj-frontend-change": {
		{Path: "getting-started/starter-kits.md", Heading: "Choose the Rendering Model"},
		{Path: "core/local-first-development.md", Heading: "Default Path"},
	},
	"goforj-validate-change": {
		{Path: "testing/index.md", Heading: "Choose a Test Layer"},
		{Path: "reference/generation-commands.md", Heading: "Full Build Pipeline"},
	},
}

// DocsSection contains one bounded docs section for a workflow.
type DocsSection struct {
	Reference DocReference      `json:"reference"`
	Section   atlasdocs.Section `json:"section,omitempty"`
	Found     bool              `json:"found"`
}

// DocsSectionPackResult contains docs sections in workflow reading order.
type DocsSectionPackResult struct {
	WorkflowID string                  `json:"workflow_id"`
	Manifest   atlasdocs.Manifest      `json:"manifest,omitempty"`
	Alignment  *VersionAlignmentResult `json:"alignment,omitempty"`
	Sections   []DocsSection           `json:"sections"`
}

// DocsSectionPack reads the bounded docs sections for a workflow or task.
func DocsSectionPack(ctx context.Context, provider atlasdocs.Provider, workflowID string, task string, tokenLimit int) (DocsSectionPackResult, error) {
	if workflowID == "" {
		workflowID = Classify(task)
	}
	refs := applicationWorkflowDocs(workflowID, task)
	if len(refs) == 0 {
		refs = WorkflowDocsMap["goforj-add-http-route"]
	}
	manifest, _ := provider.Manifest(ctx)
	sections := make([]DocsSection, 0, len(refs))
	for _, ref := range refs {
		section, found, err := atlasdocs.ReadSection(ctx, provider, ref.Path, ref.Heading, tokenLimit)
		if err != nil {
			return DocsSectionPackResult{}, err
		}
		sections = append(sections, DocsSection{Reference: ref, Section: section, Found: found})
	}
	return DocsSectionPackResult{WorkflowID: workflowID, Manifest: manifest, Sections: sections}, nil
}

// CommandAdviceRequest asks Atlas for the preferred GoForj command for a task.
type CommandAdviceRequest struct {
	Task     string `json:"task"`
	App      string `json:"app,omitempty"`
	Resource string `json:"resource,omitempty"`
}

// CommandAdviceResult describes one preferred command.
type CommandAdviceResult struct {
	WorkflowID string `json:"workflow_id"`
	App        string `json:"app"`
	Command    string `json:"command"`
	Reason     string `json:"reason"`
}

// CommandAdvice returns the most specific make or validation command for a task.
func CommandAdvice(ctx Context, req CommandAdviceRequest) (CommandAdviceResult, bool) {
	p := ctx.Project.WithDiscoveredDefaults()
	app, ok := p.AppByName(req.App)
	if !ok {
		return CommandAdviceResult{}, false
	}
	workflowID := Classify(req.Task)
	resource := strings.TrimSpace(req.Resource)
	if resource == "" {
		resource = "<name>"
	}
	scope := makeScope(app)
	scope.searchConfirmed = projectHasSearch(p)
	command := firstCommand(workflowCommands(workflowID, scope))
	command = strings.ReplaceAll(command, "<name>", resource)
	reason := "GoForj make commands update generated files, App registration, and Wire surfaces together."
	if _, ok := applicationWorkflowDefinitions[workflowID]; ok {
		reason = "Update the application-owned boundary and provider inputs, then build and test the selected App."
	}
	if workflowID == "goforj-catalog-search" {
		reason = "Generate named search accessors from the owning App resource configuration, then provision its Go-defined schemas explicitly."
		if !scope.searchConfirmed {
			reason = searchCapabilityWarning
		}
	}
	if workflowID == "goforj-read-docs" {
		command = "Atlas search-docs"
		reason = "Read the requested documentation without proposing source changes."
	}
	if workflowID == "goforj-notification-delivery" {
		reason = "Declare NOTIFICATION_* sender resources before generating the manager and typed accessors for the owning App."
		if !projectHasNotificationComponent(p) {
			command = scope.command("generate --help")
			reason = "Inspect whether the selected CLI supports --notification before enabling the optional component."
		}
	}

	if workflowID == "goforj-feature-rollouts" {
		if projectHasFeatureComponent(p) {
			reason = "Declare FEATURE_* evaluator resources before generating their typed accessors for the owning App."
		} else {
			command = scope.command("generate --help")
			reason = "Inspect whether the selected CLI supports --feature before enabling or generating the optional component."
		}
	}
	return CommandAdviceResult{
		WorkflowID: workflowID,
		App:        app.Name,
		Command:    command,
		Reason:     reason,
	}, true
}

// EvalFixture describes a deterministic workflow planning scenario.
type EvalFixture struct {
	Name                string   `json:"name"`
	Task                string   `json:"task"`
	App                 string   `json:"app,omitempty"`
	WantWorkflowID      string   `json:"want_workflow_id"`
	WantWorkflowIDs     []string `json:"want_workflow_ids,omitempty"`
	WantCommandPart     string   `json:"want_command_part,omitempty"`
	WantCommandParts    []string `json:"want_command_parts,omitempty"`
	WantFilePart        string   `json:"want_file_part,omitempty"`
	WantFileParts       []string `json:"want_file_parts,omitempty"`
	WantDocsPath        string   `json:"want_docs_path,omitempty"`
	WantDocsPaths       []string `json:"want_docs_paths,omitempty"`
	WantTools           []string `json:"want_tools,omitempty"`
	WantValidationParts []string `json:"want_validation_parts,omitempty"`
	WantWarningParts    []string `json:"want_warning_parts,omitempty"`
	AvoidFileParts      []string `json:"avoid_file_parts,omitempty"`
}

// EvalFixtures returns common agent tasks used to guard workflow quality.
func EvalFixtures() []EvalFixture {
	fixtures := []EvalFixture{
		{
			Name:                "add route with service and repository",
			Task:                "add users HTTP route with service and repository",
			WantWorkflowID:      "goforj-add-http-route",
			WantCommandParts:    []string{"make:controller", "route:list"},
			WantFileParts:       []string{"internal/<domain>/controller.go", "internal/<domain>/service.go", "internal/<domain>/repository.go", "app/routes.go", "inject_http_controllers_app.go", "inject_repositories_app.go"},
			WantDocsPaths:       []string{"applications/routes.md", "applications/controllers.md", "developer-tools/wiring-recipes.md"},
			WantTools:           []string{"workflow-plan", "registration-points", "docs-section-pack", "generated-file-policy", "validation-plan", "route-list"},
			WantValidationParts: []string{"go test ./...", "forj build", "route:list"},
			AvoidFileParts:      []string{"wire_gen.go"},
		},
		{
			Name:                "repair missing wire provider",
			Task:                "fix wire missing provider for users service",
			WantWorkflowID:      "goforj-wire-repair",
			WantCommandParts:    []string{"forj build"},
			WantFileParts:       []string{"inject_services_app.go", "inject_http_controllers_app.go"},
			WantDocsPaths:       []string{"developer-tools/reading-wire-errors.md", "developer-tools/wiring-recipes.md"},
			WantTools:           []string{"wire-diagnostics", "registration-points", "generated-file-policy", "validation-plan"},
			WantValidationParts: []string{"go test ./...", "forj build"},
			WantWarningParts:    []string{"Do not edit wire_gen.go", "Do not add nil guards"},
			AvoidFileParts:      []string{"wire_gen.go"},
		},
		{
			Name:                "add scheduled durable job",
			Task:                "add reports daily schedule that dispatches a durable queue job",
			WantWorkflowID:      "goforj-add-job-schedule",
			WantCommandParts:    []string{"make:job", "make:schedule", "worker"},
			WantFileParts:       []string{"internal/<domain>/<name>_job.go", "internal/<domain>/<name>_schedule.go", "app/schedules.go", "inject_jobs_app.go", "inject_schedules_app.go"},
			WantDocsPaths:       []string{"async/jobs.md", "async/scheduler.md"},
			WantTools:           []string{"scenario-guide", "schedule-list", "resource-inventory", "validation-plan"},
			WantValidationParts: []string{"schedule-list", "worker"},
			WantWarningParts:    []string{"dispatch durable queue work", "payload small"},
			AvoidFileParts:      []string{"wire_gen.go"},
		},
		{
			Name:                "add named app route",
			Task:                "add checkout route",
			App:                 "marketplace",
			WantWorkflowID:      "goforj-add-http-route",
			WantCommandParts:    []string{"forj marketplace make:controller", "forj marketplace route:list"},
			WantFileParts:       []string{"app/marketplace/routes.go", "app/marketplace/wire/inject_http_controllers_app.go"},
			WantDocsPaths:       []string{"applications/routes.md", "developer-tools/wiring-recipes.md"},
			WantTools:           []string{"application-info", "registration-points", "route-list"},
			WantValidationParts: []string{"forj marketplace build", "forj marketplace route:list"},
			AvoidFileParts:      []string{"app/routes.go", "app/wire/inject_http_controllers_app.go", "wire_gen.go"},
		},
		{
			Name:                "debug browser route failure",
			Task:                "debug runtime route failure with browser logs and metrics",
			WantWorkflowID:      "goforj-debug-runtime",
			WantCommandParts:    []string{"forj build"},
			WantFileParts:       []string{"app", "app/wire", "internal"},
			WantDocsPaths:       []string{"operations/logging.md", "operations/metrics.md", "operations/inspects.md"},
			WantTools:           []string{"read-log-entries", "last-error", "get-absolute-url", "browser-logs", "metrics-metadata", "resource-inventory"},
			WantValidationParts: []string{"read-log-entries", "last-error"},
			WantWarningParts:    []string{"before changing code"},
			AvoidFileParts:      []string{"wire_gen.go"},
		},
	}
	return append(fixtures, RegressionFixtures()...)
}

// RegressionFixtures returns workflow cases captured from failed scorecard runs.
func RegressionFixtures() []EvalFixture {
	return []EvalFixture{
		{Name: "catalog job keyword regression", Task: "add sync catalog job", App: "marketplace", WantWorkflowID: "goforj-add-job", WantCommandPart: "forj marketplace make:job", WantFilePart: "inject_jobs_app.go", WantDocsPath: "async/jobs.md", WantTools: []string{"workflow-plan", "scenario-guide", "resource-inventory"}},
		{
			Name:           "photodrop compositional planning regression",
			Task:           "Build PhotoDrop with a photos database table and repository, upload API and gallery UI, thumbnail queue job, photo-created event subscriber, expired-share schedule, operator cleanup command, and existing observability.",
			WantWorkflowID: "goforj-add-job-schedule",
			WantWorkflowIDs: []string{
				"goforj-add-data-resource",
				"goforj-add-http-route",
				"goforj-add-job",
				"goforj-add-event-workflow",
				"goforj-add-schedule",
				"goforj-add-app-command",
				"goforj-frontend-change",
			},
			WantCommandParts: []string{"make:migration", "forj migrate", "make:model", "make:controller", "make:job", "make:event", "make:subscriber", "make:schedule", "make:command"},
			WantWarningParts: []string{"Never hand-create GORM models", "domain-native table names"},
		},
		{
			Name:           "library application composition generalization",
			Task:           "Build a library catalog with a database schema and repository, HTTP API controller, background indexing job, book-added event subscriber, recurring maintenance schedule, operator CLI command, and frontend page.",
			WantWorkflowID: "goforj-add-job-schedule",
			WantWorkflowIDs: []string{
				"goforj-add-data-resource",
				"goforj-add-http-route",
				"goforj-add-job",
				"goforj-add-event-workflow",
				"goforj-add-schedule",
				"goforj-add-app-command",
				"goforj-frontend-change",
			},
			WantCommandParts: []string{"make:migration", "make:model", "make:controller", "make:job", "make:event", "make:subscriber", "make:schedule", "make:command"},
		},
	}
}

var wireMissingPattern = regexp.MustCompile(`(?i)no provider found for ([^\n]+?)(?:\n|$)`)
var wireNeededByPattern = regexp.MustCompile(`(?i)needed by ([^\n]+?)(?:\n|$)`)

func enrichWireDiagnostic(output string, diagnostic WireDiagnostic) WireDiagnostic {
	if matches := wireMissingPattern.FindStringSubmatch(output); len(matches) == 2 {
		diagnostic.MissingType = strings.TrimSpace(matches[1])
	}
	if matches := wireNeededByPattern.FindStringSubmatch(output); len(matches) == 2 {
		diagnostic.Consumer = strings.TrimSpace(matches[1])
	}
	diagnostic.ProviderSet = likelyProviderSet(output)
	return diagnostic
}

func likelyProviderSet(output string) string {
	lower := strings.ToLower(output)
	switch {
	case strings.Contains(lower, "controller") || strings.Contains(lower, "http"):
		return "inject_http_controllers_app.go"
	case strings.Contains(lower, "command") || strings.Contains(lower, "cmd"):
		return "inject_cmd_app.go"
	case strings.Contains(lower, "job"):
		return "inject_jobs_app.go"
	case strings.Contains(lower, "schedule"):
		return "inject_schedules_app.go"
	case strings.Contains(lower, "repository"):
		return "inject_repositories_app.go"
	default:
		return "inject_services_app.go"
	}
}

func firstCommand(commands []string) string {
	if len(commands) == 0 {
		return "forj build"
	}
	return commands[0]
}

func matchingOverlays(overlays []string, task string) []string {
	if len(overlays) == 0 {
		return nil
	}
	task = strings.ToLower(task)
	matched := []string{}
	for _, overlay := range overlays {
		overlay = strings.TrimSpace(overlay)
		if overlay == "" {
			continue
		}
		normalized := strings.ReplaceAll(strings.ToLower(overlay), "-", " ")
		if strings.Contains(task, normalized) || strings.Contains(task, strings.ToLower(overlay)) {
			matched = append(matched, overlay)
		}
	}
	if len(matched) > 0 {
		return matched
	}
	return append([]string(nil), overlays...)
}

func productOverlays(project project.Project, task string) []string {
	lower := strings.ToLower(task)
	kit := strings.ToLower(strings.TrimSpace(project.FrontendKit))
	if kit == "" {
		return nil
	}
	if !containsAny(lower, "frontend", "ui", "view", "page", "screen", "dashboard", "login", "auth", "vue", "react", "templ", "htmx", "starter") {
		return nil
	}
	switch kit {
	case "vue":
		return []string{"goforj-vue-starter-kit"}
	case "react":
		return []string{"goforj-react-starter-kit"}
	case "templ_htmx", "templ-htmx", "templ", "htmx":
		return []string{"goforj-templ-htmx-starter-kit"}
	default:
		return nil
	}
}

func mergeOverlays(groups ...[]string) []string {
	seen := map[string]struct{}{}
	out := []string{}
	for _, group := range groups {
		for _, overlay := range group {
			overlay = strings.TrimSpace(overlay)
			if overlay == "" {
				continue
			}
			if _, ok := seen[overlay]; ok {
				continue
			}
			seen[overlay] = struct{}{}
			out = append(out, overlay)
		}
	}
	return out
}
