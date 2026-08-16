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
	verifier := newSurfaceVerifier(runner, contract)
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
	verifier := newSurfaceVerifier(&fakeCommandRunner{}, surfaceContract{
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
		{Path: "storage", After: ProjectPathState{Kind: "directory"}},
		{Path: "storage/app/private", After: ProjectPathState{Kind: "directory"}},
		{Path: "internal/avatars/storage", After: ProjectPathState{Kind: "directory"}},
		{Path: "internal/avatars/storage/app/private", After: ProjectPathState{Kind: "directory"}},
		{Path: "app/wire/wire_gen.go"},
		{Path: "internal/database/_data", After: ProjectPathState{Kind: "directory"}},
		{Path: "internal/database/_data/sqlite/app.db"},
	}, nil)
	if result.Status != EndpointPassed {
		t.Fatalf("result = %#v", result)
	}
}

// TestSurfaceVerifierRejectsSourceInsideNestedRuntimeStorage proves the directory exception cannot hide an authored Go package.
func TestSurfaceVerifierRejectsSourceInsideNestedRuntimeStorage(t *testing.T) {
	result := verifySurfaceOwnership(
		[]ProjectChange{{Path: "internal/avatars/storage/service.go", After: ProjectPathState{Kind: "file"}}},
		[]string{"internal/avatars/avatar_storage.go"},
	)
	if result.Status != EndpointFailed {
		t.Fatalf("ownership result = %#v, want nested storage source rejected", result)
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

// TestSurfaceVerifierScopesRelatedEvidenceToDeclarations prevents unused helpers from satisfying behavior owned by another function.
func TestSurfaceVerifierScopesRelatedEvidenceToDeclarations(t *testing.T) {
	root := t.TempDir()
	path := filepath.Join(root, "service.go")
	mutant := `package feature
type Repository struct{}
func (Repository) WithTransaction(callback func()) { callback() }
func (Repository) AdjustBalance() {}
func unused(repository Repository) { repository.WithTransaction(func() { repository.AdjustBalance() }) }
func Transfer(repository Repository) { repository.AdjustBalance() }
`
	if err := os.WriteFile(path, []byte(mutant), 0o600); err != nil {
		t.Fatal(err)
	}
	contract := sourceContract{
		id:    "transaction",
		paths: []string{"service.go"},
		declarations: []declarationContract{{
			name:          "Transfer",
			selectorCalls: []string{"WithTransaction", "AdjustBalance"},
			nestedCalls:   []nestedCallContract{{outer: "WithTransaction", inner: "AdjustBalance"}},
		}},
	}
	if result := verifySurfaceSource(root, contract); result.Status != EndpointFailed {
		t.Fatalf("mutant result = %#v, want declaration-scoped failure", result)
	}
	valid := `package feature
type Repository struct{}
func (Repository) WithTransaction(callback func()) { callback() }
func (Repository) AdjustBalance() {}
func Transfer(repository Repository) { repository.WithTransaction(func() { repository.AdjustBalance() }) }
`
	if err := os.WriteFile(path, []byte(valid), 0o600); err != nil {
		t.Fatal(err)
	}
	if result := verifySurfaceSource(root, contract); result.Status != EndpointPassed {
		t.Fatalf("valid result = %#v", result)
	}
}

// TestSurfaceVerifierScopesMethodsToTheirOwningReceiver prevents an unrelated type from satisfying a required application boundary.
func TestSurfaceVerifierScopesMethodsToTheirOwningReceiver(t *testing.T) {
	root := t.TempDir()
	path := filepath.Join(root, "service.go")
	mutant := `package feature
type Service struct{}
type Decoy struct{}
func (Service) Find() {}
func (Decoy) Run() { Service{}.Find() }
func (Service) Run() {}
`
	if err := os.WriteFile(path, []byte(mutant), 0o600); err != nil {
		t.Fatal(err)
	}
	contract := sourceContract{
		id:    "service-run",
		paths: []string{"service.go"},
		declarations: []declarationContract{{
			name:          "Run",
			receiver:      "Service",
			selectorCalls: []string{"Find"},
		}},
	}
	if result := verifySurfaceSource(root, contract); result.Status != EndpointFailed {
		t.Fatalf("mutant result = %#v, want receiver-scoped failure", result)
	}
}

// TestSurfaceVerifierRecognizesGenericCalls keeps typed helper APIs visible to declaration-scoped contracts.
func TestSurfaceVerifierRecognizesGenericCalls(t *testing.T) {
	root := t.TempDir()
	path := filepath.Join(root, "repository.go")
	source := `package feature
type Cache struct{}
func Get[T any](Cache, string) (T, bool) { var zero T; return zero, false }
type Repository struct{ cache Cache }
func (repository Repository) Find() { _, _ = Get[string](repository.cache, "key") }
`
	if err := os.WriteFile(path, []byte(source), 0o600); err != nil {
		t.Fatal(err)
	}
	contract := sourceContract{
		id:           "generic-call",
		paths:        []string{"repository.go"},
		declarations: []declarationContract{{name: "Find", receiver: "Repository", selectorCalls: []string{"Get"}}},
	}
	if result := verifySurfaceSource(root, contract); result.Status != EndpointPassed {
		t.Fatalf("result = %#v, want generic call recognized", result)
	}
}

// TestSurfaceVerifierReportsCandidateTestsAsNonGatingQuality keeps authored coverage visible without making candidate code its own oracle.
func TestSurfaceVerifierReportsCandidateTestsAsNonGatingQuality(t *testing.T) {
	root := t.TempDir()
	path := filepath.Join(root, "internal", "feature", "service.go")
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, []byte("package feature\ntype Service struct{}\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	verifier := newSurfaceVerifier(&fakeCommandRunner{}, surfaceContract{
		id:                  "quality-test/v1",
		allowedChanges:      []string{"internal/feature/*.go"},
		qualityTestPatterns: []string{"internal/feature/*_test.go"},
		sources:             []sourceContract{{id: "shape", paths: []string{"internal/feature/*.go"}, identifiers: []string{"Service"}}},
	})
	result, err := verifier.Verify(context.Background(), VerificationInput{
		ProjectRoot: root,
		Changes:     []ProjectChange{{Path: "internal/feature/service.go", After: ProjectPathState{Kind: "file"}}},
	})
	if err != nil {
		t.Fatalf("Verify(): %v", err)
	}
	if result.FrameworkOutcome.Status != EndpointPassed {
		t.Fatalf("framework outcome = %#v, want quality signal to remain non-gating", result.FrameworkOutcome)
	}
	if len(result.Checks) < 2 || result.Checks[1].ID != "focused-tests-added" || result.Checks[1].Kind != RequirementQuality || result.Checks[1].Status != EndpointFailed {
		t.Fatalf("quality check = %#v", result.Checks)
	}

	result, err = verifier.Verify(context.Background(), VerificationInput{
		ProjectRoot: root,
		Changes: []ProjectChange{
			{Path: "internal/feature/service.go", After: ProjectPathState{Kind: "file"}},
			{Path: "internal/feature/service_test.go", After: ProjectPathState{Kind: "file"}},
		},
	})
	if err != nil {
		t.Fatalf("Verify(with test): %v", err)
	}
	if result.Checks[1].Status != EndpointPassed {
		t.Fatalf("quality check = %#v, want focused test reported", result.Checks[1])
	}
}

// TestSurfaceVerifierRelatesRoutesToTheirAssignedGroup rejects a compiling route placed in the public group.
func TestSurfaceVerifierRelatesRoutesToTheirAssignedGroup(t *testing.T) {
	root := t.TempDir()
	path := filepath.Join(root, "routes.go")
	mutant := `package app
func routes(invoicesController Controller) {
	publicRoutes := concat(invoicesController.Routes())
	protectedRoutes := concat()
	_, _ = publicRoutes, protectedRoutes
}
`
	if err := os.WriteFile(path, []byte(mutant), 0o600); err != nil {
		t.Fatal(err)
	}
	contract := sourceContract{
		id:    "route-groups",
		paths: []string{"routes.go"},
		assignments: []assignmentContract{
			{name: "publicRoutes", forbiddenIdentifiers: []string{"invoicesController"}},
			{name: "protectedRoutes", identifiers: []string{"invoicesController"}, selectorCalls: []string{"Routes"}},
		},
	}
	if result := verifySurfaceSource(root, contract); result.Status != EndpointFailed {
		t.Fatalf("mutant result = %#v, want wrong-group failure", result)
	}
}

// TestSurfaceVerifierExcludesCandidateTestsFromSourceEvidence prevents candidate-owned tests from satisfying or invalidating implementation contracts.
func TestSurfaceVerifierExcludesCandidateTestsFromSourceEvidence(t *testing.T) {
	root := t.TempDir()
	if err := os.WriteFile(filepath.Join(root, "service.go"), []byte("package feature\ntype Service struct{}\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	testSource := `package feature
import "context"
func candidateEvidence() { _ = context.Background() }
`
	if err := os.WriteFile(filepath.Join(root, "service_test.go"), []byte(testSource), 0o600); err != nil {
		t.Fatal(err)
	}
	contract := sourceContract{id: "service", paths: []string{"*.go"}, identifiers: []string{"Service"}, forbiddenCalls: []string{"Background"}}
	if result := verifySurfaceSource(root, contract); result.Status != EndpointPassed {
		t.Fatalf("result = %#v, want candidate tests excluded", result)
	}
}

// TestRunIsolatedCommandInstallsSupervisorFiles proves executable behavior comes from verifier-owned source after candidate tests are removed.
func TestRunIsolatedCommandInstallsSupervisorFiles(t *testing.T) {
	runner := &fakeCommandRunner{}
	contract := commandContract{
		id:        "trusted-probe",
		arguments: []string{"go", "test", "./feature"},
		supervisorFiles: []supervisorFile{{
			path: "feature/atlas_eval_test.go",
			body: "package feature\n",
		}},
	}
	result := runIsolatedCommand(context.Background(), runner, t.TempDir(), contract)
	if result.Status != EndpointPassed {
		t.Fatalf("result = %#v", result)
	}
	if got := string(runner.files["feature/atlas_eval_test.go"]); got != "package feature\n" {
		t.Fatalf("supervisor file = %q", got)
	}
}
