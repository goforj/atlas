package workflows

import (
	"context"
	"strings"
	"testing"

	"github.com/goforj/atlas/diagnostics"
	atlasdocs "github.com/goforj/atlas/docs"
	"github.com/goforj/atlas/project"
)

func TestPlanNamedAppHTTPRoute(t *testing.T) {
	result, ok := Plan(fixtureContext(), PlanRequest{Task: "add checkout route", App: "marketplace"})
	if !ok {
		t.Fatal("expected plan")
	}
	if result.WorkflowID != "goforj-add-http-route" {
		t.Fatalf("workflow = %q", result.WorkflowID)
	}
	if !contains(result.Commands, "forj marketplace make:controller <name>") {
		t.Fatalf("commands = %#v", result.Commands)
	}
	if !contains(result.Files, "app/marketplace/routes.go") {
		t.Fatalf("files = %#v", result.Files)
	}
}

func TestEvalFixturesPlanExpectedWorkflows(t *testing.T) {
	ctx := fixtureContext()
	for _, fixture := range EvalFixtures() {
		result, ok := Plan(ctx, PlanRequest{Task: fixture.Task, App: fixture.App})
		if !ok {
			t.Fatalf("%s: expected plan", fixture.Name)
		}
		if result.WorkflowID != fixture.WantWorkflowID {
			t.Fatalf("%s: workflow = %q, want %q", fixture.Name, result.WorkflowID, fixture.WantWorkflowID)
		}
		for _, part := range fixture.commandParts() {
			if !containsPart(result.Commands, part) {
				t.Fatalf("%s: commands = %#v, want part %q", fixture.Name, result.Commands, part)
			}
		}
		for _, part := range fixture.fileParts() {
			if !containsPart(result.Files, part) {
				t.Fatalf("%s: files = %#v, want part %q", fixture.Name, result.Files, part)
			}
		}
		for _, path := range fixture.docsPaths() {
			if !docsContain(result.Docs, path) {
				t.Fatalf("%s: docs = %#v, want path %q", fixture.Name, result.Docs, path)
			}
		}
	}
}

func TestRunEvalFixturesScorecardWithTranscript(t *testing.T) {
	scorecard := RunEvalFixtures(fixtureContext(), true)
	if scorecard.Total == 0 || scorecard.Failed != 0 || scorecard.Passed != scorecard.Total {
		t.Fatalf("scorecard = %#v", scorecard)
	}
	if len(scorecard.Results[0].Transcript) == 0 {
		t.Fatalf("expected transcript in %#v", scorecard.Results[0])
	}
	if len(scorecard.Results) < 5 {
		t.Fatalf("expected at least five realistic fixtures, got %d", len(scorecard.Results))
	}
	if len(scorecard.Results[0].Checks) == 0 {
		t.Fatalf("expected scored checks in %#v", scorecard.Results[0])
	}
}

func TestRunEvalFixturesReportsActionableFailures(t *testing.T) {
	ctx := fixtureContext()
	ctx.Project.Apps = []project.App{{Name: "app", Default: true}}
	scorecard := RunEvalFixtures(ctx, false)
	if scorecard.Failed == 0 {
		t.Fatalf("expected failures for missing named app: %#v", scorecard)
	}
	found := false
	for _, result := range scorecard.Results {
		if result.Name == "add named app route" && containsPart(result.Failures, "plan returned") {
			found = true
		}
	}
	if !found {
		t.Fatalf("expected actionable named app failure: %#v", scorecard.Results)
	}
}

func TestRegressionFixturesCaptureKnownFailures(t *testing.T) {
	fixtures := RegressionFixtures()
	if len(fixtures) == 0 {
		t.Fatal("expected regression fixtures")
	}
	result, ok := Plan(fixtureContext(), PlanRequest{Task: fixtures[0].Task, App: fixtures[0].App})
	if !ok {
		t.Fatal("expected plan")
	}
	if result.WorkflowID != fixtures[0].WantWorkflowID {
		t.Fatalf("workflow = %q, want %q", result.WorkflowID, fixtures[0].WantWorkflowID)
	}
}

