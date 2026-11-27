package workflows

import (
	"context"
	"slices"
	"strings"
	"testing"

	"github.com/goforj/atlas/docs"
)

// TestFeaturePlansUseEvaluatorResources keeps stable tenant rollouts out of controller and database scaffolding.
func TestFeaturePlansUseEvaluatorResources(t *testing.T) {
	ctx := fixtureContext()
	ctx.Project.Components = append(ctx.Project.Components, "feature")
	for _, task := range []string{"stable tenant checkout rollout", "configure feature flag provider", "declare a flag resource for request snapshots", "configure FEATURE_CHECKOUT_DRIVER=file", "schedule a tenant flag rollout"} {
		if got := Classify(task); got != "goforj-feature-rollouts" {
			t.Fatalf("Classify(%q) = %q", task, got)
		}
		for _, app := range []string{"app", "marketplace"} {
			result, ok := Plan(ctx, PlanRequest{Task: task, App: app})
			if !ok {
				t.Fatal("missing App plan")
			}
			command := "forj generate --feature"
			wire := "app/wire/inject_services_app.go"
			if app != "app" {
				command = "forj " + app + " generate --feature"
				wire = "app/" + app + "/wire/inject_services_app.go"
			}
			if !slices.Contains(result.Commands, command) {
				t.Fatalf("commands = %v", result.Commands)
			}
			for _, path := range []string{".goforj.yml", ".env", "internal/features/manager_gen.go", "internal/features/accessors_gen.go", wire} {
				if !slices.Contains(result.Files, path) {
					t.Fatalf("missing %s: %v", path, result.Files)
				}
			}
			for _, command := range result.Commands {
				if strings.Contains(command, "make:") {
					t.Fatalf("unrelated generator for %q: %s", task, command)
				}
			}
			if !docsContain(result.Docs, "developer-tools/wiring-recipes.md") {
				t.Fatalf("docs = %#v", result.Docs)
			}
			advice, ok := CommandAdvice(ctx, CommandAdviceRequest{Task: task, App: app})
			if !ok || advice.Command != command {
				t.Fatalf("advice = %#v", advice)
			}
		}
	}
}

// TestFeaturePlansRequireCapabilityEvidence avoids inventing a minimum release for the new optional component.
func TestFeaturePlansRequireCapabilityEvidence(t *testing.T) {
	ctx := fixtureContext()
	ctx.Project.GoForjVersion = "v0.37.0"
	result, ok := Plan(ctx, PlanRequest{Task: "stable tenant checkout rollout", App: "marketplace"})
	if !ok {
		t.Fatal("missing plan")
	}
	if slices.Contains(result.Commands, "forj marketplace generate --feature") || !slices.Contains(result.Commands, "forj marketplace generate --help") {
		t.Fatalf("unsupported generator advice: %v", result.Commands)
	}
	if !strings.Contains(strings.Join(result.Warnings, " "), "does not declare the feature component") {
		t.Fatalf("warnings = %v", result.Warnings)
	}
	for _, step := range result.Steps {
		if step.Action == "forj marketplace generate --feature" {
			t.Fatal("unsupported generation step remained")
		}
	}
	advice, ok := CommandAdvice(ctx, CommandAdviceRequest{Task: "configure feature evaluator", App: "marketplace"})
	if !ok || advice.Command != "forj marketplace generate --help" {
		t.Fatalf("advice = %#v", advice)
	}
}

// TestFeaturePlansComposeHTTPAndPreserveFrontendMeaning limits rollout advice to evaluator concerns.
func TestFeaturePlansComposeHTTPAndPreserveFrontendMeaning(t *testing.T) {
	ids := ClassifyAll("add HTTP endpoint for stable tenant checkout rollout")
	if !slices.Contains(ids, "goforj-add-http-route") || !slices.Contains(ids, "goforj-feature-rollouts") || slices.Contains(ids, "goforj-add-data-resource") {
		t.Fatalf("composed ids = %v", ids)
	}
	if got := Classify("add HTTP endpoint for stable tenant checkout rollout"); got != "goforj-add-http-route" {
		t.Fatal(got)
	}
	if got := Classify("repair missing provider for a feature flag"); got != "goforj-wire-repair" {
		t.Fatal(got)
	}
	for _, task := range []string{"refresh feature-local server data in Vue view", "add a frontend feature", "create a frontend feature resource for profiles", "discuss the feature flag API"} {
		ids := ClassifyAll(task)
		if task == "discuss the feature flag API" {
			if slices.Contains(ids, "goforj-add-http-route") {
				t.Fatalf("API discussion invented transport: %v", ids)
			}
		} else if slices.Contains(ids, "goforj-feature-rollouts") {
			t.Fatalf("generic feature vocabulary selected flags: %v", ids)
		}
	}
}

// TestFeatureDocsPackAndGeneratedOwnership exercises bounded hosted guidance and source-of-truth policy.
func TestFeatureDocsPackAndGeneratedOwnership(t *testing.T) {
	provider := docs.StaticProvider{Docs: []docs.Document{
		{Path: "developer-tools/wiring-recipes.md", Content: "# Library Resources\n\n## Named Resource\n\nDeclare FEATURE_* resources before generation.\n"},
	}}
	pack, err := DocsSectionPack(context.Background(), provider, "", "stable tenant checkout rollout", 120)
	if err != nil || pack.WorkflowID != "goforj-feature-rollouts" || len(pack.Sections) != 1 {
		t.Fatalf("pack = %#v, err = %v", pack, err)
	}
	for _, section := range pack.Sections {
		if !section.Found {
			t.Fatalf("missing section: %#v", section.Reference)
		}
	}
	for _, path := range []string{"internal/features/manager_gen.go", "internal/features/accessors_gen.go"} {
		policy := FilePolicy(FilePolicyRequest{Path: path})
		if policy.Editable || !strings.Contains(policy.ChangeThrough, "generate --feature") {
			t.Fatalf("policy = %#v", policy)
		}
	}
}

// TestFeatureDocumentationPlansRemainReadOnly keeps evaluator explanations independent from source generation.
func TestFeatureDocumentationPlansRemainReadOnly(t *testing.T) {
	for _, task := range []string{"explain tenant feature rollouts", "read docs for feature flag resources", "search documentation for stable tenant rollout snapshots"} {
		for _, app := range []string{"app", "marketplace"} {
			plan, ok := Plan(fixtureContext(), PlanRequest{Task: task, App: app})
			if !ok || plan.WorkflowID != "goforj-read-docs" || len(plan.Commands) != 0 || len(plan.Files) != 0 || len(plan.Verification) != 0 || !docsContain(plan.Docs, "developer-tools/wiring-recipes.md") {
				t.Fatalf("docs plan=%#v", plan)
			}
			advice, ok := CommandAdvice(fixtureContext(), CommandAdviceRequest{Task: task, App: app})
			if !ok || advice.Command != "Atlas search-docs" {
				t.Fatalf("advice=%#v", advice)
			}
		}
	}
}
