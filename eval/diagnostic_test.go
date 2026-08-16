package eval

import (
	"context"
	"os"
	"testing"
)

// TestRunGuidanceDiagnosticRunsBothIsolatedTreatments verifies the first product loop is one paired logical trial.
func TestRunGuidanceDiagnosticRunsBothIsolatedTreatments(t *testing.T) {
	runner, _, preparer, _, _ := newFakeRunner(t)
	definition, err := LoadPromotedDefinition("add-http-controller")
	if err != nil {
		t.Fatal(err)
	}
	result, err := runner.RunGuidanceDiagnostic(nil, GuidanceDiagnosticRequest{
		LogicalTrialID:  "diagnostic-01",
		Definition:      definition,
		DestinationRoot: "/private/projects",
		ForjExecutable:  "/tools/forj",
		Environments: map[string][]string{
			GuidanceProfileNone:   os.Environ(),
			GuidanceProfileAgents: os.Environ(),
		},
	})
	if err != nil {
		t.Fatalf("RunGuidanceDiagnostic(): %v", err)
	}
	if len(result.Attempts) != 2 || result.Attempts[0].Profile != GuidanceProfileNone || result.Attempts[1].Profile != GuidanceProfileAgents {
		t.Fatalf("attempts = %#v", result.Attempts)
	}
	for _, attempt := range result.Attempts {
		if attempt.Error != "" || attempt.Result.LogicalTrialID != result.LogicalTrialID || attempt.Result.EvaluationStatus != EvaluationDiagnostic || attempt.Result.GuidanceProfile != attempt.Profile {
			t.Fatalf("attempt = %#v", attempt)
		}
	}
	if result.Attempts[0].Result.BaselineTree != result.Attempts[1].Result.BaselineTree {
		t.Fatalf("paired baselines differ: %#v", result.Attempts)
	}
	if preparer.resolveCalls != 2 || preparer.prepareCalls != 2 {
		t.Fatalf("preparer calls = resolve:%d prepare:%d", preparer.resolveCalls, preparer.prepareCalls)
	}
}

// TestRunGuidanceDiagnosticValidatesBothEnvironmentsBeforeMutation keeps a malformed pair atomic at preflight.
func TestRunGuidanceDiagnosticValidatesBothEnvironmentsBeforeMutation(t *testing.T) {
	runner, _, preparer, _, _ := newFakeRunner(t)
	definition, err := LoadPromotedDefinition("add-http-controller")
	if err != nil {
		t.Fatal(err)
	}
	_, err = runner.RunGuidanceDiagnostic(context.Background(), GuidanceDiagnosticRequest{
		LogicalTrialID:  "diagnostic-01",
		Definition:      definition,
		DestinationRoot: "/private/projects",
		ForjExecutable:  "/tools/forj",
		Environments: map[string][]string{
			GuidanceProfileNone: os.Environ(),
		},
	})
	if err == nil {
		t.Fatal("RunGuidanceDiagnostic() accepted an incomplete environment pair")
	}
	if preparer.resolveCalls != 0 || preparer.prepareCalls != 0 {
		t.Fatalf("preparer mutated after invalid preflight: resolve=%d prepare=%d", preparer.resolveCalls, preparer.prepareCalls)
	}
}

// TestRunGuidanceDiagnosticRejectsDifferentPlans prevents a paired comparison across different fixture resolutions.
func TestRunGuidanceDiagnosticRejectsDifferentPlans(t *testing.T) {
	runner, _, preparer, _, _ := newFakeRunner(t)
	preparer.planDigests = []string{"sha256:control-plan", "sha256:treatment-plan"}
	definition, err := LoadPromotedDefinition("add-http-controller")
	if err != nil {
		t.Fatal(err)
	}
	result, err := runner.RunGuidanceDiagnostic(context.Background(), GuidanceDiagnosticRequest{
		LogicalTrialID:  "diagnostic-01",
		Definition:      definition,
		DestinationRoot: "/private/projects",
		ForjExecutable:  "/tools/forj",
		Environments: map[string][]string{
			GuidanceProfileNone:   os.Environ(),
			GuidanceProfileAgents: os.Environ(),
		},
	})
	if err == nil || len(result.Attempts) != 2 {
		t.Fatalf("RunGuidanceDiagnostic() = (%#v, %v), want completed attempts and parity error", result, err)
	}
}
