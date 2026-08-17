package eval

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"testing"
)

// TestWireOutputParityRequiresAuthoritativeRegeneration verifies generated output is compared only inside a disposable verifier clone.
func TestWireOutputParityRequiresAuthoritativeRegeneration(t *testing.T) {
	runner := &fakeCommandRunner{files: map[string][]byte{"app/wire/wire_gen.go": []byte("generated")}}
	probe := verifyWireOutputParity()
	result := probe(context.Background(), runner, VerifierProject{})
	if result.Status != EndpointPassed {
		t.Fatalf("golden parity = %#v", result)
	}
	if len(runner.commands) != 1 || !slices.Equal(runner.commands[0], []string{"forj", "build"}) {
		t.Fatalf("commands = %#v", runner.commands)
	}

	runner = &fakeCommandRunner{files: map[string][]byte{"app/wire/wire_gen.go": []byte("manual edit")}}
	runner.onRun = func(command []string) {
		if slices.Equal(command, []string{"forj", "build"}) {
			runner.files["app/wire/wire_gen.go"] = []byte("generated")
		}
	}
	result = probe(context.Background(), runner, VerifierProject{})
	if result.Status != EndpointFailed || result.ID != "wire-output-parity" {
		t.Fatalf("manual output mutant = %#v", result)
	}

	runner = &fakeCommandRunner{}
	runner.onRun = func(command []string) {
		if slices.Equal(command, []string{"forj", "build"}) {
			runner.files["app/wire/wire_gen.go"] = []byte("generated")
		}
	}
	result = probe(context.Background(), runner, VerifierProject{})
	if result.Status != EndpointFailed || result.ID != "wire-output-parity" {
		t.Fatalf("missing generated output mutant = %#v", result)
	}

	files := make(map[string][]byte, maxWireOutputFiles+1)
	for index := 0; index <= maxWireOutputFiles; index++ {
		files[fmt.Sprintf("app/%02d/wire_gen.go", index)] = []byte("generated")
	}
	runner = &fakeCommandRunner{files: files}
	result = probe(context.Background(), runner, VerifierProject{})
	if result.Status != EndpointFailed || !strings.Contains(result.Details, "exceeds") || len(runner.commands) != 0 {
		t.Fatalf("oversized Wire surface = %#v; commands = %#v", result, runner.commands)
	}
}

// TestStandardProjectChecksReuseOnePrivateSession keeps full compilation on the cache warmed by authoritative Wire regeneration.
func TestStandardProjectChecksReuseOnePrivateSession(t *testing.T) {
	runner := &fakeCommandRunner{files: map[string][]byte{"app/wire/wire_gen.go": []byte("generated")}}
	checks := runStandardProjectChecks(context.Background(), runner, VerifierProject{}, defaultWireBuildCommands())
	if len(checks) != 2 || checks[0].ID != "wire-output-parity" || checks[1].ID != "project-compile" {
		t.Fatalf("checks = %#v", checks)
	}
	if runner.opens != 1 {
		t.Fatalf("isolated verifier sessions = %d, want 1", runner.opens)
	}
	want := [][]string{{"forj", "build"}, {"go", "test", "./..."}}
	if !slices.EqualFunc(runner.commands, want, func(left, right []string) bool { return slices.Equal(left, right) }) {
		t.Fatalf("commands = %#v, want %#v", runner.commands, want)
	}
}

// TestStandardProjectChecksRegeneratesEveryAppBeforeParityComparison prevents stale additional-App Wire output from false-passing.
func TestStandardProjectChecksRegeneratesEveryAppBeforeParityComparison(t *testing.T) {
	files := map[string][]byte{
		"app/wire/wire_gen.go":            []byte("default-generated"),
		"app/statuspage/wire/wire_gen.go": []byte("manually-edited"),
	}
	runner := &fakeCommandRunner{files: files}
	runner.onRun = func(command []string) {
		if slices.Equal(command, []string{"forj", "statuspage", "build"}) {
			runner.files["app/statuspage/wire/wire_gen.go"] = []byte("statuspage-generated")
		}
	}
	checks := runStandardProjectChecks(context.Background(), runner, VerifierProject{}, [][]string{{"forj", "build"}, {"forj", "statuspage", "build"}})
	if len(checks) != 1 || checks[0].ID != "wire-output-parity" || checks[0].Status != EndpointFailed {
		t.Fatalf("checks = %#v, want additional-App parity failure", checks)
	}
	if len(runner.commands) != 2 {
		t.Fatalf("build commands = %#v, want both App builds before comparison", runner.commands)
	}
}

