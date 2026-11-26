package workflows

import (
	"context"
	"slices"
	"strings"
	"testing"

	"github.com/goforj/atlas/docs"
)

// TestCatalogSearchPlanUsesAppResources verifies discovery without a preinstalled dependency and App-aware generation.
func TestCatalogSearchPlanUsesAppResources(t *testing.T) {
	for _, app := range []string{"app", "marketplace"} {
		ctx := fixtureContext()
		ctx.Project.Components = append(ctx.Project.Components, "search")
		result, ok := Plan(ctx, PlanRequest{Task: "make product catalog descriptions searchable with category filtering and stable pagination", App: app})
		if !ok || result.WorkflowID != "goforj-catalog-search" {
			t.Fatalf("plan: %#v", result)
		}
		command := "forj generate --search"
		schema := "app/search.go"
		if app != "app" {
			command = "forj " + app + " generate --search"
			schema = "app/" + app + "/search.go"
		}
		if !slices.Contains(result.Commands, command) || !slices.Contains(result.Files, schema) || !slices.Contains(result.Files, "internal/searches/accessors_gen.go") {
			t.Fatalf("resource plan: %#v", result)
		}
		for _, cmd := range result.Commands {
			if strings.Contains(cmd, "make:") {
				t.Fatalf("invented generator: %s", cmd)
			}
		}
		if !docsContain(result.Docs, "applications/library-resources.md") {
			t.Fatal("missing resource composition docs")
		}
		advice, ok := CommandAdvice(ctx, CommandAdviceRequest{Task: "add a lexical search resource", App: app})
		if !ok || advice.Command != command || strings.Contains(advice.Reason, "make commands") {
			t.Fatalf("command advice: %#v", advice)
		}
	}
}

// TestCatalogSearchRoutingComposesHTTPAndRejectsLookupCues prevents ordinary navigation from becoming index creation.
func TestCatalogSearchRoutingComposesHTTPAndRejectsLookupCues(t *testing.T) {
	endpoint, ok := Plan(fixtureContext(), PlanRequest{Task: "add a catalog search endpoint with category filters"})
	if !ok || endpoint.WorkflowID != "goforj-add-http-route" || !slices.Contains(endpoint.WorkflowIDs, "goforj-catalog-search") {
		t.Fatalf("endpoint plan: %#v", endpoint)
	}
	for _, task := range []string{"search docs for a Wire error", "search source files for catalog filters", "research product catalog options"} {
		if slices.Contains(ClassifyAll(task), "goforj-catalog-search") || Classify(task) == "goforj-catalog-search" {
			t.Fatalf("lookup became index creation: %s", task)
		}
	}
	resource := ClassifyAll("configure a Meilisearch backend endpoint for the search index")
	if !slices.Contains(resource, "goforj-catalog-search") || slices.Contains(resource, "goforj-add-http-route") {
		t.Fatalf("backend endpoint created a controller: %#v", resource)
	}
	schema := ClassifyAll("declare the catalog search schema and resource")
	if !slices.Contains(schema, "goforj-catalog-search") || slices.Contains(schema, "goforj-add-data-resource") {
		t.Fatalf("index schema created a SQL resource: %#v", schema)
	}
}

// TestCatalogSearchDocsPackUsesResourceGuide verifies the MCP docs-pack surface chooses the new workflow sections.
func TestCatalogSearchDocsPackUsesResourceGuide(t *testing.T) {
	provider := docs.StaticProvider{Docs: []docs.Document{
		{Path: "applications/library-resources.md", Content: "# Resources\n\n## Declare and Provision a Search Index\n\nDeclare an App schema and provision explicitly.\n"},
		{Path: "libraries/search.md", Content: "# Search\n\n## Quick Start {#quick-start}\n\nLexical search uses typed filters.\n"},
	}}
	pack, err := DocsSectionPack(context.Background(), provider, "", "add lexical search for a product catalog", 300)
	if err != nil || pack.WorkflowID != "goforj-catalog-search" || len(pack.Sections) != 2 {
		t.Fatalf("docs pack: %#v, %v", pack, err)
	}
	for _, section := range pack.Sections {
		if !section.Found {
			t.Fatalf("missing docs section: %#v", section.Reference)
		}
	}
}

