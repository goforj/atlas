package eval

import (
	"context"
	"go/parser"
	"go/token"
	"os"
	"path/filepath"
	"testing"
)

// TestBusinessMetricsManifestAndQuality keeps adoption separate from required executed behavior.
func TestBusinessMetricsManifestAndQuality(t *testing.T) {
	definition, err := LoadPromotedDefinition("instrument-checkout-metrics")
	if err != nil {
		t.Fatal(err)
	}
	if definition.ProjectScenario != "checkout-business-metrics" || definition.VerifierID != "checkout-business-metrics/v1" {
		t.Fatalf("definition=%#v", definition)
	}
	root := t.TempDir()
	if err := os.WriteFile(filepath.Join(root, "service.go"), []byte("package checkout\n"), 0600); err != nil {
		t.Fatal(err)
	}
	verifier := newSurfaceVerifier(&fakeCommandRunner{}, surfaceContract{id: "metrics-quality/v1", sources: []sourceContract{{id: "adoption", kind: RequirementQuality, paths: []string{"service.go"}, stringLiterals: []string{"github.com/goforj/metrics"}}}})
	result, err := verifier.Verify(context.Background(), VerificationInput{ProjectRoot: root})
	if err != nil {
		t.Fatal(err)
	}
	if result.FrameworkOutcome.Status != EndpointPassed {
		t.Fatalf("quality gated behavior: %#v", result)
	}
}

// TestBusinessMetricsLocalProviderRegistrationRequiresSameDirectory keeps private App factories connected without accepting sibling names.
func TestBusinessMetricsLocalProviderRegistrationRequiresSameDirectory(t *testing.T) {
	file, err := parser.ParseFile(token.NewFileSet(), "inject_services_app.go", `package wire;import "github.com/goforj/wire";var appSet=wire.NewSet(provideCheckout)`, 0)
	if err != nil {
		t.Fatal(err)
	}
	provider := namedResourceProvider{name: "provideCheckout", directory: "app/wire", packageName: "wire"}
	if !wireSetReferencesProvider(file, "app/wire", []namedResourceProvider{provider}) {
		t.Fatal("private factory is not connected")
	}
	if wireSetReferencesProvider(file, "app/admin/wire", []namedResourceProvider{provider}) {
		t.Fatal("sibling factory satisfied local registration")
	}
}

// TestBusinessMetricsAppRuntimeSources preserves compilable supervisor command and active App coverage.
func TestBusinessMetricsAppRuntimeSources(t *testing.T) {
	contract := appResourceRuntimeContract("business-metrics-app-runtime", "checkout", businessMetricsAppRuntimeProbe, businessMetricsAppRuntimeCommand)
	for path, source := range map[string]string{"probe_test.go": contract.supervisorFiles[0].body, "command.go": businessMetricsAppRuntimeCommand} {
		if _, err := parser.ParseFile(token.NewFileSet(), path, source, parser.ParseComments); err != nil {
			t.Fatal(err)
		}
	}
}