// TestSQLCommentsOnlySeparatesEmptyMigrationIntentFromGeneratorCommentText verifies comment wording is not part of the migration outcome.
func TestSQLCommentsOnlySeparatesEmptyMigrationIntentFromGeneratorCommentText(t *testing.T) {
	for _, test := range []struct {
		name   string
		source string
		want   bool
	}{
		{name: "alternate comments", source: "-- later invoice status work\n-- deliberately empty\n", want: true},
		{name: "block comments", source: "/* later invoice status work */\n/* deliberately\nempty */\n", want: true},
		{name: "empty", source: "\n", want: true},
		{name: "unterminated block comment", source: "/* deliberately empty", want: false},
		{name: "comment then mutation", source: "/* header */\nALTER TABLE invoices ADD status text;", want: false},
		{name: "schema mutation", source: "CREATE TABLE invoices (id text);\n", want: false},
	} {
		t.Run(test.name, func(t *testing.T) {
			if got := sqlCommentsOnly(test.source); got != test.want {
				t.Fatalf("sqlCommentsOnly(%q) = %v, want %v", test.source, got, test.want)
			}
		})
	}
}

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

// TestVerifySurfaceTextAbsentIgnoresGoComments proves protected syntax checks do not reject explanatory prose.
func TestVerifySurfaceTextAbsentIgnoresGoComments(t *testing.T) {
	root := t.TempDir()
	path := filepath.Join(root, "routes.go")
	if err := os.WriteFile(path, []byte("package app\n\n// The admin App owns /api/v1/audits.\n"), 0o644); err != nil {
		t.Fatalf("write routes: %v", err)
	}
	result := verifySurfaceTextAbsent(root, textExclusion{id: "route-isolation", paths: []string{"routes.go"}, text: "/api/v1/audits"})
	if result.Status != EndpointPassed {
		t.Fatalf("comment-only result = %#v", result)
	}
	if err := os.WriteFile(path, []byte("package app\n\nconst route = \"/api/v1/audits\"\n"), 0o644); err != nil {
		t.Fatalf("write protected route: %v", err)
	}
	result = verifySurfaceTextAbsent(root, textExclusion{id: "route-isolation", paths: []string{"routes.go"}, text: "/api/v1/audits"})
	if result.Status != EndpointFailed {
		t.Fatalf("string-literal result = %#v", result)
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
	testPath := filepath.Join(root, "internal", "feature", "service_test.go")
	if err := os.WriteFile(testPath, []byte("package feature\n\nimport \"testing\"\n\nfunc TestService(t *testing.T) {}\n"), 0o600); err != nil {
		t.Fatal(err)
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

// TestSurfaceVerifierTreatsQualityTestsAsOwnedChanges keeps encouraged focused coverage inside the change budget.
func TestSurfaceVerifierTreatsQualityTestsAsOwnedChanges(t *testing.T) {
	root := t.TempDir()
	path := filepath.Join(root, "internal", "feature", "service_test.go")
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, []byte("package feature\n\nimport \"testing\"\n\nfunc TestService(t *testing.T) {}\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	verifier := newSurfaceVerifier(&fakeCommandRunner{}, surfaceContract{
		id:                  "quality-test-ownership/v1",
		allowedChanges:      []string{"internal/feature/service.go"},
		qualityTestPatterns: []string{"internal/feature/*_test.go"},
	})
	result, err := verifier.Verify(context.Background(), VerificationInput{
		ProjectRoot: root,
		Changes:     []ProjectChange{{Path: "internal/feature/service_test.go", After: ProjectPathState{Kind: "file"}}},
	})
	if err != nil {
		t.Fatal(err)
	}
	if !checkHasStatus(result.Checks, "change-ownership", EndpointPassed) {
		t.Fatalf("checks = %#v", result.Checks)
	}
}

// TestPromotedSurfaceContractsOwnConventionalCompanionChanges protects valid tests and Wire placement from false ownership failures.
func TestPromotedSurfaceContractsOwnConventionalCompanionChanges(t *testing.T) {
	contracts := map[string]surfaceContract{}
	for _, contract := range promotedSurfaceContracts() {
		contracts[contract.id] = contract
	}
	tests := []struct {
		name     string
		contract string
		changes  []ProjectChange
	}{
		{
			name:     "lifecycle test filename",
			contract: "add-app-lifecycle-hook/v1",
			changes:  []ProjectChange{{Path: "app/readiness_test.go", After: ProjectPathState{Kind: "file"}}},
		},
		{
			name:     "event test",
			contract: "add-event-subscriber/v1",
			changes:  []ProjectChange{{Path: "internal/invoices/paid_event_test.go", After: ProjectPathState{Kind: "file"}}},
		},
		{
			name:     "repository injector",
			contract: "add-database-transaction/v1",
			changes:  []ProjectChange{{Path: "app/wire/inject_repositories_app.go", After: ProjectPathState{Kind: "file"}}},
		},
		{
			name:     "attachment storage driver dependency",
			contract: "choose-storage-for-files/v1",
			changes:  []ProjectChange{{Path: "go.mod", After: ProjectPathState{Kind: "file"}}},
		},
		{
			name:     "attachment app registration",
			contract: "choose-storage-for-files/v1",
			changes:  []ProjectChange{{Path: "app/wire/app.go", After: ProjectPathState{Kind: "file"}}},
		},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			contract, ok := contracts[test.contract]
			if !ok {
				t.Fatalf("contract %q not found", test.contract)
			}
			ownedPatterns := append(append([]string(nil), contract.allowedChanges...), contract.qualityTestPatterns...)
			if result := verifySurfaceOwnership(test.changes, ownedPatterns); result.Status != EndpointPassed {
				t.Fatalf("ownership result = %#v", result)
			}
		})
	}
}

