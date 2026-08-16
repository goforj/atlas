package eval

import (
	"fmt"
	"path"
	"sort"
	"strings"
)

// Registry owns the promoted workflow and verifier contracts available to manifest resolution.
type Registry struct {
	workflows map[string]WorkflowExpectation
	verifiers map[string]Verifier
}

// ResolvedEvaluation binds one manifest to exact promoted contracts and their capability union.
type ResolvedEvaluation struct {
	Definition   EvaluationDefinition
	Workflow     WorkflowExpectation
	Verifier     Verifier
	Capabilities []Capability
}

// NewRegistry validates and snapshots promoted contracts so later caller mutation cannot change resolution.
func NewRegistry(workflows []WorkflowExpectation, verifiers []Verifier) (*Registry, error) {
	registry := &Registry{
		workflows: make(map[string]WorkflowExpectation, len(workflows)),
		verifiers: make(map[string]Verifier, len(verifiers)),
	}
	for _, workflow := range workflows {
		if !contractIDPattern.MatchString(workflow.ID) {
			return nil, fmt.Errorf("workflow ID %q must be versioned", workflow.ID)
		}
		if _, exists := registry.workflows[workflow.ID]; exists {
			return nil, fmt.Errorf("duplicate workflow ID %q", workflow.ID)
		}
		seenRequirements := map[string]bool{}
		cloned := workflow
		cloned.Requirements = cloneWorkflowRequirements(workflow.Requirements)
		cloned.Generators = cloneGeneratorRequirements(workflow.Generators)
		for _, requirement := range cloned.Requirements {
			if !evaluationIDPattern.MatchString(requirement.ID) {
				return nil, fmt.Errorf("workflow %q requirement ID %q must be a safe slug", workflow.ID, requirement.ID)
			}
			if seenRequirements[requirement.ID] {
				return nil, fmt.Errorf("workflow %q has duplicate requirement ID %q", workflow.ID, requirement.ID)
			}
			if requirement.Kind != RequirementWorkflow && requirement.Kind != RequirementQuality {
				return nil, fmt.Errorf("workflow %q requirement %q has unknown kind %q", workflow.ID, requirement.ID, requirement.Kind)
			}
			if requirement.Capability == "" {
				return nil, fmt.Errorf("workflow %q requirement %q has no observation capability", workflow.ID, requirement.ID)
			}
			for _, pattern := range requirement.Paths {
				normalized := strings.ReplaceAll(strings.TrimSpace(pattern), "\\", "/")
				firstSegment := strings.SplitN(normalized, "/", 2)[0]
				if normalized == "" || normalized != pattern || path.IsAbs(normalized) || strings.Contains(firstSegment, ":") || strings.HasPrefix(path.Clean(normalized), "../") {
					return nil, fmt.Errorf("workflow %q requirement %q has unsafe Project path pattern %q", workflow.ID, requirement.ID, pattern)
				}
				if _, err := path.Match(normalized, "candidate"); err != nil {
					return nil, fmt.Errorf("workflow %q requirement %q has invalid Project path pattern %q: %w", workflow.ID, requirement.ID, pattern, err)
				}
			}
			seenRequirements[requirement.ID] = true
		}
		for _, generator := range cloned.Generators {
			if !evaluationIDPattern.MatchString(generator.ID) {
				return nil, fmt.Errorf("workflow %q generator ID %q must be a safe slug", workflow.ID, generator.ID)
			}
			if seenRequirements[generator.ID] {
				return nil, fmt.Errorf("workflow %q has duplicate requirement ID %q", workflow.ID, generator.ID)
			}
			if len(generator.Arguments) < 2 || generator.Arguments[0] == "" {
				return nil, fmt.Errorf("workflow %q generator %q requires structured arguments", workflow.ID, generator.ID)
			}
			seenRequirements[generator.ID] = true
		}
		registry.workflows[workflow.ID] = cloned
	}
	for _, verifier := range verifiers {
		if verifier == nil {
			return nil, fmt.Errorf("nil verifier is not supported")
		}
		id := verifier.ID()
		if !contractIDPattern.MatchString(id) {
			return nil, fmt.Errorf("verifier ID %q must be versioned", id)
		}
		if _, exists := registry.verifiers[id]; exists {
			return nil, fmt.Errorf("duplicate verifier ID %q", id)
		}
		registry.verifiers[id] = verifier
	}
	return registry, nil
}

// cloneWorkflowRequirements prevents callers from mutating promoted observation paths after registration.
func cloneWorkflowRequirements(requirements []WorkflowRequirement) []WorkflowRequirement {
	cloned := make([]WorkflowRequirement, 0, len(requirements))
	for _, requirement := range requirements {
		requirement.Paths = append([]string(nil), requirement.Paths...)
		cloned = append(cloned, requirement)
	}
	return cloned
}

// Resolve binds exact references and computes capabilities without allowing manifest overrides.
func (registry *Registry) Resolve(definition EvaluationDefinition) (ResolvedEvaluation, error) {
	if registry == nil {
		return ResolvedEvaluation{}, fmt.Errorf("evaluation registry is required")
	}
	workflow, ok := registry.workflows[definition.WorkflowID]
	if !ok {
		return ResolvedEvaluation{}, fmt.Errorf("workflow %q is not promoted", definition.WorkflowID)
	}
	verifier, ok := registry.verifiers[definition.VerifierID]
	if !ok {
		return ResolvedEvaluation{}, fmt.Errorf("verifier %q is not promoted", definition.VerifierID)
	}
	capabilitySet := map[Capability]bool{}
	for _, requirement := range workflow.Requirements {
		if requirement.Kind == RequirementWorkflow {
			capabilitySet[requirement.Capability] = true
		}
	}
	if len(workflow.Generators) > 0 {
		capabilitySet[CapabilityCommands] = true
	}
	for _, capability := range verifier.Capabilities() {
		if capability == "" {
			return ResolvedEvaluation{}, fmt.Errorf("verifier %q declares an empty capability", verifier.ID())
		}
		capabilitySet[capability] = true
	}
	capabilities := make([]Capability, 0, len(capabilitySet))
	for capability := range capabilitySet {
		capabilities = append(capabilities, capability)
	}
	sort.Slice(capabilities, func(left, right int) bool { return capabilities[left] < capabilities[right] })
	return ResolvedEvaluation{
		Definition:   definition,
		Workflow:     workflow,
		Verifier:     verifier,
		Capabilities: capabilities,
	}, nil
}

// cloneGeneratorRequirements prevents callers from mutating promoted argument contracts after registration.
func cloneGeneratorRequirements(requirements []GeneratorRequirement) []GeneratorRequirement {
	cloned := make([]GeneratorRequirement, 0, len(requirements))
	for _, requirement := range requirements {
		requirement.Arguments = append([]string(nil), requirement.Arguments...)
		cloned = append(cloned, requirement)
	}
	return cloned
}
