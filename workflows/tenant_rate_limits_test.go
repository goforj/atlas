package workflows

import (
	"context"
	"slices"
	"strings"
	"testing"

	"github.com/goforj/atlas/docs"
)

// TestTenantRateLimitPlanUsesExistingAppBoundary avoids generating new routes for a policy around existing exports.
func TestTenantRateLimitPlanUsesExistingAppBoundary(t *testing.T) {
	for _, app := range []string{"app", "marketplace"} {
		result, ok := Plan(fixtureContext(), PlanRequest{Task: "protect the existing export route with a shared per-tenant rate limit", App: app})
		if !ok || result.WorkflowID != "goforj-tenant-request-rate-limits" || !slices.Equal(result.WorkflowIDs, []string{"goforj-tenant-request-rate-limits"}) {
			t.Fatalf("policy plan: %#v", result)
		}
		build := "forj build"
		if app != "app" {
			build = "forj " + app + " build"
		}
		if !slices.Contains(result.Commands, build) || !slices.Contains(result.Commands, "go test ./...") {
			t.Fatalf("commands: %v", result.Commands)
		}
		for _, command := range result.Commands {
			if strings.Contains(command, "make:") {
				t.Fatalf("unrequested generator: %s", command)
			}
		}
		if !docsContain(result.Docs, "applications/middleware.md") || !docsContain(result.Docs, "libraries/cache.md") {
			t.Fatalf("docs: %#v", result.Docs)
		}
	}
}

// TestTenantRateLimitDocsPackUsesActualSections keeps docs-only discovery separate from policy source work.
func TestTenantRateLimitDocsPackUsesActualSections(t *testing.T) {
	provider := docs.StaticProvider{Docs: []docs.Document{
		{Path: "applications/middleware.md", Content: "# Middleware\n\n## Route Middleware\n\nApply a policy to its route.\n"},
		{Path: "libraries/cache.md", Content: "# Cache\n\n## Increment {#cache-increment}\n\nUse atomic counters.\n"},
	}}
	pack, err := DocsSectionPack(context.Background(), provider, "", "explain tenant request rate limits", 200)
	if err != nil || len(pack.Sections) != 2 {
		t.Fatalf("docs pack: %#v %v", pack, err)
	}
	for _, section := range pack.Sections {
		if !section.Found {
			t.Fatalf("missing section: %#v", section.Reference)
		}
	}
	plan, _ := Plan(fixtureContext(), PlanRequest{Task: "search docs for tenant rate limits"})
	if len(plan.Commands) != 0 || len(plan.Files) != 0 {
		t.Fatalf("docs changed source: %#v", plan)
	}
	if slices.Contains(ClassifyAll("implement a cache lock with ownership-safe release"), "goforj-tenant-request-rate-limits") {
		t.Fatal("lock conflated with quota")
	}
}