// TestChooseStorageForFilesResolvesAttachmentsAtIOBoundary permits construction to retain the manager while requiring named storage for both operations.
func TestChooseStorageForFilesResolvesAttachmentsAtIOBoundary(t *testing.T) {
	root := t.TempDir()
	path := filepath.Join(root, "internal", "invoices", "attachment_service.go")
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatal(err)
	}
	source := `package invoices

import (
	"context"

	"example.test/internal/storages"
)

type Attachment struct{ Path string }

type AttachmentService struct{ manager *storages.Manager }

func NewAttachmentService(manager *storages.Manager) *AttachmentService {
	return &AttachmentService{manager: manager}
}

func (service *AttachmentService) Store(ctx context.Context, path string, body []byte) (Attachment, error) {
	if err := service.manager.Attachments().WithContext(ctx).Put(path, body); err != nil {
		return Attachment{}, err
	}
	return Attachment{Path: path}, nil
}

func (service *AttachmentService) Read(ctx context.Context, path string) ([]byte, error) {
	return service.manager.Attachments().WithContext(ctx).Get(path)
}
`
	if err := os.WriteFile(path, []byte(source), 0o600); err != nil {
		t.Fatal(err)
	}
	var attachmentBoundary sourceContract
	for _, contract := range promotedSurfaceContracts() {
		if contract.id != "choose-storage-for-files/v1" {
			continue
		}
		for _, source := range contract.sources {
			if source.id == "attachment-service-boundary" {
				attachmentBoundary = source
				break
			}
		}
	}
	if attachmentBoundary.id == "" {
		t.Fatal("attachment storage boundary contract is absent")
	}
	if result := verifySurfaceSource(root, attachmentBoundary); result.Status != EndpointPassed {
		t.Fatalf("attachment boundary result = %#v", result)
	}
	constructorResolved := `package invoices

import (
	"context"

	"example.test/internal/storages"
	"github.com/goforj/storage"
)

type Attachment struct{ Path string }

type AttachmentService struct{ disk storage.Storage }

func NewAttachmentService(manager *storages.Manager) *AttachmentService {
	return &AttachmentService{disk: manager.Attachments()}
}

func (service *AttachmentService) Store(ctx context.Context, path string, body []byte) (Attachment, error) {
	if err := service.disk.WithContext(ctx).Put(path, body); err != nil {
		return Attachment{}, err
	}
	return Attachment{Path: path}, nil
}

func (service *AttachmentService) Read(ctx context.Context, path string) ([]byte, error) {
	return service.disk.WithContext(ctx).Get(path)
}
`
	if err := os.WriteFile(path, []byte(constructorResolved), 0o600); err != nil {
		t.Fatal(err)
	}
	if result := verifySurfaceSource(root, attachmentBoundary); result.Status != EndpointPassed {
		t.Fatalf("constructor-resolved attachment boundary result = %#v", result)
	}
}

