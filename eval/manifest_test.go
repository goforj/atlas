package eval

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

const validEvaluationManifest = `schema_version: 1
id: add-http-controller
summary: Add an HTTP controller for existing invoice behavior
suite: core
task_kind: scaffold
project_scenario: invoice-http-route
workflow: goforj-add-http-route/v1
verifier: add-http-controller/v1
limits:
  wall_time: 10m
  commands: 80
  shell_network: off
`

// testVerifier is a deterministic promoted contract used by registry tests.
type testVerifier struct {
	id           string
	capabilities []Capability
}

// ID returns the promoted verifier identity.
func (verifier testVerifier) ID() string {
	return verifier.id
}

// Capabilities returns the trusted observation classes required by the verifier.
func (verifier testVerifier) Capabilities() []Capability {
	return append([]Capability(nil), verifier.capabilities...)
}

// Verify returns stable passing endpoints because registry tests do not exercise verifier behavior.
func (verifier testVerifier) Verify(context.Context, VerificationInput) (VerificationResult, error) {
	return VerificationResult{
		FrameworkOutcome:    EndpointResult{ID: "framework", Status: EndpointPassed},
		WorkflowConformance: EndpointResult{ID: "workflow", Status: EndpointPassed},
	}, nil
}

// TestLoadDefinitionReadsMinimalContract verifies manifests join IDs and an adjacent prompt without duplicating policy.
func TestLoadDefinitionReadsMinimalContract(t *testing.T) {
	directory := t.TempDir()
	writeEvaluationFixture(t, directory, "evaluation.yaml", validEvaluationManifest)
	writeEvaluationFixture(t, directory, "prompt.md", "Add an endpoint that returns an invoice by ID.\n")

	definition, err := LoadDefinition(directory)
	if err != nil {
		t.Fatalf("LoadDefinition(): %v", err)
	}
	if definition.ID != "add-http-controller" || definition.TaskKind != TaskScaffold || definition.WorkflowID != "goforj-add-http-route/v1" || definition.VerifierID != "add-http-controller/v1" {
		t.Fatalf("resolved definition = %#v", definition)
	}
	if definition.Limits.WallTime.String() != "10m0s" || definition.Limits.Commands != 80 || definition.PromptDigest == "" {
		t.Fatalf("resolved limits and prompt = %#v", definition)
	}
}

// TestPromotedEvaluationIDsMatchingSeparatesMeasurementKinds proves focused runs do not rely on naming conventions.
func TestPromotedEvaluationIDsMatchingSeparatesMeasurementKinds(t *testing.T) {
	featureIDs, err := PromotedEvaluationIDsMatching(EvaluationFilter{Suite: "core", TaskKind: TaskFeature})
	if err != nil {
		t.Fatalf("PromotedEvaluationIDsMatching(): %v", err)
	}
	if len(featureIDs) == 0 {
		t.Fatal("feature selection returned no promoted evaluations")
	}
	for _, id := range featureIDs {
		definition, err := LoadPromotedDefinition(id)
		if err != nil {
			t.Fatalf("LoadPromotedDefinition(%q): %v", id, err)
		}
		if definition.Suite != "core" || definition.TaskKind != TaskFeature {
			t.Fatalf("selected definition = %#v", definition)
		}
	}
}

// TestDecodeEvaluationManifestRejectsAmbiguity proves misspelled or executable policy cannot silently enter evaluation data.
func TestDecodeEvaluationManifestRejectsAmbiguity(t *testing.T) {
	tests := []struct {
		name    string
		body    string
		wantErr string
	}{
		{name: "unknown version", body: strings.Replace(validEvaluationManifest, "schema_version: 1", "schema_version: 2", 1), wantErr: "unsupported schema_version 2"},
		{name: "unknown field", body: validEvaluationManifest + "requirements: []\n", wantErr: "field requirements not found"},
		{name: "multiple documents", body: validEvaluationManifest + "---\nid: hidden\n", wantErr: "multiple YAML documents"},
		{name: "duplicate field", body: validEvaluationManifest + "id: second\n", wantErr: "mapping key \"id\" already defined"},
		{name: "anchor", body: strings.Replace(validEvaluationManifest, "summary: Add", "summary: &summary Add", 1), wantErr: "aliases and anchors are not supported"},
		{name: "unversioned workflow", body: strings.Replace(validEvaluationManifest, "goforj-add-http-route/v1", "goforj-add-http-route", 1), wantErr: "must be a versioned contract ID"},
		{name: "invalid task kind", body: strings.Replace(validEvaluationManifest, "task_kind: scaffold", "task_kind: vague", 1), wantErr: "task_kind \"vague\" is invalid"},
		{name: "invalid duration", body: strings.Replace(validEvaluationManifest, "wall_time: 10m", "wall_time: someday", 1), wantErr: "must be a positive duration"},
		{name: "invalid command budget", body: strings.Replace(validEvaluationManifest, "commands: 80", "commands: 0", 1), wantErr: "limits.commands must be positive"},
		{name: "network broadening", body: strings.Replace(validEvaluationManifest, "shell_network: off", "shell_network: full", 1), wantErr: "limits.shell_network \"full\" is unsupported"},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			_, err := decodeEvaluationManifest([]byte(test.body))
			if err == nil || !strings.Contains(err.Error(), test.wantErr) {
				t.Fatalf("decodeEvaluationManifest() error = %v, want %q", err, test.wantErr)
			}
		})
	}
}

