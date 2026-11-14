package eval

import (
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
