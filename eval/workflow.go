package eval

import (
	"encoding/json"
	"fmt"
	"path"
	"reflect"
	"strconv"
	"strings"
)

// EvaluateWorkflow checks promoted workflow actions against supervisor-observed command events.
func EvaluateWorkflow(workflow WorkflowExpectation, events []Event, forjDigest string, capabilities []Capability) (EndpointResult, []EndpointResult) {
	checks := make([]EndpointResult, 0, len(workflow.Requirements)+len(workflow.Generators))
	failed := 0
	ineligible := 0
	requiredActions := len(workflow.Generators)
	for _, requirement := range workflow.Requirements {
		result := evaluateWorkflowRequirement(requirement, events, capabilities)
		checks = append(checks, result)
		if requirement.Kind == RequirementWorkflow {
			requiredActions++
			switch result.Status {
			case EndpointFailed:
				failed++
			case EndpointIneligible:
				ineligible++
			}
		}
	}
	if len(workflow.Generators) > 0 && !hasCapability(capabilities, CapabilityCommands) {
		for _, requirement := range workflow.Generators {
			checks = append(checks, EndpointResult{ID: requirement.ID, Kind: RequirementWorkflow, Status: EndpointIneligible, Details: "trusted command evidence is unavailable"})
		}
		return EndpointResult{ID: workflow.ID, Status: EndpointIneligible, Details: "trusted command evidence is unavailable"}, checks
	}
	for _, requirement := range workflow.Generators {
		result := evaluateGeneratorRequirement(requirement, events, forjDigest)
		result.Kind = RequirementWorkflow
		checks = append(checks, result)
		if result.Status != EndpointPassed {
			failed++
		}
	}
	if failed > 0 {
		return EndpointResult{
			ID:      workflow.ID,
			Status:  EndpointFailed,
			Details: fmt.Sprintf("%d of %d required workflow actions failed", failed, requiredActions),
		}, checks
	}
	if ineligible > 0 {
		return EndpointResult{ID: workflow.ID, Status: EndpointIneligible, Details: fmt.Sprintf("%d required workflow observations lack trusted evidence", ineligible)}, checks
	}
	return EndpointResult{ID: workflow.ID, Status: EndpointPassed}, checks
}

// evaluateWorkflowRequirement reports optional quality evidence without allowing it to silently become a conformance gate.
func evaluateWorkflowRequirement(requirement WorkflowRequirement, events []Event, capabilities []Capability) EndpointResult {
	result := EndpointResult{ID: requirement.ID, Kind: requirement.Kind}
	if !hasCapability(capabilities, requirement.Capability) {
		result.Status = EndpointIneligible
		result.Details = "trusted " + string(requirement.Capability) + " evidence is unavailable"
		return result
	}
	switch requirement.Capability {
	case CapabilityFileReads:
		for _, event := range events {
			if event.Kind == EventFileWrite || event.Kind == EventCommandStarted {
				break
			}
			if event.Kind == EventFileRead && workflowPathMatches(event.Fields[EventFieldPath], requirement.Paths) {
				result.Status = EndpointPassed
				return result
			}
		}
		result.Status = EndpointFailed
		result.Details = "required Project inspection was not observed before the first mutation"
		return result
	default:
		result.Status = EndpointIneligible
		result.Details = "no promoted evaluator exists for " + string(requirement.Capability)
		return result
	}
}

// workflowPathMatches keeps observation contracts Project-relative while supporting explicit recursive package roots.
func workflowPathMatches(candidate string, patterns []string) bool {
	candidate = strings.TrimPrefix(path.Clean(strings.ReplaceAll(candidate, "\\", "/")), "./")
	firstSegment := strings.SplitN(candidate, "/", 2)[0]
	if candidate == "." || strings.HasPrefix(candidate, "../") || strings.HasPrefix(candidate, "/") || strings.Contains(firstSegment, ":") {
		return false
	}
	if len(patterns) == 0 {
		return true
	}
	for _, pattern := range patterns {
		if strings.HasSuffix(pattern, "/**") {
			root := strings.TrimSuffix(pattern, "/**")
			if candidate == root || strings.HasPrefix(candidate, root+"/") {
				return true
			}
			continue
		}
		if matched, _ := path.Match(pattern, candidate); matched {
			return true
		}
	}
	return false
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
