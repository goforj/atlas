package eval

import (
	"context"
	"go/parser"
	"go/token"
	"os"
	"path/filepath"
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
	if _, err := parser.ParseFile(token.NewFileSet(), "probe_test.go", dispatchEnvironmentProbe+appResourceRuntimeHelpers, parser.AllErrors); err != nil {
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

// TestDispatchStartupOwningCompositionTests permits real App regression coverage while preserving production source ownership.
func TestDispatchStartupOwningCompositionTests(t *testing.T) {
	for _, test := range []struct {
		path    string
		allowed bool
	}{{path: "app/wire/dispatch_test.go", allowed: true}, {path: "app/lifecycle_test.go", allowed: true}, {path: "internal/dispatch/config_test.go", allowed: true}, {path: "app/wire/runtime.go", allowed: false}, {path: "internal/invoices/dispatch_test.go", allowed: false}} {
		t.Run(test.path, func(t *testing.T) {
			root := t.TempDir()
			path := filepath.Join(root, filepath.FromSlash(test.path))
			if err := os.MkdirAll(filepath.Dir(path), 0700); err != nil {
				t.Fatal(err)
			}
			if err := os.WriteFile(path, []byte("// Package fixture supplies ownership regression inputs.\npackage fixture\n\nimport \"testing\"\n\n// TestStartup covers the owning App startup boundary.\nfunc TestStartup(t *testing.T) {}\n"), 0600); err != nil {
				t.Fatal(err)
			}
			contract := dispatchEnvironmentContract()
			contract.sources = nil
			contract.commands = []commandContract{{id: "executed-behavior", arguments: []string{"go", "test", "./..."}}}
			runner := &fakeCommandRunner{}
			result, err := newSurfaceVerifier(runner, contract).Verify(context.Background(), VerificationInput{ProjectRoot: root, Changes: []ProjectChange{{Path: test.path, After: ProjectPathState{Kind: "file"}}}})
			if err != nil {
				t.Fatal(err)
			}
			want := EndpointFailed
			if test.allowed {
				want = EndpointPassed
			}
			if !checkHasStatus(result.Checks, "change-ownership", want) || (len(runner.commands) > 0) != test.allowed {
				t.Fatalf("ownership/behavior for %s = %#v, commands=%d", test.path, result.Checks, len(runner.commands))
			}
		})
	}
}
