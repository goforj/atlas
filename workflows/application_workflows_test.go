package workflows

import (
	"context"
	"slices"
	"strings"
	"testing"

	"github.com/goforj/atlas/docs"
)

// applicationPlanningCases fixes natural language and docs expectations independently from registered metadata.
func applicationPlanningCases() map[string]struct{ task, topic, path string } {
	return map[string]struct{ task, topic, path string }{
		"goforj-request-validation":    {"Update existing PATCH settings with reusable request validation and distinguish omitted fields, null, zero and false.", "request validation", "applications/requests-validation.md"},
		"goforj-configure-environment": {"Configure typed startup configuration with a provider, required environment variables and worker count.", "environment configuration", "reference/configuration.md"},
		"goforj-encrypt-payloads":      {"Encrypt existing partner payloads and retain previous keys for key rotation.", "payload encryption", "libraries/crypt.md"},
		"goforj-read-bound-secrets":    {"Read a named credential through a bound secret.Reader provider.", "bound secrets", "libraries/secret.md"},
	}
}

// applicationPlanningDocs supplies actual public paths and heading spellings rather than mirroring the workflow map.
func applicationPlanningDocs() docs.StaticProvider {
	return docs.StaticProvider{Docs: []docs.Document{
		{Path: "applications/requests-validation.md", Content: "# Requests and Validation\n\n## Bind, Normalize, and Validate\n\nPreserve omitted and null values through reusable request rules."},
		{Path: "reference/configuration.md", Content: "# Configuration Reference\n\n## Environment File Resolution\n\nProcess configuration overrides files before typed startup validation."},
		{Path: "libraries/crypt.md", Content: "# Crypt Library for Go {#crypt-library-for-go}\n\n## Key format & rotation {#key-format-&-rotation}\n\nCurrent keys encrypt while previous keys retain authenticated reads."},
		{Path: "libraries/secret.md", Content: "# Secret Library for Go {#secret-library-for-go}\n\n## Quick Start {#quick-start}\n\nInject a bound Reader and preserve classified lookup failures."},
		{Path: "applications/library-selection.md", Content: "# Choosing Libraries\n\n## Match the Application Requirement\n\nChoose the supported library for application policy.\n\n## Preserve Input Meaning\n\nKeep presence and field errors separate from operational failure."},
		{Path: "developer-tools/wiring-recipes.md", Content: "# Wiring Recipes\n\n## Quick Map\n\nRegister application constructors in the selected service provider set."},
		{Path: "developer-tools/reading-wire-errors.md", Content: "# Reading Wire Errors\n\n## Fast Checklist\n\nInspect the provider chain before repairing required dependencies."},
	}}
}

