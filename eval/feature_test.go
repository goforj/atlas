package eval

import (
	"context"
	"os"
	"path/filepath"
	"testing"
)

// TestFeatureSelectionQualityDoesNotGateBehavior keeps library preference separate from application correctness.
func TestFeatureSelectionQualityDoesNotGateBehavior(t *testing.T) {
	root := t.TempDir()
	if err := os.WriteFile(filepath.Join(root, "service.go"), []byte("package checkout\n"), 0600); err != nil {
		t.Fatal(err)
	}
	verifier := newSurfaceVerifier(&fakeCommandRunner{}, surfaceContract{id: "quality-test/v1", sources: []sourceContract{{id: "library-selection", kind: RequirementQuality, paths: []string{"service.go"}, stringLiterals: []string{"github.com/goforj/feature"}}}})
	result, err := verifier.Verify(context.Background(), VerificationInput{ProjectRoot: root})
	if err != nil {
		t.Fatal(err)
	}
	if result.FrameworkOutcome.Status != EndpointPassed {
		t.Fatalf("quality preference gated behavior: %#v", result)
	}
	check := result.Checks[len(result.Checks)-1]
	if check.Kind != RequirementQuality || check.Status != EndpointFailed {
		t.Fatalf("selection result = %#v", check)
	}
}

// TestFeatureEvaluationDisclosesRequestBoundary keeps the supervisor probe's public API in the task.
func TestFeatureEvaluationDisclosesRequestBoundary(t *testing.T) {
	definition, err := LoadPromotedDefinition("target-feature-rollout")
	if err != nil {
		t.Fatal(err)
	}
	if definition.ProjectScenario != "tenant-checkout-rollout" || definition.VerifierID != "target-feature-rollout/v1" {
		t.Fatalf("definition = %#v", definition)
	}
}
