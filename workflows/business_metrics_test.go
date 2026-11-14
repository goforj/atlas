package workflows

import (
	"slices"
	"strings"
	"testing"
)

// TestBusinessMetricsPlansUseExistingRegistry keeps instrumentation distinct from runtime diagnostics.
func TestBusinessMetricsPlansUseExistingRegistry(t *testing.T) {
	ctx := fixtureContext()
	for _, task := range []string{"instrument checkout with business metrics", "record checkout counters and duration histogram", "add a successful purchase counter"} {
		if got := Classify(task); got != "goforj-business-metrics" {
			t.Fatalf("Classify(%q)=%s", task, got)
		}
		for _, app := range []string{"app", "marketplace"} {
			result, ok := Plan(ctx, PlanRequest{Task: task, App: app})
			if !ok {
				t.Fatal("missing plan")
			}
			wire := "app/wire/inject_services_app.go"
			if app != "app" {
				wire = "app/" + app + "/wire/inject_services_app.go"
			}
			if !slices.Contains(result.Files, wire) || !docsContain(result.Docs, "operations/metrics.md") {
				t.Fatalf("plan=%#v", result)
			}
			for _, command := range result.Commands {
				if strings.Contains(command, "make:") {
					t.Fatal(command)
				}
			}
		}
	}
	if Classify("debug Lighthouse runtime metrics") != "goforj-debug-runtime" {
		t.Fatal("diagnostic route changed")
	}
	ids := ClassifyAll("add HTTP endpoint and business metrics for checkout")
	if !slices.Contains(ids, "goforj-add-http-route") || !slices.Contains(ids, "goforj-business-metrics") {
		t.Fatal(ids)
	}
}

// TestBusinessMetricsDocumentationRemainsReadOnly preserves retrieval intent across focused instrumentation topics.
func TestBusinessMetricsDocumentationRemainsReadOnly(t *testing.T) {
	for _, task := range []string{"search docs for business metrics", "explain checkout counters", "read documentation about duration histograms"} {
		plan, ok := Plan(fixtureContext(), PlanRequest{Task: task, App: "marketplace"})
		if !ok || plan.WorkflowID != "goforj-read-docs" || len(plan.Commands) != 0 || len(plan.Files) != 0 || len(plan.Verification) != 0 || !docsContain(plan.Docs, "operations/metrics.md") {
			t.Fatalf("docs plan=%#v", plan)
		}
	}
}
