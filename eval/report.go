package eval

import (
	"encoding/json"
	"fmt"
	"sort"
	"strings"
)

// AttemptScorecard is the compact machine-readable outcome layer retained beside full run metadata.
type AttemptScorecard struct {
	EvaluationStatus    EvaluationStatus `json:"evaluation_status"`
	AgentOutcome        AgentOutcome     `json:"agent_outcome"`
	FrameworkOutcome    *EndpointResult  `json:"framework_outcome,omitempty"`
	WorkflowConformance *EndpointResult  `json:"workflow_conformance,omitempty"`
	UnavailableEvidence []Capability     `json:"unavailable_evidence,omitempty"`
}

// AttemptEnvironment records names and negotiated capabilities without persisting environment values.
type AttemptEnvironment struct {
	Intent                RunIntent       `json:"intent"`
	Backend               string          `json:"backend"`
	RequestedShellNetwork string          `json:"requested_shell_network"`
	EnvironmentKeys       []string        `json:"environment_keys"`
	AgentCapabilities     []Capability    `json:"agent_capabilities,omitempty"`
	BackendCapabilities   []Capability    `json:"backend_capabilities,omitempty"`
	UnavailableEvidence   []Capability    `json:"unavailable_evidence,omitempty"`
	Runtime               RuntimeIdentity `json:"runtime"`
}

// writeAttemptReportArtifacts retains the human, diagnostic, and compact machine layers for every started attempt.
func writeAttemptReportArtifacts(artifacts *AttemptArtifacts, request AttemptRequest, result AttemptResult, events []Event, agentCapabilities, backendCapabilities []Capability) []SecondaryFailure {
	var failures []SecondaryFailure
	writeText := func(name, phase, content string) {
		if err := artifacts.WriteText(name, content); err != nil {
			failures = append(failures, SecondaryFailure{Phase: phase, Message: err.Error()})
		}
	}
	writeJSON := func(name, phase string, value any) {
		if err := artifacts.WriteJSON(name, value); err != nil {
			failures = append(failures, SecondaryFailure{Phase: phase, Message: err.Error()})
		}
	}
	writeText("summary.txt", "artifact_summary", attemptSummary(request, result))
	writeText("transcript.redacted.txt", "artifact_transcript", attemptTranscript(events))
	writeText("commands.jsonl", "artifact_commands", attemptCommands(events))
	writeJSON("scorecard.json", "artifact_scorecard", attemptScorecard(result))
	writeJSON("environment.json", "artifact_environment", AttemptEnvironment{
		Intent:                request.Intent,
		Backend:               result.Backend,
		RequestedShellNetwork: request.Definition.Limits.ShellNetwork,
		EnvironmentKeys:       environmentKeys(request.Preparation.Environment),
		AgentCapabilities:     sortedCapabilities(agentCapabilities),
		BackendCapabilities:   sortedCapabilities(backendCapabilities),
		UnavailableEvidence:   append([]Capability(nil), result.UnavailableEvidence...),
		Runtime:               result.Runtime,
	})
	return failures
}

// attemptScorecard projects only the outcomes needed to compare attempts without parsing full lifecycle metadata.
func attemptScorecard(result AttemptResult) AttemptScorecard {
	scorecard := AttemptScorecard{
		EvaluationStatus:    result.EvaluationStatus,
		AgentOutcome:        result.AgentOutcome,
		UnavailableEvidence: append([]Capability(nil), result.UnavailableEvidence...),
	}
	if result.Verification != nil {
		framework := result.Verification.FrameworkOutcome
		workflow := result.Verification.WorkflowConformance
		scorecard.FrameworkOutcome = &framework
		scorecard.WorkflowConformance = &workflow
	}
	return scorecard
}

