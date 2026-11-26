package eval

import (
	"context"
	"go/parser"
	"go/token"
	"slices"
	"strings"
	"testing"
)

// TestPartnerPayloadEncryptionEvaluationResolves keeps the new contract independently addressable without fixing the portfolio size.
func TestPartnerPayloadEncryptionEvaluationResolves(t *testing.T) {
	definition, err := LoadPromotedDefinition("encrypt-partner-payloads")
	if err != nil {
		t.Fatal(err)
	}
	registry, err := NewRegistry(PromotedWorkflows(), PromotedVerifiers(&fakeCommandRunner{}))
	if err != nil {
		t.Fatal(err)
	}
	resolved, err := registry.Resolve(definition)
	if err != nil || resolved.Verifier.ID() != "encrypt-partner-payloads/v1" || definition.ProjectScenario != "partner-payload-rotation" || definition.Limits.ShellNetwork != "off" {
		t.Fatalf("definition = %#v, error = %v", definition, err)
	}
	ids, err := PromotedEvaluationIDs("core")
	if err != nil || !slices.Contains(ids, definition.ID) {
		t.Fatalf("catalog = %v, error = %v", ids, err)
	}
	for _, recipe := range []string{"github.com/goforj/crypt", "crypt.NewFromEnv", "wire.NewSet"} {
		if strings.Contains(definition.Prompt, recipe) {
			t.Errorf("natural task leaks %q", recipe)
		}
	}
	if _, err := parser.ParseFile(token.NewFileSet(), "probe_test.go", payloadEncryptionProbe, parser.AllErrors); err != nil {
		t.Fatal(err)
	}
}

// TestCryptSourceAttributionDoesNotGateBehavior preserves independent results for compatible alternative implementations.
func TestCryptSourceAttributionDoesNotGateBehavior(t *testing.T) {
	runner := &fakeCommandRunner{}
	verifier := newSurfaceVerifier(runner, surfaceContract{id: "crypt-attribution/v1", sources: []sourceContract{{id: "crypt-api", kind: RequirementQuality, paths: []string{"missing.go"}}}, commands: []commandContract{{id: "behavior", arguments: []string{"go", "test", "./..."}}}})
	result, err := verifier.Verify(context.Background(), VerificationInput{ProjectRoot: t.TempDir()})
	if err != nil || result.FrameworkOutcome.Status != EndpointPassed || len(runner.commands) != 1 {
		t.Fatalf("result = %#v, error = %v", result, err)
	}
	if partnerPayloadEncryptionContract().sources[1].kind != RequirementQuality {
		t.Fatal("library source attribution became a behavior gate")
	}
}

// TestPayloadAppRuntimeSourcesParse catches invalid nested supervisor programs before rendered calibration.
func TestPayloadAppRuntimeSourcesParse(t *testing.T) {
	contract := appResourceRuntimeContract("payload-app-runtime", "partnerpayloads", payloadAppRuntimeProbe, payloadAppRuntimeCommand)
	for name, source := range map[string]string{"app_test.go": contract.supervisorFiles[0].body, "command.go": payloadAppRuntimeCommand} {
		if _, err := parser.ParseFile(token.NewFileSet(), name, source, parser.AllErrors); err != nil {
			t.Fatal(err)
		}
	}
}

// TestPayloadAppCompositionTestsStayInScope permits startup regression tests while rejecting unrelated App implementation changes.
func TestPayloadAppCompositionTestsStayInScope(t *testing.T) {
	contract := partnerPayloadEncryptionContract()
	for _, test := range []struct {
		path string
		want EndpointStatus
	}{
		{"app/wire/partnerpayloads_test.go", EndpointPassed},
		{"app/wire/unrelated.go", EndpointFailed},
	} {
		result := verifySurfaceOwnership([]ProjectChange{{Path: test.path, After: ProjectPathState{Kind: "file"}}}, contract.allowedChanges)
		if result.Status != test.want {
			t.Fatalf("ownership %s = %s", test.path, result.Status)
		}
	}
}
