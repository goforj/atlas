package eval

import (
	"reflect"
	"testing"
)

// TestEvaluateWorkflowRequiresSuccessfulTrustedGenerator proves spelling alone cannot satisfy generator conformance.
func TestEvaluateWorkflowRequiresSuccessfulTrustedGenerator(t *testing.T) {
	workflow := PromotedWorkflows()[0]
	matchingStart := Event{Kind: EventCommandStarted, Fields: map[string]string{
		EventFieldCommandID:        "command-1",
		EventFieldExecutableDigest: "sha256:forj",
		EventFieldArguments:        `["make:controller","invoices"]`,
	}}
	matchingFinish := Event{Kind: EventCommandFinished, Fields: map[string]string{
		EventFieldCommandID: "command-1",
		EventFieldExitCode:  "0",
	}}

	tests := []struct {
		name       string
		events     []Event
		digest     string
		wantStatus EndpointStatus
	}{
		{name: "matching success", events: []Event{matchingStart, matchingFinish}, digest: "sha256:forj", wantStatus: EndpointPassed},
		{name: "copied executable", events: []Event{matchingStart, matchingFinish}, digest: "sha256:other", wantStatus: EndpointFailed},
		{name: "wrong resource", events: []Event{{Kind: EventCommandStarted, Fields: map[string]string{EventFieldCommandID: "command-1", EventFieldExecutableDigest: "sha256:forj", EventFieldArguments: `["make:controller","payments"]`}}, matchingFinish}, digest: "sha256:forj", wantStatus: EndpointFailed},
		{name: "failed command", events: []Event{matchingStart, {Kind: EventCommandFinished, Fields: map[string]string{EventFieldCommandID: "command-1", EventFieldExitCode: "1"}}}, digest: "sha256:forj", wantStatus: EndpointFailed},
		{name: "missing completion", events: []Event{matchingStart}, digest: "sha256:forj", wantStatus: EndpointFailed},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			result, checks := EvaluateWorkflow(workflow, test.events, test.digest, []Capability{CapabilityCommands})
			if result.Status != test.wantStatus || len(checks) != 1 || checks[0].Status != test.wantStatus {
				t.Fatalf("EvaluateWorkflow() = %#v, %#v", result, checks)
			}
		})
	}
}

// TestEvaluateWorkflowMarksUnavailableEvidenceIneligible preserves diagnostic outcome checks without inventing conformance proof.
func TestEvaluateWorkflowMarksUnavailableEvidenceIneligible(t *testing.T) {
	result, checks := EvaluateWorkflow(PromotedWorkflows()[0], nil, "sha256:forj", nil)
	if result.Status != EndpointIneligible || len(checks) != 1 || checks[0].Status != EndpointIneligible {
		t.Fatalf("EvaluateWorkflow() = %#v, %#v", result, checks)
	}
}

// TestPromotedWorkflowReturnsCopies prevents tests or callers from mutating a later evaluation's command contract.
func TestPromotedWorkflowReturnsCopies(t *testing.T) {
	first := PromotedWorkflows()
	first[0].Generators[0].Arguments[1] = "payments"
	second := PromotedWorkflows()
	if reflect.DeepEqual(first, second) || second[0].Generators[0].Arguments[1] != "invoices" {
		t.Fatalf("promoted workflow was mutated: %#v", second)
	}
}
