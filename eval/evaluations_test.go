package eval

import (
	"path/filepath"
	"strings"
	"testing"
)

// TestLoadPromotedDefinitionKeepsBundledEvaluationAddressable verifies consumers need no source-tree path at runtime.
func TestLoadPromotedDefinitionKeepsBundledEvaluationAddressable(t *testing.T) {
	definition, err := LoadPromotedDefinition("add-http-controller")
	if err != nil {
		t.Fatalf("LoadPromotedDefinition(): %v", err)
	}
	if definition.ID != "add-http-controller" || definition.ProjectScenario != "invoice-http-route" || definition.PromptDigest == "" {
		t.Fatalf("definition = %#v", definition)
	}
	if _, err := LoadPromotedDefinition("../private"); err == nil {
		t.Fatal("unknown promoted evaluation was accepted")
	}
}

// TestAddHTTPControllerEvaluationLoads keeps the prompt compact and policy-free while preserving promoted contract references.
func TestAddHTTPControllerEvaluationLoads(t *testing.T) {
	definition, err := LoadDefinition(filepath.Join("evaluations", "add_http_controller"))
	if err != nil {
		t.Fatalf("LoadDefinition(): %v", err)
	}
	if definition.ProjectScenario != "invoice-http-route" || definition.WorkflowID != "goforj-add-http-route/v1" || definition.VerifierID != "add-http-controller/v1" {
		t.Fatalf("definition = %#v", definition)
	}
	for _, leakedRecipe := range []string{"make:controller", "inject_http_controllers_app.go", "wire_gen.go"} {
		if strings.Contains(definition.Prompt, leakedRecipe) {
			t.Fatalf("prompt leaks workflow recipe %q", leakedRecipe)
		}
	}
}

// TestPromotedMajorSurfaceEvaluationsResolve keeps every shipped manifest joined to reviewed workflow and verifier contracts.
func TestPromotedMajorSurfaceEvaluationsResolve(t *testing.T) {
	runner := &fakeCommandRunner{}
	registry, err := NewRegistry(PromotedWorkflows(), PromotedVerifiers(runner))
	if err != nil {
		t.Fatalf("NewRegistry(): %v", err)
	}
	ids, err := PromotedEvaluationIDs("")
	if err != nil {
		t.Fatalf("PromotedEvaluationIDs(): %v", err)
	}
	for _, id := range ids {
		t.Run(id, func(t *testing.T) {
			definition, err := LoadPromotedDefinition(id)
			if err != nil {
				t.Fatalf("LoadPromotedDefinition(): %v", err)
			}
			if definition.ID != id || definition.PromptDigest == "" || definition.Suite != "core" {
				t.Fatalf("definition = %#v", definition)
			}
			if _, err := registry.Resolve(definition); err != nil {
				t.Fatalf("Resolve(): %v", err)
			}
		})
	}
}

// TestPromotedEvaluationIDsReturnsStableSuiteCatalog keeps CLI discovery aligned with embedded manifests.
func TestPromotedEvaluationIDsReturnsStableSuiteCatalog(t *testing.T) {
	ids, err := PromotedEvaluationIDs("core")
	if err != nil {
		t.Fatalf("PromotedEvaluationIDs(): %v", err)
	}
	if len(ids) != 27 || ids[0] != "add-app-command" || ids[len(ids)-1] != "unknown-framework-shape" {
		t.Fatalf("ids = %v", ids)
	}
}
