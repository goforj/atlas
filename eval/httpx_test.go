package eval

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// TestTaxRateResilienceEvaluationPreservesOriginalContract prevents the new retry task from regrading historical results.
func TestTaxRateResilienceEvaluationPreservesOriginalContract(t *testing.T) {
	definition, err := LoadPromotedDefinition("harden-tax-rate-client")
	if err != nil {
		t.Fatal(err)
	}
	registry, err := NewRegistry(PromotedWorkflows(), PromotedVerifiers(VerifierCommands{}))
	if err != nil {
		t.Fatal(err)
	}
	if _, err := registry.Resolve(definition); err != nil {
		t.Fatal(err)
	}
	original, err := LoadPromotedDefinition("add-outbound-http-integration")
	if err != nil {
		t.Fatal(err)
	}
	if original.VerifierID != "add-outbound-http-integration/v1" || definition.ProjectScenario != "resilient-tax-rate-http" {
		t.Fatal("original contract replaced")
	}
	for _, leak := range []string{"github.com/goforj/httpx", "RetryCount", "make:", "TestAtlas"} {
		if strings.Contains(definition.Prompt, leak) {
			t.Errorf("task leaks %q", leak)
		}
	}
}

// TestHTTPClientSelectionQualityDoesNotGateBehavior separates retaining a library from the observable request policy.
func TestHTTPClientSelectionQualityDoesNotGateBehavior(t *testing.T) {
	root := t.TempDir()
	if err := os.WriteFile(filepath.Join(root, "client.go"), []byte("package taxrates\n"), 0600); err != nil {
		t.Fatal(err)
	}
	verifier := newSurfaceVerifier(&fakeCommandRunner{}, surfaceContract{id: "quality-test/v1", sources: []sourceContract{{id: "library-selection", kind: RequirementQuality, paths: []string{"client.go"}, stringLiterals: []string{"github.com/goforj/httpx/v2"}}}})
	result, err := verifier.Verify(context.Background(), VerificationInput{ProjectRoot: root})
	if err != nil {
		t.Fatal(err)
	}
	if result.FrameworkOutcome.Status != EndpointPassed {
		t.Fatalf("selection gated behavior: %#v", result)
	}
	check := result.Checks[len(result.Checks)-1]
	if check.Kind != RequirementQuality || check.Status != EndpointFailed {
		t.Fatalf("selection = %#v", check)
	}
}
