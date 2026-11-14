package eval

import (
	"context"
	"go/parser"
	"go/token"
	"slices"
	"strings"
	"testing"
)

// TestDispatchStartupEvaluationResolves preserves independent catalog membership without fixing the portfolio size.
func TestDispatchStartupEvaluationResolves(t *testing.T) {
	definition, err := LoadPromotedDefinition("configure-dispatch-startup")
	if err != nil {
		t.Fatal(err)
	}
	registry, err := NewRegistry(PromotedWorkflows(), PromotedVerifiers(&fakeCommandRunner{}))
	if err != nil {
		t.Fatal(err)
	}
	resolved, err := registry.Resolve(definition)
	if err != nil || resolved.Verifier.ID() != "configure-dispatch-startup/v1" || definition.ProjectScenario != "dispatch-startup-config" || definition.Limits.ShellNetwork != "off" {
		t.Fatalf("definition = %#v, error = %v", definition, err)
	}
	ids, err := PromotedEvaluationIDs("core")
	if err != nil || !slices.Contains(ids, definition.ID) {
		t.Fatalf("catalog = %v, error = %v", ids, err)
	}
	for _, recipe := range []string{"github.com/goforj/env", "env.GetInt", "wire.NewSet"} {
		if strings.Contains(definition.Prompt, recipe) {
			t.Errorf("natural task leaks %q", recipe)
		}
	}
	if _, err := parser.ParseFile(token.NewFileSet(), "probe_test.go", dispatchEnvironmentProbe, parser.AllErrors); err != nil {
		t.Fatal(err)
	}
}

// TestEnvironmentLibraryAttributionDoesNotGateStartup verifies source observations remain separate from application behavior.
func TestEnvironmentLibraryAttributionDoesNotGateStartup(t *testing.T) {
	runner := &fakeCommandRunner{}
	verifier := newSurfaceVerifier(runner, surfaceContract{
		id:       "env-attribution/v1",
		sources:  []sourceContract{{id: "env-api", kind: RequirementQuality, paths: []string{"missing.go"}}},
		commands: []commandContract{{id: "startup", arguments: []string{"go", "test", "./..."}}},
	})
	result, err := verifier.Verify(context.Background(), VerificationInput{ProjectRoot: t.TempDir()})
	if err != nil || result.FrameworkOutcome.Status != EndpointPassed || len(runner.commands) != 1 {
		t.Fatalf("result = %#v, error = %v", result, err)
	}
	if dispatchEnvironmentContract().sources[1].kind != RequirementQuality {
		t.Fatal("env API observation became a behavior gate")
	}
}
