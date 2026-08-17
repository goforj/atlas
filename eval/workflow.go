package eval

import (
	"encoding/json"
	"fmt"
	"reflect"
	"strconv"
)

// EvaluateWorkflow checks promoted workflow actions against supervisor-observed command events.
func EvaluateWorkflow(workflow WorkflowExpectation, events []Event, forjDigest string, capabilities []Capability) (EndpointResult, []EndpointResult) {
	checks := make([]EndpointResult, 0, len(workflow.Generators))
	if len(workflow.Generators) > 0 && !hasCapability(capabilities, CapabilityCommands) {
		for _, requirement := range workflow.Generators {
			checks = append(checks, EndpointResult{ID: requirement.ID, Status: EndpointIneligible, Details: "trusted command evidence is unavailable"})
		}
		return EndpointResult{ID: workflow.ID, Status: EndpointIneligible, Details: "trusted command evidence is unavailable"}, checks
	}
	failed := 0
	for _, requirement := range workflow.Generators {
		result := evaluateGeneratorRequirement(requirement, events, forjDigest)
		checks = append(checks, result)
		if result.Status != EndpointPassed {
			failed++
		}
	}
	if failed > 0 {
		return EndpointResult{
			ID:      workflow.ID,
			Status:  EndpointFailed,
			Details: fmt.Sprintf("%d of %d required workflow actions failed", failed, len(workflow.Generators)),
		}, checks
	}
	return EndpointResult{ID: workflow.ID, Status: EndpointPassed}, checks
}

// hasCapability reports whether preflight qualified one observation class for this attempt.
func hasCapability(capabilities []Capability, target Capability) bool {
	for _, capability := range capabilities {
		if capability == target {
			return true
		}
	}
	return false
}

// evaluateGeneratorRequirement requires the trusted GoForj executable, exact argv, and a successful correlated completion.
func evaluateGeneratorRequirement(requirement GeneratorRequirement, events []Event, forjDigest string) EndpointResult {
	for _, started := range events {
		if started.Kind != EventCommandStarted || started.Fields[EventFieldExecutableDigest] != forjDigest {
			continue
		}
		var arguments []string
		if err := json.Unmarshal([]byte(started.Fields[EventFieldArguments]), &arguments); err != nil || !reflect.DeepEqual(arguments, requirement.Arguments) {
			continue
		}
		commandID := started.Fields[EventFieldCommandID]
		for _, finished := range events {
			if finished.Kind != EventCommandFinished || finished.Fields[EventFieldCommandID] != commandID {
				continue
			}
			exitCode, err := strconv.Atoi(finished.Fields[EventFieldExitCode])
			if err == nil && exitCode == 0 {
				return EndpointResult{ID: requirement.ID, Status: EndpointPassed}
			}
			return EndpointResult{ID: requirement.ID, Status: EndpointFailed, Details: "the matching generator command did not succeed"}
		}
		return EndpointResult{ID: requirement.ID, Status: EndpointFailed, Details: "the matching generator command has no trusted completion"}
	}
	return EndpointResult{ID: requirement.ID, Status: EndpointFailed, Details: "the required GoForj generator command was not observed"}
}
