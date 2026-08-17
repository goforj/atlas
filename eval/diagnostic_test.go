package eval

import (
	"context"
	"encoding/json"
	"errors"
	"os"
	"strings"
	"syscall"
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
	if result.Attempts[0].Result.PreparedTree != result.Attempts[1].Result.PreparedTree {
		t.Fatalf("paired prepared trees differ: %#v", result.Attempts)
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

// TestRunGuidanceDiagnosticRetainsUnserializedAttemptCause keeps operational error identity available to callers without adding it to reports.
func TestRunGuidanceDiagnosticRetainsUnserializedAttemptCause(t *testing.T) {
	runner, _, _, _, agent := newFakeRunner(t)
	cause := errors.New("command fatal")
	agent.session.waitErr = cause
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
	if err != nil {
		t.Fatalf("RunGuidanceDiagnostic(): %v", err)
	}
	if len(result.Attempts) != 2 || !errors.Is(result.Attempts[0].Cause, cause) || !errors.Is(result.Attempts[1].Cause, cause) {
		t.Fatalf("attempts = %#v, want both underlying causes", result.Attempts)
	}
	body, err := json.Marshal(result.Attempts[0])
	if err != nil {
		t.Fatal(err)
	}
	if strings.Contains(string(body), "Cause") || strings.Contains(string(body), "cause\":") {
		t.Fatalf("serialized attempt exposed Cause: %s", body)
	}
}

// TestRunGuidanceDiagnosticStopsAfterResourceExhaustion avoids consuming the shared disk again for the second treatment.
func TestRunGuidanceDiagnosticStopsAfterResourceExhaustion(t *testing.T) {
	runner, _, preparer, _, agent := newFakeRunner(t)
	agent.session.waitErr = &os.PathError{Op: "write", Path: "artifact", Err: syscall.ENOSPC}
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
	if !errors.Is(err, syscall.ENOSPC) {
		t.Fatalf("RunGuidanceDiagnostic() error = %v, want ENOSPC", err)
	}
	if len(result.Attempts) != 1 || !errors.Is(result.Attempts[0].Cause, syscall.ENOSPC) {
		t.Fatalf("attempts = %#v, want only the exhausted control treatment", result.Attempts)
	}
	if preparer.prepareCalls != 1 || agent.startCalls != 1 {
		t.Fatalf("resource exhaustion ran another treatment: prepare=%d start=%d", preparer.prepareCalls, agent.startCalls)
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

// TestRunGuidanceDiagnosticRejectsDifferentPairIdentities prevents comparisons across changed provider authority.
func TestRunGuidanceDiagnosticRejectsDifferentPairIdentities(t *testing.T) {
	runner, _, _, _, agent := newFakeRunner(t)
	agent.sessionIdentities = []AgentSessionIdentity{
		{Version: "fake-agent/1", Model: "fake-model", ModelProvider: "fake-provider", AuthorityDigest: "sha256:first"},
		{Version: "fake-agent/1", Model: "fake-model", ModelProvider: "fake-provider", AuthorityDigest: "sha256:second"},
	}
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
	if err == nil || !strings.Contains(err.Error(), "different agent or provider identities") || len(result.Attempts) != 2 {
		t.Fatalf("RunGuidanceDiagnostic() = (%#v, %v), want pair identity failure", result, err)
	}
}

// TestRunGuidanceDiagnosticRejectsIncompleteOrReusedProviderSessions prevents paired treatments from sharing or omitting model-context identities.
func TestRunGuidanceDiagnosticRejectsIncompleteOrReusedProviderSessions(t *testing.T) {
	tests := []struct {
		name       string
		identities []AgentSessionIdentity
	}{
		{
			name: "both missing",
			identities: []AgentSessionIdentity{
				{Version: "fake-agent/1", Model: "fake-model", ModelProvider: "fake-provider", AuthorityDigest: "sha256:authority"},
				{Version: "fake-agent/1", Model: "fake-model", ModelProvider: "fake-provider", AuthorityDigest: "sha256:authority"},
			},
		},
		{
			name: "control missing",
			identities: []AgentSessionIdentity{
				{Version: "fake-agent/1", Model: "fake-model", ModelProvider: "fake-provider", AuthorityDigest: "sha256:authority"},
				{Version: "fake-agent/1", Model: "fake-model", ModelProvider: "fake-provider", AuthorityDigest: "sha256:authority", SessionDigest: "sha256:treatment"},
			},
		},
		{
			name: "treatment missing",
			identities: []AgentSessionIdentity{
				{Version: "fake-agent/1", Model: "fake-model", ModelProvider: "fake-provider", AuthorityDigest: "sha256:authority", SessionDigest: "sha256:control"},
				{Version: "fake-agent/1", Model: "fake-model", ModelProvider: "fake-provider", AuthorityDigest: "sha256:authority"},
			},
		},
		{
			name: "reused",
			identities: []AgentSessionIdentity{
				{Version: "fake-agent/1", Model: "fake-model", ModelProvider: "fake-provider", AuthorityDigest: "sha256:authority", SessionDigest: "sha256:reused"},
				{Version: "fake-agent/1", Model: "fake-model", ModelProvider: "fake-provider", AuthorityDigest: "sha256:authority", SessionDigest: "sha256:reused"},
			},
		},
	}
	definition, err := LoadPromotedDefinition("add-http-controller")
	if err != nil {
		t.Fatal(err)
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			runner, _, _, _, agent := newFakeRunner(t)
			agent.sessionIdentities = test.identities
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
			if err == nil || !strings.Contains(err.Error(), "distinct non-empty provider session identities") || len(result.Attempts) != 2 {
				t.Fatalf("RunGuidanceDiagnostic() = (%#v, %v), want provider-session identity failure", result, err)
			}
		})
	}
}
