package skills

import (
	"slices"
	"strings"
	"testing"

	"github.com/goforj/atlas/project"
)

// TestFeatureRolloutGuidancePrecedesInstallation keeps rollout advice available before a dependency exists.
func TestFeatureRolloutGuidancePrecedesInstallation(t *testing.T) {
	if !slices.Contains(RecommendedNames(project.Project{Name: "checkout"}), "goforj-feature-rollouts") {
		t.Fatal("rollout skill requires an installed dependency")
	}
	skill, ok := ByName("goforj-feature-rollouts")
	if !ok {
		t.Fatal("missing rollout skill")
	}
	for _, phrase := range []string{"Authorize the tenant separately", "Ordinary For scopes", "flags.Snapshot()", "fallback", "forj render", "Plain configuration"} {
		if !strings.Contains(skill.Content, phrase) {
			t.Fatalf("missing %q", phrase)
		}
	}
}