func TestClassifyWorkflowTypes(t *testing.T) {
	cases := map[string]string{
		"fix wire missing provider":            "goforj-wire-repair",
		"add users HTTP route with repository": "goforj-add-http-route",
		"add daily queue job schedule":         "goforj-add-job-schedule",
		"add reports daily schedule":           "goforj-add-schedule",
		"add background queue job":             "goforj-add-job",
		"publish user event":                   "goforj-add-event-workflow",
		"add repository and migration":         "goforj-add-data-resource",
		"add operator command":                 "goforj-add-app-command",
		"debug runtime logs":                   "goforj-debug-runtime",
	}
	for task, want := range cases {
		if got := Classify(task); got != want {
			t.Fatalf("Classify(%q) = %q, want %q", task, got, want)
		}
	}
}

func TestWorkflowDocsMapCoversAllWorkflowIDs(t *testing.T) {
	for _, id := range []string{
		"goforj-add-http-route",
		"goforj-add-app-command",
		"goforj-add-job",
		"goforj-add-job-schedule",
		"goforj-add-schedule",
		"goforj-add-event-workflow",
		"goforj-add-data-resource",
		"goforj-wire-repair",
		"goforj-debug-runtime",
		"goforj-multi-app-change",
		"goforj-validate-change",
	} {
		if len(WorkflowDocsMap[id]) == 0 {
			t.Fatalf("missing docs map for %s", id)
		}
	}
}

func TestRegistrationPointsNamedApp(t *testing.T) {
	result, ok := RegistrationPoints(fixtureContext().Project, "marketplace")
	if !ok {
		t.Fatal("expected registration points")
	}
	if result.App != "marketplace" {
		t.Fatalf("app = %q", result.App)
	}
	if result.Points[0].Files[0] != "app/marketplace/routes.go" {
		t.Fatalf("points = %#v", result.Points)
	}
}

func TestValidationPlanIncludesCacheEnv(t *testing.T) {
	result := ValidationPlan(fixtureContext(), PlanRequest{Task: "add route"})
	if len(result.Steps) == 0 || !strings.Contains(result.Steps[0].Command, "GOCACHE=/tmp/gocache") {
		t.Fatalf("steps = %#v", result.Steps)
	}
}

func TestDiagnoseWireMissingProvider(t *testing.T) {
	diagnostics := DiagnoseWire("wire: no provider found for *users.Service\nneeded by *users.Controller")
	if diagnostics[0].Category != "missing-provider" {
		t.Fatalf("diagnostics = %#v", diagnostics)
	}
	if diagnostics[0].MissingType != "*users.Service" || diagnostics[0].Consumer != "*users.Controller" || diagnostics[0].ProviderSet == "" {
		t.Fatalf("diagnostics = %#v", diagnostics)
	}
}

func TestFilePolicyClassifiesGeneratedAndAppOwnedPaths(t *testing.T) {
	ctx := fixtureContext()
	cases := []struct {
		path           string
		classification string
		editable       bool
		action         string
		app            string
		frontendKit    string
	}{
		{path: "app/wire/wire_gen.go", classification: "generated", editable: false, action: "regenerate", app: "app"},
		{path: "app/routes.go", classification: "app-owned-registration", editable: true, action: "prefer forj make:*", app: "app"},
		{path: "app/marketplace/routes.go", classification: "named-app-owned-registration", editable: true, action: "prefer forj make:*", app: "marketplace"},
		{path: "app/marketplace/wire/inject_http_controllers_app.go", classification: "named-app-owned-wire", editable: true, action: "direct edit provider set", app: "marketplace"},
		{path: "cmd/app/frontend/src/App.vue", classification: "frontend-owned", editable: true, action: "direct edit", app: "app", frontendKit: "vue"},
		{path: "cmd/app/main.go", classification: "framework-owned-entrypoint", editable: false, action: "do not edit", app: "app"},
		{path: "internal/runtime/server.go", classification: "framework-owned-runtime", editable: false, action: "do not edit"},
		{path: "internal/users/service.go", classification: "user-owned-domain", editable: true, action: "direct edit"},
		{path: "migrations/app/default/001_create_users.sql", classification: "migration-owned", editable: true, action: "prefer forj make:migration"},
		{path: ".goforj.yml", classification: "config-owned", editable: true, action: "direct edit"},
		{path: "docs/runbook.md", classification: "user-owned-docs", editable: true, action: "direct edit"},
		{path: "README.md", classification: "user-owned", editable: true, action: "direct edit"},
	}
	for _, tc := range cases {
		policy := FilePolicy(FilePolicyRequest{Path: tc.path, Project: ctx.Project})
		if policy.Classification != tc.classification || policy.Editable != tc.editable || policy.PreferredAction != tc.action {
			t.Fatalf("%s policy = %#v", tc.path, policy)
		}
		if tc.app != "" && policy.App != tc.app {
			t.Fatalf("%s app = %q, want %q in %#v", tc.path, policy.App, tc.app, policy)
		}
		if tc.frontendKit != "" && policy.FrontendKit != tc.frontendKit {
			t.Fatalf("%s frontend kit = %q, want %q in %#v", tc.path, policy.FrontendKit, tc.frontendKit, policy)
		}
		if policy.Reason == "" {
			t.Fatalf("%s missing reason: %#v", tc.path, policy)
		}
	}
}

