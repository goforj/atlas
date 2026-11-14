package eval

import (
	"context"
	"go/ast"
	"go/parser"
	"go/token"
	"os"
	"path/filepath"
	"regexp"
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

// TestTaxRateResilienceCompletionMarkerKeepsEveryProbeSelected prevents a successful marker-only run from accepting semantic mutants.
func TestTaxRateResilienceCompletionMarkerKeepsEveryProbeSelected(t *testing.T) {
	var command commandContract
	for _, contract := range promotedSurfaceContracts() {
		if contract.id == "harden-tax-rate-client/v1" {
			command = contract.commands[1]
		}
	}
	runner := &fakeCommandRunner{files: map[string][]byte{}}
	arguments, _, err := installSupervisorCompletionMarker(runner, command.supervisorFiles[0], command.arguments)
	if err != nil {
		t.Fatal(err)
	}
	var selection *regexp.Regexp
	for index, argument := range arguments {
		if argument == "-run" {
			selection, err = regexp.Compile(arguments[index+1])
			if err != nil {
				t.Fatal(err)
			}
			break
		}
	}
	if selection == nil {
		t.Fatal("missing test selection")
	}
	file, err := parser.ParseFile(token.NewFileSet(), "probe_test.go", taxRateResilienceBehaviorProbe, 0)
	if err != nil {
		t.Fatal(err)
	}
	checked := 0
	for _, declaration := range file.Decls {
		function, ok := declaration.(*ast.FuncDecl)
		if !ok || !strings.HasPrefix(function.Name.Name, "TestAtlasTaxRateResilience") {
			continue
		}
		if !selection.MatchString(function.Name.Name) {
			t.Errorf("completion marker excluded %s", function.Name.Name)
		}
		checked++
	}
	if checked < 4 {
		t.Fatalf("probe tests = %d", checked)
	}
}
