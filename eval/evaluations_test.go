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