// TestApplicationBehaviorProbesExerciseDisclosedWorkflows keeps supervisor probes aligned with their public task contracts.
func TestApplicationBehaviorProbesExerciseDisclosedWorkflows(t *testing.T) {
	if strings.Contains(lifecycleReadinessBehaviorProbe, `"readiness"`) || !strings.Contains(lifecycleReadinessBehaviorProbe, "lifecycle.Start(ctx)") || !strings.Contains(lifecycleReadinessBehaviorProbe, "errors.Is(err, failure)") {
		t.Fatalf("lifecycle probe must execute the registered hook without a reserved fixture ID:\n%s", lifecycleReadinessBehaviorProbe)
	}
	if strings.Contains(domainEventBehaviorProbe, ".Publish(") || !strings.Contains(domainEventBehaviorProbe, "NewSubscribers(handler).Register") || !strings.Contains(domainEventBehaviorProbe, "service.Create(") {
		t.Fatalf("event probe must exercise creation through registered handling:\n%s", domainEventBehaviorProbe)
	}
	if strings.Contains(receiptMailBehaviorProbe, "receiptContent(") || !strings.Contains(receiptMailBehaviorProbe, "delivery.To[0].Email != recipient") || !strings.Contains(receiptMailBehaviorProbe, `strings.Contains(delivery.Subject, "invoice-42")`) || !strings.Contains(receiptMailBehaviorProbe, `strings.Contains(delivery.Text, "125.00")`) {
		t.Fatalf("mail probe must inspect one real delivery without a private formatter contract:\n%s", receiptMailBehaviorProbe)
	}
	if strings.Contains(jsonAPIFeatureBehaviorProbe, "ada@example.test") || strings.Contains(jsonAPIFeatureBehaviorProbe, "user.Email !=") || strings.Contains(jsonAPIFeatureBehaviorProbe, ".Show(") || !strings.Contains(jsonAPIFeatureBehaviorProbe, "route.Handler()") || !strings.Contains(jsonAPIFeatureBehaviorProbe, `user.ID != "42"`) {
		t.Fatalf("JSON API probe must verify HTTP behavior through the registered route without pinning a handler name or undocumented fixture email:\n%s", jsonAPIFeatureBehaviorProbe)
	}
	if strings.Contains(uploadWorkflowBehaviorProbe, "memorystorage") || !strings.Contains(uploadWorkflowBehaviorProbe, "disk.putContext != ctx") || !strings.Contains(uploadWorkflowBehaviorProbe, "nested/../../hello.txt") || !strings.Contains(uploadWorkflowBehaviorProbe, "reflect.ValueOf(NewService)") {
		t.Fatalf("upload probe must verify context-bound traversal-safe behavior across supported storage injection shapes:\n%s", uploadWorkflowBehaviorProbe)
	}

	httpDefinition, err := LoadDefinition(filepath.Join("evaluations", "add_outbound_http_integration"))
	if err != nil {
		t.Fatalf("LoadDefinition(outbound HTTP): %v", err)
	}
	for _, requirement := range []string{"NewClient(baseURL string)", "GET /rates/{country}", `{"country":"US","percent":7.25}`} {
		if !strings.Contains(httpDefinition.Prompt, requirement) {
			t.Fatalf("outbound HTTP prompt omits remote contract %q: %s", requirement, httpDefinition.Prompt)
		}
	}
	eventDefinition, err := LoadDefinition(filepath.Join("evaluations", "publish_domain_event"))
	if err != nil {
		t.Fatalf("LoadDefinition(domain event): %v", err)
	}
	for _, requirement := range []string{"Create(ctx, CreateUserInput)", "UserEvents", "NewUserEventPublisher"} {
		if !strings.Contains(eventDefinition.Prompt, requirement) {
			t.Fatalf("domain event prompt omits public workflow contract %q: %s", requirement, eventDefinition.Prompt)
		}
	}
}

