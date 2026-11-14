package skills

import (
	"github.com/goforj/atlas/project"
	"slices"
	"strings"
	"testing"
)

// TestBusinessMetricsSkillIsDiscoverableBeforeInstallation preserves selection guidance in minimal Projects.
func TestBusinessMetricsSkillIsDiscoverableBeforeInstallation(t *testing.T) {
	if !slices.Contains(RecommendedNames(project.Project{Name: "checkout"}), "goforj-business-metrics") {
		t.Fatal("missing skill")
	}
	skill, ok := ByName("goforj-business-metrics")
	if !ok {
		t.Fatal("missing skill")
	}
	for _, phrase := range []string{"manager.Registry()", "ObserveDuration", "tenant IDs", "Prometheus", "no make:metric"} {
		if !strings.Contains(skill.Content, phrase) {
			t.Fatalf("missing %q", phrase)
		}
	}
}