// TestRegistryResolvesPromotedContracts verifies required capabilities compose deterministically from typed owners.
func TestRegistryResolvesPromotedContracts(t *testing.T) {
	workflow := WorkflowExpectation{
		ID: "goforj-add-http-route/v1",
		Requirements: []WorkflowRequirement{
			{ID: "use-generator", Kind: RequirementWorkflow, Capability: CapabilityCommands},
			{ID: "inspect-project", Kind: RequirementQuality, Capability: CapabilityFileReads},
		},
	}
	verifier := testVerifier{id: "add-http-controller/v1", capabilities: []Capability{CapabilityFileWrites, CapabilityCommands}}
	registry, err := NewRegistry([]WorkflowExpectation{workflow}, []Verifier{verifier})
	if err != nil {
		t.Fatalf("NewRegistry(): %v", err)
	}
	definition, err := decodeEvaluationManifest([]byte(validEvaluationManifest))
	if err != nil {
		t.Fatalf("decode manifest: %v", err)
	}
	resolved, err := registry.Resolve(definition)
	if err != nil {
		t.Fatalf("Resolve(): %v", err)
	}
	got := []Capability{CapabilityCommands, CapabilityFileWrites}
	if len(resolved.Capabilities) != len(got) {
		t.Fatalf("capabilities = %q, want %q", resolved.Capabilities, got)
	}
	for index := range got {
		if resolved.Capabilities[index] != got[index] {
			t.Fatalf("capabilities = %q, want %q", resolved.Capabilities, got)
		}
	}
}

// TestRegistryRejectsInvalidPromotedContracts keeps hard requirements typed and unambiguous.
func TestRegistryRejectsInvalidPromotedContracts(t *testing.T) {
	tests := []struct {
		name      string
		workflows []WorkflowExpectation
		verifiers []Verifier
		wantErr   string
	}{
		{name: "duplicate workflow", workflows: []WorkflowExpectation{{ID: "workflow/v1"}, {ID: "workflow/v1"}}, wantErr: "duplicate workflow ID"},
		{name: "duplicate requirement", workflows: []WorkflowExpectation{{ID: "workflow/v1", Requirements: []WorkflowRequirement{{ID: "inspect", Kind: RequirementWorkflow, Capability: CapabilityFileReads}, {ID: "inspect", Kind: RequirementWorkflow, Capability: CapabilityFileReads}}}}, wantErr: "duplicate requirement ID"},
		{name: "duplicate generator requirement", workflows: []WorkflowExpectation{{ID: "workflow/v1", Requirements: []WorkflowRequirement{{ID: "generate", Kind: RequirementQuality, Capability: CapabilityCommands}}, Generators: []GeneratorRequirement{{ID: "generate", Arguments: []string{"make:controller", "invoices"}}}}}, wantErr: "duplicate requirement ID"},
		{name: "invalid generator arguments", workflows: []WorkflowExpectation{{ID: "workflow/v1", Generators: []GeneratorRequirement{{ID: "generate", Arguments: []string{"make:controller"}}}}}, wantErr: "requires structured arguments"},
		{name: "unobservable requirement", workflows: []WorkflowExpectation{{ID: "workflow/v1", Requirements: []WorkflowRequirement{{ID: "inspect", Kind: RequirementWorkflow}}}}, wantErr: "no observation capability"},
		{name: "unsafe requirement path", workflows: []WorkflowExpectation{{ID: "workflow/v1", Requirements: []WorkflowRequirement{{ID: "inspect", Kind: RequirementQuality, Capability: CapabilityFileReads, Paths: []string{"../outside"}}}}}, wantErr: "unsafe Project path pattern"},
		{name: "windows requirement path", workflows: []WorkflowExpectation{{ID: "workflow/v1", Requirements: []WorkflowRequirement{{ID: "inspect", Kind: RequirementQuality, Capability: CapabilityFileReads, Paths: []string{`C:\outside`}}}}}, wantErr: "unsafe Project path pattern"},
		{name: "invalid requirement path", workflows: []WorkflowExpectation{{ID: "workflow/v1", Requirements: []WorkflowRequirement{{ID: "inspect", Kind: RequirementQuality, Capability: CapabilityFileReads, Paths: []string{"internal/["}}}}}, wantErr: "invalid Project path pattern"},
		{name: "nil verifier", verifiers: []Verifier{nil}, wantErr: "nil verifier"},
		{name: "duplicate verifier", verifiers: []Verifier{testVerifier{id: "verify/v1"}, testVerifier{id: "verify/v1"}}, wantErr: "duplicate verifier ID"},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			_, err := NewRegistry(test.workflows, test.verifiers)
			if err == nil || !strings.Contains(err.Error(), test.wantErr) {
				t.Fatalf("NewRegistry() error = %v, want %q", err, test.wantErr)
			}
		})
	}
}

// TestRegistryRejectsMissingReferences ensures stale cross-contract IDs fail before Project mutation.
func TestRegistryRejectsMissingReferences(t *testing.T) {
	registry, err := NewRegistry(nil, nil)
	if err != nil {
		t.Fatalf("NewRegistry(): %v", err)
	}
	definition, err := decodeEvaluationManifest([]byte(validEvaluationManifest))
	if err != nil {
		t.Fatalf("decode manifest: %v", err)
	}
	if _, err := registry.Resolve(definition); err == nil || !strings.Contains(err.Error(), "is not promoted") {
		t.Fatalf("Resolve() error = %v, want missing promoted reference", err)
	}
}

// writeEvaluationFixture writes one contract file beneath a private test directory.
func writeEvaluationFixture(t *testing.T, directory, name, body string) {
	t.Helper()
	if err := os.WriteFile(filepath.Join(directory, name), []byte(body), 0o600); err != nil {
		t.Fatalf("write %s: %v", name, err)
	}
}