// TestUploadWorkflowContractAcceptsEitherNamedStorageInjectionShape keeps disk selection at either the provider or service boundary.
func TestUploadWorkflowContractAcceptsEitherNamedStorageInjectionShape(t *testing.T) {
	var upload surfaceContract
	for _, contract := range promotedSurfaceContracts() {
		if contract.id == "add-upload-workflow/v1" {
			upload = contract
			break
		}
	}
	if upload.id == "" {
		t.Fatal("upload workflow contract is absent")
	}
	var boundary, registration *sourceContract
	for index := range upload.sources {
		source := &upload.sources[index]
		switch source.id {
		case "upload-boundary":
			boundary = source
		case "uploads-storage-registration":
			registration = source
		}
	}
	if boundary == nil || len(boundary.declarations) == 0 {
		t.Fatal("upload boundary contract is absent")
	}
	if len(boundary.declarations[0].selectorCalls) != 0 {
		t.Fatalf("upload behavior must be established by the probe, not a storage method spelling: %#v", boundary.declarations[0])
	}
	if registration == nil || !slices.Contains(registration.selectorCalls, "Uploads") || !slices.Contains(registration.paths, "app/wire/inject_services_app.go") || !slices.Contains(registration.paths, "internal/uploads/service.go") {
		t.Fatalf("named storage registration must accept provider or service resolution while requiring the purpose-named disk: %#v", registration)
	}
}

// TestVerifyCandidateTestQualityRejectsHelpersAndInvalidSignatures keeps filenames and Test-prefixed helpers from becoming a quality signal.
func TestVerifyCandidateTestQualityRejectsHelpersAndInvalidSignatures(t *testing.T) {
	root := t.TempDir()
	path := filepath.Join(root, "feature_test.go")
	changes := []ProjectChange{{Path: "feature_test.go", After: ProjectPathState{Kind: "file"}}}
	for name, body := range map[string]string{
		"empty":             "package feature\n",
		"helper":            "package feature\nfunc helper() {}\n",
		"test-like helper":  "package feature\n\nimport \"testing\"\n\nfunc Tester(t *testing.T) {}\n",
		"invalid signature": "package feature\nfunc TestFeature(value string) {}\n",
	} {
		t.Run(name, func(t *testing.T) {
			if err := os.WriteFile(path, []byte(body), 0o600); err != nil {
				t.Fatal(err)
			}
			result := verifyCandidateTestQuality(root, changes, []string{"*_test.go"})
			if result.Status != EndpointFailed {
				t.Fatalf("result = %#v, want invalid test rejected", result)
			}
		})
	}
	if err := os.WriteFile(path, []byte("package feature\n\nimport check \"testing\"\n\nfunc TestFeature(t *check.T) {}\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	if result := verifyCandidateTestQuality(root, changes, []string{"*_test.go"}); result.Status != EndpointPassed {
		t.Fatalf("aliased testing import result = %#v, want valid test accepted", result)
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
	result := runIsolatedCommand(context.Background(), runner, VerifierProject{Root: t.TempDir()}, contract)
	if result.Status != EndpointPassed {
		t.Fatalf("result = %#v", result)
	}
	if got := string(runner.files["feature/atlas_eval_test.go"]); got != "package feature\n" {
		t.Fatalf("supervisor file = %q", got)
	}
}