// TestApplicationPlansUseProvidersWithoutInventingGenerators covers default and named-App composition.
func TestApplicationPlansUseProvidersWithoutInventingGenerators(t *testing.T) {
	for id, test := range applicationPlanningCases() {
		if _, registered := applicationWorkflowDefinitions[id]; !registered {
			continue
		}
		t.Run(id, func(t *testing.T) {
			for _, app := range []string{"app", "marketplace"} {
				plan, ok := Plan(fixtureContext(), PlanRequest{Task: test.task, App: app})
				if !ok {
					t.Fatal("missing selected App plan")
				}
				if plan.WorkflowID != id || !slices.Equal(plan.WorkflowIDs, []string{id}) {
					t.Fatalf("workflows = %q/%v", plan.WorkflowID, plan.WorkflowIDs)
				}
				prefix := "forj "
				wire := "app/wire/inject_services_app.go"
				if app == "marketplace" {
					prefix = "forj marketplace "
					wire = "app/marketplace/wire/inject_services_app.go"
				}
				if !slices.Contains(plan.Commands, prefix+"build") || !slices.Contains(plan.Commands, "go test ./...") || !slices.Contains(plan.Files, wire) {
					t.Fatalf("commands/files = %v/%v", plan.Commands, plan.Files)
				}

				if slices.Index(plan.Commands, prefix+"build") > slices.Index(plan.Commands, "go test ./...") {
					t.Fatal("tests precede provider graph regeneration")
				}
				buildOrder, testOrder := 0, 0
				for _, step := range plan.Steps {
					if step.Action == prefix+"build" {
						buildOrder = step.Order
					}
					if step.Action == "go test ./..." {
						testOrder = step.Order
					}
				}
				if buildOrder == 0 || testOrder <= buildOrder {
					t.Fatal("planned steps test stale provider wiring")
				}
				verification := plan.Verification
				if len(verification) < 3 || verification[1].Command != prefix+"build" || !strings.Contains(verification[2].Command, "go test ./...") {
					t.Fatalf("provider verification order = %#v", verification)
				}
				for _, command := range plan.Commands {
					if containsAny(command, "make:", " migrate", " worker") {
						t.Fatalf("unrequested generator or runtime command %q", command)
					}
				}
				if len(plan.Steps) < 3 || !docsContain(plan.Docs, test.path) || docsContain(plan.Docs, "applications/library-resources.md") {
					t.Fatalf("steps/docs = %#v/%#v", plan.Steps, plan.Docs)
				}
				advice, ok := CommandAdvice(fixtureContext(), CommandAdviceRequest{Task: test.task, App: app})
				if !ok || advice.Command != prefix+"build" {
					t.Fatalf("advice = %#v", advice)
				}
				plan.Steps[0].Action = "changed by caller"
				again, _ := Plan(fixtureContext(), PlanRequest{Task: test.task, App: app})
				if again.Steps[0].Action == "changed by caller" {
					t.Fatal("plan mutated shared workflow metadata")
				}
				pack, err := DocsSectionPack(context.Background(), applicationPlanningDocs(), "", test.task, 40)
				if err != nil || pack.WorkflowID != id || len(pack.Sections) < 2 || pack.Sections[0].Reference.Path != test.path {
					t.Fatalf("docs pack = %#v, error = %v", pack, err)
				}
				for _, section := range pack.Sections {
					if !section.Found {
						t.Fatalf("missing real docs path/heading %#v", section.Reference)
					}
				}
			}
		})
	}
}

// TestApplicationCompositionAddsOnlyExplicitNewSurfaces prevents worker counts and existing controllers from creating artifacts.
func TestApplicationCompositionAddsOnlyExplicitNewSurfaces(t *testing.T) {
	for id, test := range applicationPlanningCases() {
		if _, registered := applicationWorkflowDefinitions[id]; !registered {
			continue
		}
		for _, surface := range []struct{ suffix, id, command string }{
			{" and create a route", "goforj-add-http-route", "make:controller"},
			{" and create an endpoint", "goforj-add-http-route", "make:controller"},
			{" and create a controller", "goforj-add-http-route", "make:controller"},
			{" and create a queue job", "goforj-add-job", "make:job"},
			{" and create a schedule", "goforj-add-schedule", "make:schedule"},
			{" and create a command", "goforj-add-app-command", "make:command"},
		} {
			plan, ok := Plan(fixtureContext(), PlanRequest{Task: test.task + surface.suffix, App: "marketplace"})
			if !ok || !slices.Contains(plan.WorkflowIDs, id) || !slices.Contains(plan.WorkflowIDs, surface.id) || !containsPart(plan.Commands, "forj marketplace "+surface.command) {
				t.Fatalf("explicit surface %q plan = %#v", surface.suffix, plan)
			}
		}
		plan, _ := Plan(fixtureContext(), PlanRequest{Task: test.task + " and add settings to an existing controller while configuring worker count 4"})
		if !slices.Equal(plan.WorkflowIDs, []string{id}) {
			t.Fatalf("incidental wording created workflows: %v", plan.WorkflowIDs)
		}
	}
}