func TestFilePolicyUsesProjectOwnershipOverrides(t *testing.T) {
	policy := FilePolicy(FilePolicyRequest{
		Path: "docs/generated/api.md",
		Rules: []OwnershipRule{{
			Pattern:         "docs/generated/**",
			Classification:  "generated-docs",
			Editable:        false,
			PreferredAction: "regenerate",
			ChangeThrough:   "forj docs:generate",
			Reason:          "Generated API docs are owned by the docs generator.",
		}},
	})
	if policy.Classification != "generated-docs" || policy.Editable || policy.PreferredAction != "regenerate" ||
		policy.MatchedRule != "docs/generated/**" || policy.ChangeThrough != "forj docs:generate" {
		t.Fatalf("policy = %#v", policy)
	}
}

func TestPlanIncludesOwnershipPolicyForPlannedFiles(t *testing.T) {
	result, ok := Plan(fixtureContext(), PlanRequest{Task: "fix wire missing provider"})
	if !ok {
		t.Fatal("expected plan")
	}
	if len(result.Ownership) == 0 {
		t.Fatalf("missing ownership in %#v", result)
	}
	if containsPart(result.Files, "wire_gen.go") {
		t.Fatalf("plan should avoid generated Wire output: %#v", result.Files)
	}
	for _, policy := range result.Ownership {
		if policy.Path == "" || policy.PreferredAction == "" || policy.Reason == "" {
			t.Fatalf("incomplete policy = %#v", policy)
		}
	}
}

func TestCommandAdviceNamedApp(t *testing.T) {
	result, ok := CommandAdvice(fixtureContext(), CommandAdviceRequest{Task: "add queue job", App: "marketplace", Resource: "sync-catalog"})
	if !ok {
		t.Fatal("expected command advice")
	}
	if result.Command != "forj marketplace make:job sync-catalog" {
		t.Fatalf("command = %q", result.Command)
	}
}

func TestDocsSectionPackReadsWorkflowDocs(t *testing.T) {
	result, err := DocsSectionPack(context.Background(), atlasdocs.StaticProvider{
		Docs: []atlasdocs.Document{{
			Path:    "applications/routes.md",
			Title:   "Routes",
			Content: "# Routes\n\n## Where Routes Live\n\nRoutes live in app routes.",
		}},
	}, "goforj-add-http-route", "", 20)
	if err != nil {
		t.Fatalf("docs section pack: %v", err)
	}
	if len(result.Sections) == 0 || !result.Sections[0].Found {
		t.Fatalf("result = %#v", result)
	}
}

