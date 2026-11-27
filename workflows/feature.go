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

// init registers evaluator concerns without replacing capability-specific generation guidance.
func init() {
	registerApplicationWorkflow(applicationWorkflowDefinition{id: "goforj-feature-rollouts", matches: isFeatureRolloutTask, docsTopic: isFeatureRolloutTask})
}