// attemptSummary states diagnostic limitations directly instead of requiring enum interpretation.
func attemptSummary(request AttemptRequest, result AttemptResult) string {
	var summary strings.Builder
	fmt.Fprintf(&summary, "Evaluation: %s\n", result.EvaluationID)
	fmt.Fprintf(&summary, "Treatment: %s\n", result.GuidanceProfile)
	fmt.Fprintf(&summary, "Agent: %s", result.Agent)
	if result.AgentVersion != "" {
		fmt.Fprintf(&summary, " %s", result.AgentVersion)
	}
	if result.Model != "" {
		fmt.Fprintf(&summary, " · %s", result.Model)
	}
	summary.WriteByte('\n')
	if result.Runtime.Framework.Version != "" || result.Runtime.Supervisor.Version != "" {
		fmt.Fprintf(&summary, "Runtime: GoForj %s · Atlas %s · %s\n", result.Runtime.Framework.Version, result.Runtime.Supervisor.Version, result.Runtime.GoVersion)
	}
	fmt.Fprintf(&summary, "Request:\n%s\n\n", strings.TrimSpace(request.Definition.Prompt))
	fmt.Fprintf(&summary, "Agent outcome: %s\n", humanAgentOutcome(result.AgentOutcome))
	fmt.Fprintf(&summary, "Evaluation: %s\n", humanEvaluationStatus(result.EvaluationStatus))
	if result.Verification != nil {
		fmt.Fprintf(&summary, "Framework outcome: %s", result.Verification.FrameworkOutcome.Status)
		if result.Verification.FrameworkOutcome.Details != "" {
			fmt.Fprintf(&summary, " · %s", result.Verification.FrameworkOutcome.Details)
		}
		summary.WriteByte('\n')
		fmt.Fprintf(&summary, "Workflow conformance: %s", result.Verification.WorkflowConformance.Status)
		if result.Verification.WorkflowConformance.Details != "" {
			fmt.Fprintf(&summary, " · %s", result.Verification.WorkflowConformance.Details)
		}
		summary.WriteByte('\n')
	}
	if len(result.UnavailableEvidence) > 0 {
		fmt.Fprintf(&summary, "Unavailable trusted evidence: %s\n", joinCapabilities(result.UnavailableEvidence))
	}
	summary.WriteString("\nDiagnostic limitation: this unconfined local run can validate the resulting Project, but it cannot make authoritative isolation or workflow-observation claims.\n")
	summary.WriteString("Reproduction: retained evidence supports diagnosis only. No sealed Project bundle is retained for verifier replay; a new live run is a fresh stochastic attempt, not a replay.\n")
	summary.WriteString("Next action: inspect verification.json, commands.jsonl, transcript.redacted.txt, and diff.patch when present.\n")
	return summary.String()
}

// attemptTranscript extracts provider messages as inert redacted text in event order.
func attemptTranscript(events []Event) string {
	var transcript strings.Builder
	for _, event := range events {
		if event.Kind != EventMessage || event.Fields["text"] == "" {
			continue
		}
		if transcript.Len() > 0 {
			transcript.WriteString("\n\n")
		}
		transcript.WriteString(event.Fields["text"])
	}
	if transcript.Len() > 0 {
		transcript.WriteByte('\n')
	}
	return transcript.String()
}

// attemptCommands retains command telemetry separately while preserving its declared evidence classification.
func attemptCommands(events []Event) string {
	var commands strings.Builder
	for _, event := range events {
		if event.Kind != EventCommandStarted && event.Kind != EventCommandFinished {
			continue
		}
		body, err := json.Marshal(event)
		if err != nil {
			continue
		}
		commands.Write(body)
		commands.WriteByte('\n')
	}
	return commands.String()
}

// environmentKeys records the effective variable names while omitting all values.
func environmentKeys(environment []string) []string {
	seen := map[string]bool{}
	for _, assignment := range environment {
		key, _, ok := strings.Cut(assignment, "=")
		if ok && key != "" {
			seen[key] = true
		}
	}
	keys := make([]string, 0, len(seen))
	for key := range seen {
		keys = append(keys, key)
	}
	sort.Strings(keys)
	return keys
}

// sortedCapabilities returns stable report order without mutating component-owned slices.
func sortedCapabilities(capabilities []Capability) []Capability {
	result := append([]Capability(nil), capabilities...)
	sort.Slice(result, func(left, right int) bool { return result[left] < result[right] })
	return result
}

// joinCapabilities renders stable evidence names for the human summary.
func joinCapabilities(capabilities []Capability) string {
	values := sortedCapabilities(capabilities)
	names := make([]string, len(values))
	for index, capability := range values {
		names[index] = string(capability)
	}
	return strings.Join(names, ", ")
}

// humanAgentOutcome translates internal lifecycle states into direct report language.
func humanAgentOutcome(outcome AgentOutcome) string {
	switch outcome {
	case AgentCompleted:
		return "Agent completed"
	case AgentAbstained:
		return "Agent abstained"
	case AgentNotStarted:
		return "Agent did not start"
	case AgentTimeout:
		return "Agent timed out"
	case AgentCancelled:
		return "Agent was cancelled"
	default:
		return "Agent failed"
	}
}

// humanEvaluationStatus translates evaluator states without exposing internal enum spelling as primary copy.
func humanEvaluationStatus(status EvaluationStatus) string {
	switch status {
	case EvaluationValid:
		return "Result verified"
	case EvaluationValidAbstention:
		return "Abstention verified"
	case EvaluationIneligible:
		return "Could not evaluate safely"
	case EvaluationFixtureError:
		return "Project preparation failed"
	case EvaluationEvaluatorError:
		return "Evaluation failed"
	default:
		return "Not evaluated"
	}
}