func TestVersionAlignmentWarnings(t *testing.T) {
	project := fixtureContext().Project
	project.GoForjVersion = "0.18.0"
	aligned := VersionAlignment(project, "atlas-test", atlasdocs.Manifest{Version: "0.18.0", Ref: "v0.18.0", Revision: "rev1", GoForjVersion: "0.18.0"})
	if !aligned.Aligned || len(aligned.Warnings) != 0 {
		t.Fatalf("aligned = %#v", aligned)
	}
	docsAhead := VersionAlignment(project, "atlas-test", atlasdocs.Manifest{Version: "0.19.0", Ref: "main", Revision: "rev2", GoForjVersion: "0.19.0"})
	if docsAhead.Aligned || len(docsAhead.Warnings) < 2 {
		t.Fatalf("docs ahead = %#v", docsAhead)
	}
	cliMismatch := VersionAlignment(project, "0.17.0", atlasdocs.Manifest{Version: "0.18.0", Ref: "v0.18.0", Revision: "rev1", GoForjVersion: "0.18.0"})
	if cliMismatch.Aligned || !containsVersionWarning(cliMismatch.Warnings, "cli-version-mismatch") {
		t.Fatalf("cli mismatch = %#v", cliMismatch)
	}
	unknown := VersionAlignment(project, "atlas-test", atlasdocs.Manifest{Version: "0.18.0"})
	if unknown.Aligned || !containsVersionWarning(unknown.Warnings, "docs-ref-unknown") {
		t.Fatalf("unknown = %#v", unknown)
	}
}

func TestScenarioGuideUsesDocsProvider(t *testing.T) {
	result, err := ScenarioGuide(context.Background(), atlasdocs.StaticProvider{
		Docs: []atlasdocs.Document{{
			Path:    "scenarios/reports-generate-job.md",
			Title:   "Reports Generate Job",
			Content: "# Reports Generate Job\n\n## What You Will Build\n\nA queued job scenario.",
		}},
	}, "job")
	if err != nil {
		t.Fatalf("scenario guide: %v", err)
	}
	if len(result.Scenarios) == 0 || result.Scenarios[0].Path != "scenarios/reports-generate-job.md" {
		t.Fatalf("result = %#v", result)
	}
}

func TestPhotoDropPlanComposesOrderedWorkflows(t *testing.T) {
	result, ok := Plan(fixtureContext(), PlanRequest{Task: "Build PhotoDrop with a photos database table and repository, upload API and gallery UI, thumbnail queue job, photo-created event subscriber, expired-share schedule, and operator cleanup command."})
	if !ok {
		t.Fatal("expected PhotoDrop plan")
	}
	for _, workflowID := range []string{"goforj-add-data-resource", "goforj-add-http-route", "goforj-add-job", "goforj-add-event-workflow", "goforj-add-schedule", "goforj-add-app-command", "goforj-frontend-change"} {
		if !contains(result.WorkflowIDs, workflowID) {
			t.Fatalf("workflow ids = %#v, want %q", result.WorkflowIDs, workflowID)
		}
	}
	migration := commandIndex(result.Commands, "make:migration")
	migrate := commandIndex(result.Commands, "forj migrate")
	model := commandIndex(result.Commands, "make:model")
	if migration < 0 || migrate <= migration || model <= migrate {
		t.Fatalf("data command order = %#v", result.Commands)
	}
}

func TestCompositionalPlanningDoesNotDependOnPhotoDropVocabulary(t *testing.T) {
	result, ok := Plan(fixtureContext(), PlanRequest{Task: "Build a library catalog with a database schema and repository, HTTP API controller, background indexing job, book-added event subscriber, recurring maintenance schedule, operator CLI command, and frontend page."})
	if !ok {
		t.Fatal("expected library plan")
	}
	for _, workflowID := range []string{"goforj-add-data-resource", "goforj-add-http-route", "goforj-add-job", "goforj-add-event-workflow", "goforj-add-schedule", "goforj-add-app-command", "goforj-frontend-change"} {
		if !contains(result.WorkflowIDs, workflowID) {
			t.Fatalf("workflow ids = %#v, want %q", result.WorkflowIDs, workflowID)
		}
	}
}

func TestFilePolicyRequiresGeneratorForPersistenceCreation(t *testing.T) {
	policy := FilePolicy(FilePolicyRequest{
		Path:        "internal/photos/repository.go",
		Task:        "create the photos model and repository",
		WorkflowIDs: []string{"goforj-add-data-resource"},
	})
	if policy.Classification != "generator-first-data" || policy.PreferredAction != "generate then extend" {
		t.Fatalf("policy = %#v", policy)
	}
	if !strings.Contains(policy.ChangeThrough, "forj make:model photos --package photos") {
		t.Fatalf("change through = %q", policy.ChangeThrough)
	}

	extension := FilePolicy(FilePolicyRequest{Path: "internal/photos/repository.go", Task: "add FindPublished to the existing repository"})
	if extension.Classification != "user-owned-domain" || extension.PreferredAction != "direct edit" {
		t.Fatalf("extension policy = %#v", extension)
	}
}