// TestPlanningInspectionRequestsRemainReadOnly preserves test, debug, documentation, and genuine repair intent.
func TestPlanningInspectionRequestsRemainReadOnly(t *testing.T) {
	for _, test := range []struct{ task, want string }{
		{"write tests for request validation", "goforj-validate-change"}, {"add regression tests for payload encryption", "goforj-validate-change"}, {"create a test for bound secret reads", "goforj-validate-change"}, {"go test validation", "goforj-validate-change"}, {"run tests for encrypted payloads", "goforj-validate-change"}, {"verify secret lookups", "goforj-validate-change"}, {"review environment configuration", "goforj-validate-change"},
		{"debug validation errors", "goforj-debug-runtime"}, {"diagnose secret provider failures", "goforj-debug-runtime"}, {"troubleshoot startup configuration", "goforj-debug-runtime"}, {"investigate encryption failures", "goforj-debug-runtime"},
		{"fix missing provider for crypt.Cipher", "goforj-wire-repair"}, {"fix duplicate provider for secret.Reader", "goforj-wire-repair"},
	} {
		if got := Classify(test.task); got != test.want {
			t.Errorf("Classify(%q) = %q, want %q", test.task, got, test.want)
		}
		for _, id := range ClassifyAll(test.task) {
			if _, implementation := applicationWorkflowDefinitions[id]; implementation {
				t.Errorf("inspection %q became implementation %q", test.task, id)
			}
		}
	}
	for _, task := range []string{"please search docs for Wire", "can you read documentation for Wire", "could you explain Wire providers", "search docs for Wire", "read documentation for Wire", "find docs for Wire", "show docs for Wire", "browse docs for Wire", "explain Wire providers", "look up Wire documentation"} {
		plan, ok := Plan(fixtureContext(), PlanRequest{Task: task, App: "marketplace"})
		if !ok || plan.WorkflowID != "goforj-read-docs" || len(plan.Commands) != 0 || len(plan.Files) != 0 || len(plan.Verification) != 0 || !docsContain(plan.Docs, "developer-tools/reading-wire-errors.md") {
			t.Fatalf("docs request created implementation work: %#v", plan)
		}
		if result := ValidationPlan(fixtureContext(), PlanRequest{Task: task}); len(result.Steps) != 0 {
			t.Fatalf("docs query got build/test work: %#v", result)
		}
		advice, ok := CommandAdvice(fixtureContext(), CommandAdviceRequest{Task: task})
		if !ok || advice.Command != "Atlas search-docs" {
			t.Fatalf("docs advice = %#v", advice)
		}
	}
	for id, test := range applicationPlanningCases() {
		if _, registered := applicationWorkflowDefinitions[id]; !registered {
			continue
		}
		task := "search docs for " + test.topic
		pack, err := DocsSectionPack(context.Background(), applicationPlanningDocs(), "", task, 20)
		if err != nil || pack.WorkflowID != "goforj-read-docs" || len(pack.Sections) == 0 || pack.Sections[0].Reference.Path != test.path {
			t.Fatalf("topic pack = %#v, error = %v", pack, err)
		}
		for _, section := range pack.Sections {
			if !section.Found {
				t.Fatal("topic query returned a missing section")
			}
		}
	}
	if got := nonImplementationWorkflow("add existing service behavior"); got != "" {
		t.Fatal("ordinary implementation was intercepted")
	}
	if ids := applicationWorkflowIDs("configuration encryption validation secrets", true); len(ids) != 0 {
		t.Fatalf("noun-only query became implementation: %v", ids)
	}
	plan, _ := Plan(fixtureContext(), PlanRequest{Task: "search docs for an unknown topic"})
	if !docsContain(plan.Docs, "applications/library-selection.md") || strings.Contains(strings.Join(plan.Commands, " "), "make:") {
		t.Fatal("unknown docs topic invented a generator")
	}
}

// TestApplicationCommandTemplatesPreserveDeclaredGenerators checks named-App interpolation without keyword-derived scaffolding.
func TestApplicationCommandTemplatesPreserveDeclaredGenerators(t *testing.T) {
	scope := makeScope(fixtureContext().Project.Apps[1])
	commands := applicationCommandTemplates([]string{"{forj} generate --notification", "{build}", "go test ./..."}, scope)
	if !slices.Equal(commands, []string{"forj marketplace generate --notification", "forj marketplace build", "go test ./..."}) {
		t.Fatalf("declared commands = %v", commands)
	}
}

// TestSharedPostBuildVerificationRunsOnce preserves one test pass after a composed provider build.
func TestSharedPostBuildVerificationRunsOnce(t *testing.T) {
	steps := appendUniquePostBuildStep(nil, PlanStep{Action: "go test ./..."})
	steps = appendUniquePostBuildStep(steps, PlanStep{Action: "go test ./..."})
	steps = appendUniquePostBuildStep(steps, PlanStep{Action: "forj marketplace route:list"})
	if len(steps) != 2 || steps[0].Action != "go test ./..." || steps[1].Action != "forj marketplace route:list" {
		t.Fatalf("post-build steps = %v", steps)
	}
}