// TestCatalogSearchUnknownSupportInspectsBeforeGeneration preserves older CLI compatibility without guessing a version gate.
func TestCatalogSearchUnknownSupportInspectsBeforeGeneration(t *testing.T) {
	ctx := fixtureContext()
	result, ok := Plan(ctx, PlanRequest{Task: "add a catalog search resource", App: "marketplace"})
	if !ok || !slices.Contains(result.Commands, "forj marketplace generate --help") || slices.Contains(result.Commands, "forj marketplace generate --search") {
		t.Fatalf("unsupported advice: %#v", result)
	}
	if slices.Contains(result.Files, "app/marketplace/search.go") || len(result.Warnings) == 0 || result.Warnings[0] != searchCapabilityWarning {
		t.Fatalf("structure planned before capability inspection: %#v", result)
	}
	if result.Steps[0].Action != "forj marketplace generate --help" {
		t.Fatalf("first action: %#v", result.Steps[0])
	}
	advice, ok := CommandAdvice(ctx, CommandAdviceRequest{Task: "add catalog search", App: "marketplace"})
	if !ok || advice.Command != "forj marketplace generate --help" || advice.Reason != searchCapabilityWarning {
		t.Fatalf("unsupported command advice: %#v", advice)
	}
}

// TestCatalogSearchOwnershipPreservesSchemaAndGeneratedBoundaries keeps agents from patching derived managers.
func TestCatalogSearchOwnershipPreservesSchemaAndGeneratedBoundaries(t *testing.T) {
	for _, path := range []string{"internal/searches/manager_gen.go", "internal/searches/accessors_gen.go"} {
		policy := FilePolicy(FilePolicyRequest{Path: path})
		if policy.Editable || policy.ChangeThrough != "forj generate --search" {
			t.Fatalf("generated ownership: %#v", policy)
		}
	}
	for _, path := range []string{"app/search.go", "app/marketplace/search.go"} {
		policy := FilePolicy(FilePolicyRequest{Path: path})
		if !policy.Editable || policy.Owner != "app" || !strings.Contains(policy.ChangeThrough, "SearchDefinitions()") {
			t.Fatalf("schema ownership: %#v", policy)
		}
	}
}

// TestCatalogSearchComposesRegisteredConfiguration keeps independent policy work visible before generic heuristics.
func TestCatalogSearchComposesRegisteredConfiguration(t *testing.T) {
	const id = "goforj-test-environment"
	applicationWorkflowDefinitions[id] = applicationWorkflowDefinition{
		id:        id,
		matches:   func(task string) bool { return strings.Contains(task, "environment") },
		docsTopic: func(task string) bool { return strings.Contains(task, "environment") },
	}
	t.Cleanup(func() { delete(applicationWorkflowDefinitions, id) })
	ctx := fixtureContext()
	ctx.Project.Components = append(ctx.Project.Components, "search")
	result, ok := Plan(ctx, PlanRequest{Task: "configure catalog search and typed environment configuration", App: "marketplace"})
	if !ok || !slices.Contains(result.WorkflowIDs, id) || !slices.Contains(result.WorkflowIDs, "goforj-catalog-search") || !slices.Contains(result.Commands, "forj marketplace generate --search") {
		t.Fatalf("composed search and configuration: %#v", result)
	}
	if slices.Contains(result.WorkflowIDs, "goforj-add-http-route") || slices.Contains(result.WorkflowIDs, "goforj-add-data-resource") {
		t.Fatalf("incidental generator workflow: %v", result.WorkflowIDs)
	}
	readOnly, ok := Plan(ctx, PlanRequest{Task: "search docs for catalog search"})
	if !ok || readOnly.WorkflowID != "goforj-read-docs" || len(readOnly.Commands) != 0 || len(readOnly.Files) != 0 || !docsContain(readOnly.Docs, "applications/library-resources.md") {
		t.Fatalf("search documentation became index creation: %#v", readOnly)
	}
}
