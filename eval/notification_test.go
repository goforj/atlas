package eval

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// TestIncidentEvaluationPromotesNaturalAcceptanceTask protects the discovery prompt and supervisor boundary.
func TestIncidentEvaluationPromotesNaturalAcceptanceTask(t *testing.T) {
	definition, err := LoadPromotedDefinition("notify-incident")
	if err != nil {
		t.Fatal(err)
	}
	registry, err := NewRegistry(PromotedWorkflows(), PromotedVerifiers(VerifierCommands{}))
	if err != nil {
		t.Fatal(err)
	}
	resolved, err := registry.Resolve(definition)
	if err != nil {
		t.Fatal(err)
	}
	if resolved.Definition.ProjectScenario != "incident-notification" {
		t.Fatal("wrong prepared scenario")
	}
	for _, leaked := range []string{"github.com/goforj/notification", "make:notification", "notification.Fallback", "TestAtlas"} {
		if strings.Contains(definition.Prompt, leaked) {
			t.Errorf("prompt leaks %q", leaked)
		}
	}
	for _, term := range []string{"NewAlertService", "Notify(ctx, Incident)", "Unknown acceptance", "dependency installation is outside"} {
		if !strings.Contains(definition.Prompt, term) {
			t.Errorf("prompt omits %q", term)
		}
	}
}

// TestNotificationSelectionQualityDoesNotGateBehavior preserves correct alternative sender composition.
func TestNotificationSelectionQualityDoesNotGateBehavior(t *testing.T) {
	root := t.TempDir()
	if err := os.WriteFile(filepath.Join(root, "service.go"), []byte("package incidents\n"), 0600); err != nil {
		t.Fatal(err)
	}
	verifier := newSurfaceVerifier(&fakeCommandRunner{}, surfaceContract{id: "quality-test/v1", sources: []sourceContract{{id: "library-selection", kind: RequirementQuality, paths: []string{"service.go"}, stringLiterals: []string{"github.com/goforj/notification"}}}})
	result, err := verifier.Verify(context.Background(), VerificationInput{ProjectRoot: root})
	if err != nil {
		t.Fatal(err)
	}
	if result.FrameworkOutcome.Status != EndpointPassed {
		t.Fatalf("library preference gated behavior: %#v", result)
	}
	check := result.Checks[len(result.Checks)-1]
	if check.Kind != RequirementQuality || check.Status != EndpointFailed {
		t.Fatalf("selection result = %#v", check)
	}
}
