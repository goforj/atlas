package eval

import (
	"strings"
	"testing"
)

// TestAttemptReportProjectionsSeparateHumanAndMachineEvidence verifies useful diagnostics without environment-value retention.
func TestAttemptReportProjectionsSeparateHumanAndMachineEvidence(t *testing.T) {
	secret := "must-not-appear"
	request := fakeAttemptRequest()
	request.Intent = IntentDiagnostic
	request.Preparation.Environment = []string{"PATH=/tools", "API_TOKEN=" + secret}
	result := AttemptResult{
		EvaluationID:        "add-http-controller",
		GuidanceProfile:     GuidanceProfileAgents,
		Agent:               "codex",
		Model:               "test-model",
		AgentOutcome:        AgentCompleted,
		EvaluationStatus:    EvaluationValid,
		UnavailableEvidence: []Capability{CapabilityCommands},
		Verification: &VerificationResult{
			FrameworkOutcome:    EndpointResult{ID: "framework", Status: EndpointPassed},
			WorkflowConformance: EndpointResult{ID: "workflow", Status: EndpointIneligible, Details: "trusted command evidence is unavailable"},
		},
	}
	summary := attemptSummary(request, result)
	for _, value := range []string{"Agent completed", "Result verified", "Framework outcome: passed", "Workflow conformance: ineligible", "Diagnostic limitation:", "retained evidence supports diagnosis only", "a fresh stochastic attempt, not a replay"} {
		if !strings.Contains(summary, value) {
			t.Fatalf("summary %q does not contain %q", summary, value)
		}
	}
	keys := environmentKeys(request.Preparation.Environment)
	if strings.Contains(strings.Join(keys, ","), secret) || strings.Join(keys, ",") != "API_TOKEN,PATH" {
		t.Fatalf("environment keys = %q", keys)
	}
	events := []Event{
		{Sequence: 1, Kind: EventCommandStarted, Fields: map[string]string{"command": "forj build", "evidence": "provider-telemetry"}},
		{Sequence: 2, Kind: EventMessage, Fields: map[string]string{"text": "Implemented the route."}},
	}
	if !strings.Contains(attemptCommands(events), "provider-telemetry") {
		t.Fatalf("commands = %q", attemptCommands(events))
	}
	if attemptTranscript(events) != "Implemented the route.\n" {
		t.Fatalf("transcript = %q", attemptTranscript(events))
	}
}

// TestAttemptSummaryIncludesProviderSessionIdentity makes the provider session available when comparing retained attempts.
func TestAttemptSummaryIncludesProviderSessionIdentity(t *testing.T) {
	result := AttemptResult{ProviderSessionDigest: "sha256:session"}
	summary := attemptSummary(fakeAttemptRequest(), result)
	if !strings.Contains(summary, "Provider session: sha256:session") {
		t.Fatalf("summary does not contain provider session identity: %q", summary)
	}
}

// TestAttemptSummaryListsFailedChecksAndUsesActualBackend verifies reports retain actionable failures without assuming one backend name.
func TestAttemptSummaryListsFailedChecksAndUsesActualBackend(t *testing.T) {
	request := fakeAttemptRequest()
	request.Intent = IntentDiagnostic
	result := AttemptResult{
		Backend:          "sandboxed-local",
		EvaluationStatus: EvaluationDiagnostic,
		Verification: &VerificationResult{
			FrameworkOutcome:    EndpointResult{ID: "framework", Status: EndpointFailed, Details: "route is missing"},
			WorkflowConformance: EndpointResult{ID: "workflow", Status: EndpointFailed},
			Checks:              []EndpointResult{{ID: "generator", Status: EndpointFailed, Details: "not observed"}},
		},
	}
	summary := attemptSummary(request, result)
	for _, value := range []string{"Diagnostic result only", "Failed checks: framework: route is missing; workflow; generator: not observed", `backend "sandboxed-local"`} {
		if !strings.Contains(summary, value) {
			t.Fatalf("summary %q does not contain %q", summary, value)
		}
	}
	if strings.Contains(summary, "unconfined local") {
		t.Fatalf("summary hardcodes backend wording: %q", summary)
	}
}

// TestAttemptSummarySeparatesQualitySignals keeps non-gating engineering feedback distinct from correctness failures.
func TestAttemptSummarySeparatesQualitySignals(t *testing.T) {
	request := fakeAttemptRequest()
	result := AttemptResult{
		EvaluationStatus: EvaluationValid,
		Verification: &VerificationResult{
			FrameworkOutcome:    EndpointResult{ID: "framework", Status: EndpointPassed},
			WorkflowConformance: EndpointResult{ID: "workflow", Status: EndpointPassed},
			Checks: []EndpointResult{
				{ID: "focused-tests-added", Kind: RequirementQuality, Status: EndpointFailed, Details: "no focused test changed"},
				{ID: "build", Status: EndpointPassed},
			},
		},
	}
	summary := attemptSummary(request, result)
	if strings.Contains(summary, "Failed checks:") {
		t.Fatalf("summary treats quality as correctness failure: %q", summary)
	}
	if !strings.Contains(summary, "Quality signals: focused-tests-added=failed: no focused test changed") {
		t.Fatalf("summary omits quality signal: %q", summary)
	}
}
