package workflows

import (
	"context"
	"slices"
	"strings"
	"testing"

	"github.com/goforj/atlas/docs"
)

// TestOutboundHTTPAPIClientAvoidsControllerScaffolding covers the HTTP/API words missing from the original regression.
func TestOutboundHTTPAPIClientAvoidsControllerScaffolding(t *testing.T) {
	task := "harden outbound HTTP client API retries"
	plan, ok := Plan(fixtureContext(), PlanRequest{Task: task, App: "marketplace"})
	if !ok || plan.WorkflowID != "goforj-http-client-resilience" || !slices.Equal(plan.WorkflowIDs, []string{"goforj-http-client-resilience"}) {
		t.Fatalf("workflows = %#v", plan)
	}
	if !slices.Contains(plan.Commands, "forj marketplace build") || !slices.Contains(plan.Commands, "go test ./...") || !slices.Contains(plan.Files, "app/marketplace/wire/inject_services_app.go") {
		t.Fatalf("commands/files = %v/%v", plan.Commands, plan.Files)
	}
	if strings.Contains(strings.Join(plan.Commands, " "), "make:controller") || containsPart(plan.Files, "controller.go") || slices.Contains(plan.WorkflowIDs, "goforj-add-http-route") {
		t.Fatal("pure client work created a controller")
	}
	for _, task := range []string{"send outbound mail to a customer", "configure a remote SMTP client", "go test outbound HTTP clients", "debug outbound HTTP errors"} {
		if slices.Contains(applicationWorkflowIDs(strings.ToLower(task), true), "goforj-http-client-resilience") {
			t.Fatalf("non-implementation request %q became HTTP client work", task)
		}
	}
	composed, _ := Plan(fixtureContext(), PlanRequest{Task: task + " and create an HTTP endpoint", App: "marketplace"})
	if !slices.Contains(composed.WorkflowIDs, "goforj-add-http-route") || !containsPart(composed.Commands, "forj marketplace make:controller") {
		t.Fatal("explicit endpoint was omitted")
	}
}

// TestHTTPClientDocsLookupReadsCanonicalSections keeps docs discovery read-only and validates real heading spellings.
func TestHTTPClientDocsLookupReadsCanonicalSections(t *testing.T) {
	task := "search docs for outbound HTTP client API retries"
	plan, ok := Plan(fixtureContext(), PlanRequest{Task: task})
	if !ok || plan.WorkflowID != "goforj-read-docs" || len(plan.Commands) != 0 || len(plan.Files) != 0 || len(plan.Verification) != 0 {
		t.Fatalf("docs query generated source work: %#v", plan)
	}
	provider := docs.StaticProvider{Docs: []docs.Document{
		{Path: "libraries/httpx.md", Content: "# HTTPX Library for Go {#httpx-library-for-go}\n\n## Installation {#installation}\n\nPreserve the installed version.\n\n## Retry {#retry-2}\n\nUse intentional bounded retries."},
		{Path: "applications/library-selection.md", Content: "# Choosing Libraries\n\n## Match the Application Requirement\n\nSelect the actual application boundary."},
	}}
	for _, request := range []struct{ id, task string }{{"", task}, {"goforj-http-client-resilience", ""}} {
		pack, err := DocsSectionPack(context.Background(), provider, request.id, request.task, 30)
		if err != nil || len(pack.Sections) != 3 || pack.Sections[0].Reference.Path != "libraries/httpx.md" {
			t.Fatalf("pack = %#v, error = %v", pack, err)
		}
		for _, section := range pack.Sections {
			if !section.Found {
				t.Fatalf("missing canonical section %#v", section.Reference)
			}
		}
	}
}
