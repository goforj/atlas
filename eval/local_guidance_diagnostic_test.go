package eval

import (
	"context"
	"errors"
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

// TestLocalGuidanceDiagnosticRunsTreatmentBoundaryBetweenPairs lets hosts inspect their shared tools without rebuilding Atlas treatments.
func TestLocalGuidanceDiagnosticRunsTreatmentBoundaryBetweenPairs(t *testing.T) {
	runner, _, preparer, _, agent := newFakeRunner(t)
	diagnostic := LocalGuidanceDiagnostic{runner: runner, forjExecutable: "/tools/forj", runtime: fakeAttemptRequest().Runtime}
	boundaries := 0
	result, err := diagnostic.Run(context.Background(), LocalGuidanceDiagnosticRequest{
		EvaluationID:    "add-http-controller",
		DestinationRoot: t.TempDir(),
		Environments: map[string][]string{
			GuidanceProfileNone:   {},
			GuidanceProfileAgents: {},
		},
		LogicalTrialID: "paired-boundary",
		TreatmentBoundary: func(context.Context) error {
			boundaries++
			if preparer.prepareCalls != 1 || agent.startCalls != 1 {
				t.Fatalf("boundary ran at the wrong lifecycle point: prepare=%d start=%d", preparer.prepareCalls, agent.startCalls)
			}
			return nil
		},
	})
	if err != nil || boundaries != 1 || len(result.Attempts) != 2 {
		t.Fatalf("Run() = (%#v, %v), boundaries=%d", result, err, boundaries)
	}
}

// TestLocalGuidanceDiagnosticPreservesTreatmentBoundaryFailure stops before the second treatment while retaining the host's cause.
func TestLocalGuidanceDiagnosticPreservesTreatmentBoundaryFailure(t *testing.T) {
	runner, _, preparer, _, _ := newFakeRunner(t)
	diagnostic := LocalGuidanceDiagnostic{runner: runner, forjExecutable: "/tools/forj", runtime: fakeAttemptRequest().Runtime}
	cause := errors.New("shared tool verification failed")
	result, err := diagnostic.Run(context.Background(), LocalGuidanceDiagnosticRequest{
		EvaluationID:    "add-http-controller",
		DestinationRoot: t.TempDir(),
		Environments: map[string][]string{
			GuidanceProfileNone:   {},
			GuidanceProfileAgents: {},
		},
		LogicalTrialID:    "paired-boundary-failure",
		TreatmentBoundary: func(context.Context) error { return cause },
	})
	if !errors.Is(err, cause) || len(result.Attempts) != 1 || preparer.prepareCalls != 1 {
		t.Fatalf("Run() = (%#v, %v), prepare=%d", result, err, preparer.prepareCalls)
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