func TestValidationPlanWarnsAboutPersistenceGeneratorBypass(t *testing.T) {
	result := ValidationPlan(fixtureContext(), PlanRequest{Task: "create a photos table, model, and repository"})
	if !containsPart(result.Warnings, "forj make:model") {
		t.Fatalf("warnings = %#v", result.Warnings)
	}
	if !validationContainsPart(result.Steps, "gofmt -l") {
		t.Fatalf("steps = %#v", result.Steps)
	}
}

// commandIndex locates one expected action without coupling ordering tests to complete command strings.
func commandIndex(commands []string, part string) int {
	for index, command := range commands {
		if strings.Contains(command, part) {
			return index
		}
	}
	return -1
}

func containsVersionWarning(warnings []VersionWarning, code string) bool {
	for _, warning := range warnings {
		if warning.Code == code {
			return true
		}
	}
	return false
}

func TestResourcesIncludesConnections(t *testing.T) {
	result := Resources(fixtureContext())
	if !contains(result.Categories, "databases") {
		t.Fatalf("categories = %#v", result.Categories)
	}
	if !contains(result.Categories, "queues") || !contains(result.Categories, "caches") || !contains(result.Categories, "disks") || !contains(result.Categories, "event_buses") {
		t.Fatalf("categories = %#v", result.Categories)
	}
	if len(result.Databases) != 1 {
		t.Fatalf("databases = %#v", result.Databases)
	}
	if result.FrontendKit != "vue" {
		t.Fatalf("frontend kit = %q", result.FrontendKit)
	}
}

func TestPlanIncludesProjectOwnedOverlays(t *testing.T) {
	ctx := fixtureContext()
	ctx.Overlays = []string{"checkout-rules"}
	result, ok := Plan(ctx, PlanRequest{Task: "change checkout rules"})
	if !ok {
		t.Fatal("expected plan")
	}
	if !contains(result.Overlays, "checkout-rules") {
		t.Fatalf("overlays = %#v", result.Overlays)
	}
}

func TestPlanIncludesStarterKitOverlayForFrontendWork(t *testing.T) {
	ctx := fixtureContext()
	result, ok := Plan(ctx, PlanRequest{Task: "add dashboard page"})
	if !ok {
		t.Fatal("expected plan")
	}
	if !contains(result.Overlays, "goforj-vue-starter-kit") {
		t.Fatalf("overlays = %#v", result.Overlays)
	}

	ctx.Project.FrontendKit = "react"
	result, ok = Plan(ctx, PlanRequest{Task: "update login screen"})
	if !ok {
		t.Fatal("expected plan")
	}
	if !contains(result.Overlays, "goforj-react-starter-kit") {
		t.Fatalf("overlays = %#v", result.Overlays)
	}
}

func fixtureContext() Context {
	return Context{
		Project: project.Project{
			Name:        "demo",
			Components:  []string{"web-api", "jobs", "scheduler"},
			FrontendKit: "vue",
			Apps: []project.App{
				{Name: "app", Default: true},
				{Name: "marketplace"},
			},
		},
		Inventory: Inventory{
			Routes:     map[string][]string{"app": []string{"GET /users"}},
			Commands:   map[string][]string{"app": []string{"reports:sync"}},
			Schedules:  map[string][]string{"app": []string{"reports:daily"}},
			Queues:     []string{"reports"},
			Caches:     []string{"profiles"},
			Disks:      []string{"uploads"},
			EventBuses: []string{"default"},
		},
		Connections: []diagnostics.DatabaseConnection{{Name: "default", Driver: "sqlite"}},
	}
}

func containsPart(values []string, want string) bool {
	for _, value := range values {
		if strings.Contains(value, want) {
			return true
		}
	}
	return false
}

func docsContain(values []DocReference, path string) bool {
	for _, value := range values {
		if value.Path == path {
			return true
		}
	}
	return false
}

func contains(values []string, want string) bool {
	for _, value := range values {
		if value == want {
			return true
		}
	}
	return false
}
