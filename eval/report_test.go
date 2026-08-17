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
