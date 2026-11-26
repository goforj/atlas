package eval

import (
	"context"
	"go/parser"
	"go/token"
	"strings"
	"testing"
)

// TestSettingsValidationContractPreservesHistoricEvaluation keeps the new presence task independently versioned and discoverable.
func TestSettingsValidationContractPreservesHistoricEvaluation(t *testing.T) {
	definition, err := LoadPromotedDefinition("patch-customer-settings")
	if err != nil {
		t.Fatal(err)
	}
	registry, err := NewRegistry(PromotedWorkflows(), PromotedVerifiers(&fakeCommandRunner{}))
	if err != nil {
		t.Fatal(err)
	}
	resolved, err := registry.Resolve(definition)
	if err != nil || resolved.Verifier.ID() != "patch-customer-settings/v1" || definition.ProjectScenario != "customer-settings-validation" || definition.Limits.ShellNetwork != "off" {
		t.Fatalf("definition = %#v, error = %v", definition, err)
	}
	old, err := LoadPromotedDefinition("add-validated-write-endpoint")
	if err != nil || old.VerifierID != "add-validated-write-endpoint/v1" || old.ProjectScenario != "create-invoice-validation" {
		t.Fatalf("historic definition = %#v, error = %v", old, err)
	}
	for _, recipe := range []string{"github.com/goforj/validation", "validation.Presence", "make:controller"} {
		if strings.Contains(definition.Prompt, recipe) {
			t.Errorf("natural prompt leaks %q", recipe)
		}
	}
}

// TestSettingsValidationProbeParses keeps supervisor-owned executable evidence valid before Project calibration.
func TestSettingsValidationProbeParses(t *testing.T) {
	if _, err := parser.ParseFile(token.NewFileSet(), "probe_test.go", settingsValidationBehaviorProbe, parser.AllErrors); err != nil {
		t.Fatal(err)
	}
	for _, source := range []string{settingsAppRuntimeProbe + appResourceRuntimeHelpers, settingsAppRuntimeCommand} {
		if _, err := parser.ParseFile(token.NewFileSet(), "app.go", source, parser.AllErrors); err != nil {
			t.Fatal(err)
		}
	}
	contract := settingsValidationContract()
	if contract.sources[0].id != "settings-library-path" || contract.commands[len(contract.commands)-4].id != "settings-validation-behavior" {
		t.Fatalf("contract = %#v", contract)
	}
}

// TestSettingsLibraryAdoptionDoesNotGateBehavior preserves independent attribution for adequate alternative implementations.
func TestSettingsLibraryAdoptionDoesNotGateBehavior(t *testing.T) {
	contract := settingsValidationContract()
	if contract.sources[0].kind != RequirementQuality {
		t.Fatal("library source observation gates application behavior")
	}
	checks := []EndpointResult{{ID: "settings-library-path", Kind: RequirementQuality, Status: EndpointFailed}, {ID: "settings-library-runtime", Kind: RequirementQuality, Status: EndpointFailed}, {ID: "settings-validation-behavior", Status: EndpointPassed}}
	if surfaceChecksFailed(checks) || summarizeSurfaceChecks(contract.id, checks).Status != EndpointPassed {
		t.Fatal("library adoption failure changed behavior outcome")
	}
	if _, err := parser.ParseFile(token.NewFileSet(), "library_test.go", settingsLibraryProbe, parser.AllErrors); err != nil {
		t.Fatal(err)
	}
}

// TestQualitySourceFailureStillExecutesBehavior checks propagation through the real verifier rather than only the summarizer.
func TestQualitySourceFailureStillExecutesBehavior(t *testing.T) {
	runner := &fakeCommandRunner{}
	verifier := newSurfaceVerifier(runner, surfaceContract{
		id:       "quality-source/v1",
		sources:  []sourceContract{{id: "library-choice", kind: RequirementQuality, paths: []string{"missing.go"}}},
		commands: []commandContract{{id: "behavior", arguments: []string{"go", "test", "./..."}}},
	})
	result, err := verifier.Verify(context.Background(), VerificationInput{ProjectRoot: t.TempDir()})
	if err != nil || result.FrameworkOutcome.Status != EndpointPassed || len(runner.commands) != 1 {
		t.Fatalf("result = %#v, error = %v, commands = %v", result, err, runner.commands)
	}
	found := false
	for _, check := range result.Checks {
		if check.ID == "library-choice" {
			found = check.Kind == RequirementQuality && check.Status == EndpointFailed
		}
	}
	if !found {
		t.Fatalf("source quality signal = %#v", result.Checks)
	}
}
