package eval

import (
	"context"
	"path/filepath"
	"testing"
)

// TestNewLocalGuidanceDiagnosticOwnsStandardWiring keeps hosts from rebuilding Atlas policy collaborators around every command.
func TestNewLocalGuidanceDiagnosticOwnsStandardWiring(t *testing.T) {
	_, _, preparer, _, _ := newFakeRunner(t)
	credential, err := NewCodexCredential([]byte(`{"access_token":"local-diagnostic-secret"}`))
	if err != nil {
		t.Fatal(err)
	}
	diagnostic, err := NewLocalGuidanceDiagnostic(LocalGuidanceDiagnosticOptions{
		WorkRoot:       t.TempDir(),
		ArtifactRoot:   filepath.Join(t.TempDir(), "artifacts"),
		ArtifactKey:    []byte("0123456789abcdef0123456789abcdef"),
		Preparer:       preparer,
		Codex:          CodexOptions{Executable: "codex", Model: "gpt-test", Credential: credential},
		GoExecutable:   "go",
		ForjExecutable: "/tools/forj",
		Runtime:        fakeAttemptRequest().Runtime,
	})
	if err != nil {
		t.Fatalf("NewLocalGuidanceDiagnostic(): %v", err)
	}
	if diagnostic.runner.Registry == nil || diagnostic.runner.Preparer != preparer || diagnostic.runner.Agent == nil || diagnostic.runner.Artifacts == nil {
		t.Fatalf("diagnostic did not retain the standard Atlas wiring: %#v", diagnostic.runner)
	}
	if _, ok := diagnostic.runner.Backend.(UnconfinedLocal); !ok {
		t.Fatalf("backend = %T, want UnconfinedLocal", diagnostic.runner.Backend)
	}
	if redacted := diagnostic.runner.Artifacts.redactor.Text("local-diagnostic-secret"); redacted != redactedValue {
		t.Fatalf("artifact redactor did not cover frozen authority: %q", redacted)
	}
	verifier := diagnostic.runner.Registry.verifiers["add-http-controller/v1"].(*AddHTTPControllerVerifier)
	commands := verifier.runner.(VerifierCommands)
	if commands.Environment == nil || len(commands.Environment) != 0 {
		t.Fatalf("default verifier environment = %#v, want an explicit clean environment", commands.Environment)
	}
}

// TestNewLocalGuidanceDiagnosticRejectsMissingHostBoundary prevents an incomplete host command from creating a partly trusted service.
func TestNewLocalGuidanceDiagnosticRejectsMissingHostBoundary(t *testing.T) {
	if _, err := NewLocalGuidanceDiagnostic(LocalGuidanceDiagnosticOptions{}); err == nil {
		t.Fatal("NewLocalGuidanceDiagnostic() accepted missing host boundaries")
	}
}

// TestLocalGuidanceDiagnosticRunsOneTreatment keeps the run command on the same Atlas lifecycle as paired comparisons.
func TestLocalGuidanceDiagnosticRunsOneTreatment(t *testing.T) {
	runner, _, _, _, _ := newFakeRunner(t)
	diagnostic := LocalGuidanceDiagnostic{runner: runner, forjExecutable: "/tools/forj", runtime: fakeAttemptRequest().Runtime}
	attempt, err := diagnostic.RunTreatment(context.Background(), LocalDiagnosticTreatmentRequest{
		EvaluationID:    "add-http-controller",
		GuidanceProfile: GuidanceProfileAgents,
		DestinationRoot: t.TempDir(),
		Environment:     []string{},
		LogicalTrialID:  "single-treatment",
	})
	if err != nil {
		t.Fatalf("RunTreatment(): %v", err)
	}
	if attempt.Profile != GuidanceProfileAgents || attempt.Result.LogicalTrialID != "single-treatment" || attempt.Result.AttemptID != "single-treatment-agents" {
		t.Fatalf("attempt = %#v", attempt)
	}
}

// TestLocalGuidanceDiagnosticRejectsUnsupportedTreatment prevents the diagnostic command from inventing unresolved guidance profiles.
func TestLocalGuidanceDiagnosticRejectsUnsupportedTreatment(t *testing.T) {
	runner, _, _, _, _ := newFakeRunner(t)
	diagnostic := LocalGuidanceDiagnostic{runner: runner, forjExecutable: "/tools/forj"}
	if _, err := diagnostic.RunTreatment(context.Background(), LocalDiagnosticTreatmentRequest{
		EvaluationID: "add-http-controller", GuidanceProfile: "skills", DestinationRoot: t.TempDir(), Environment: []string{},
	}); err == nil {
		t.Fatal("RunTreatment() accepted an unsupported guidance profile")
	}
}
