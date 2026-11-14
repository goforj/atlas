package workflows

import (
	"strings"

	"github.com/goforj/atlas/project"
)

// projectHasFeatureComponent treats declared component metadata as capability evidence without guessing a release threshold.
func projectHasFeatureComponent(p project.Project) bool {
	for _, component := range p.Components {
		if strings.EqualFold(strings.TrimSpace(component), "feature") {
			return true
		}
	}
	return false
}

// constrainFeaturePlan requests CLI capability evidence before proposing an unreleased component to an older Project.
func constrainFeaturePlan(result *PlanResult, p project.Project, scope appScope) {
	if !stringSliceContains(result.WorkflowIDs, "goforj-feature-rollouts") || projectHasFeatureComponent(p) {
		return
	}
	generate := scope.command("generate --feature")
	inspect := scope.command("generate --help")
	for i, command := range result.Commands {
		if command == generate {
			result.Commands[i] = inspect
		}
	}
	result.Warnings = append(result.Warnings, "Project metadata does not declare the feature component. Inspect generate --help for --feature support before enabling it; use application-owned native composition if the CLI lacks that capability.")
	conditional := []PlanStep{
		{WorkflowID: "goforj-feature-rollouts", Action: inspect, Purpose: "verify optional feature generation is supported by the selected CLI"},
		{WorkflowID: "goforj-feature-rollouts", Action: "after confirming support, enable feature for the owning App and request a new workflow plan", Purpose: "keep unsupported component keys and commands out of older Projects"},
	}
	steps := make([]PlanStep, 0, len(result.Steps))
	inserted := false
	for _, step := range result.Steps {
		if step.WorkflowID == "goforj-feature-rollouts" {
			if !inserted {
				steps = append(steps, conditional...)
				inserted = true
			}
			continue
		}
		steps = append(steps, step)
	}
	for i := range steps {
		steps[i].Order = i + 1
	}
	result.Steps = steps
	for i := range result.Segments {
		segment := &result.Segments[i]
		if segment.WorkflowID != "goforj-feature-rollouts" {
			continue
		}
		for index, command := range segment.Commands {
			if command == generate {
				segment.Commands[index] = inspect
			}
		}
		segment.Steps = nil
		for _, step := range steps {
			if step.WorkflowID == segment.WorkflowID {
				segment.Steps = append(segment.Steps, step)
			}
		}
		segment.Warnings = append(segment.Warnings, "Inspect CLI support before enabling the optional feature component.")
	}
}
