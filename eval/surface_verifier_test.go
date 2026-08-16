package eval

import (
	"context"
	"os"
	"path/filepath"
	"testing"
)

// TestSurfaceVerifierUsesSyntaxAndStopsBeforeExecutingInvalidCandidates proves comments cannot satisfy contracts and static failures do not run code.
func TestSurfaceVerifierUsesSyntaxAndStopsBeforeExecutingInvalidCandidates(t *testing.T) {
	root := t.TempDir()
	path := filepath.Join(root, "internal", "feature", "command.go")
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatal(err)
	}
	valid := `package feature
import "context"
type Service struct{}
func (*Service) Find(context.Context, string) {}
type ShowCmd struct{ service *Service }
func (command *ShowCmd) Run(ctx context.Context) { command.service.Find(ctx, "42") }
`
	if err := os.WriteFile(path, []byte(valid), 0o644); err != nil {
		t.Fatal(err)
	}
	contract := surfaceContract{
		id:             "surface-test/v1",
		allowedChanges: []string{"internal/feature/*.go"},
		sources:        []sourceContract{{id: "shape", paths: []string{"internal/feature/*.go"}, identifiers: []string{"ShowCmd", "Service"}, selectorCalls: []string{"Find"}, forbiddenCalls: []string{"Background"}}},
		commands:       []commandContract{{id: "build", arguments: []string{"forj", "build"}}},
	}
	runner := &fakeCommandRunner{}
	verifier := NewSurfaceVerifier(runner, contract)
	result, err := verifier.Verify(context.Background(), VerificationInput{ProjectRoot: root})
	if err != nil {
		t.Fatalf("Verify(): %v", err)
	}
	if result.FrameworkOutcome.Status != EndpointPassed || len(runner.commands) != 1 {
		t.Fatalf("valid result = %#v; commands = %#v", result, runner.commands)
	}
	mutant := `package feature
import "context"
// ShowCmd Service Find are comments, not implementation evidence.
func Run() { _ = context.Background() }
`
	if err := os.WriteFile(path, []byte(mutant), 0o644); err != nil {
		t.Fatal(err)
	}
	runner.commands = nil
	result, err = verifier.Verify(context.Background(), VerificationInput{ProjectRoot: root})
	if err != nil {
		t.Fatalf("Verify(mutant): %v", err)
	}
	if result.FrameworkOutcome.Status != EndpointFailed || len(runner.commands) != 0 {
		t.Fatalf("mutant result = %#v; commands = %#v", result, runner.commands)
	}
}

// TestSurfaceVerifierRejectsOutOfScopeChanges keeps semantic success from hiding unrelated Project mutation.
func TestSurfaceVerifierRejectsOutOfScopeChanges(t *testing.T) {
	root := t.TempDir()
	path := filepath.Join(root, "internal", "feature", "feature.go")
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, []byte("package feature\ntype Feature struct{}\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	verifier := NewSurfaceVerifier(&fakeCommandRunner{}, surfaceContract{
		id:             "ownership-test/v1",
		allowedChanges: []string{"internal/feature/*.go"},
		sources:        []sourceContract{{id: "shape", paths: []string{"internal/feature/*.go"}, identifiers: []string{"Feature"}}},
	})
	result, err := verifier.Verify(context.Background(), VerificationInput{ProjectRoot: root, Changes: []ProjectChange{{Path: "app/routes.go"}}})
	if err != nil {
		t.Fatalf("Verify(): %v", err)
	}
	if result.FrameworkOutcome.Status != EndpointFailed {
		t.Fatalf("result = %#v", result)
	}
}

// TestSurfaceVerifierAllowsToolDerivedOutputs keeps build products from being misclassified as application ownership.
func TestSurfaceVerifierAllowsToolDerivedOutputs(t *testing.T) {
	result := verifySurfaceOwnership([]ProjectChange{
		{Path: "go.sum"},
		{Path: "bin"},
		{Path: "bin/app"},
		{Path: "build"},
		{Path: "build/api_index.json"},
		{Path: "app/wire/wire_gen.go"},
		{Path: "internal/database/_data", After: ProjectPathState{Kind: "directory"}},
		{Path: "internal/database/_data/sqlite/app.db"},
	}, nil)
	if result.Status != EndpointPassed {
		t.Fatalf("result = %#v", result)
	}
}

// TestSurfaceVerifierAllowsOwnedPackageDirectories keeps new cohesive package roots aligned with their reviewed file patterns.
func TestSurfaceVerifierAllowsOwnedPackageDirectories(t *testing.T) {
	result := verifySurfaceOwnership(
		[]ProjectChange{{Path: "internal/audits", After: ProjectPathState{Kind: "directory"}}},
		[]string{"internal/audits/*.go"},
	)
	if result.Status != EndpointPassed {
		t.Fatalf("result = %#v", result)
	}
}

// TestSurfaceVerifierAcceptsReviewedIdentifierFamilies keeps cohesive package naming flexible without weakening required structure.
func TestSurfaceVerifierAcceptsReviewedIdentifierFamilies(t *testing.T) {
	root := t.TempDir()
	path := filepath.Join(root, "internal", "profiles", "cache.go")
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, []byte("package profiles\ntype Cache struct{}\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	contract := sourceContract{id: "profile-cache", paths: []string{"internal/profiles/*.go"}, identifierChoices: [][]string{{"ProfileCache", "Cache"}}}
	if result := verifySurfaceSource(root, contract); result.Status != EndpointPassed {
		t.Fatalf("cohesive family result = %#v", result)
	}
	contract.identifierChoices = [][]string{{"ProfileCache", "Store"}}
	if result := verifySurfaceSource(root, contract); result.Status != EndpointFailed {
		t.Fatalf("unknown family result = %#v", result)
	}
}
