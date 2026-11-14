package workflows

import "testing"

// TestEnvironmentConfigurationNaturalTasks covers implementation phrasing independently from the shared metadata fixtures.
func TestEnvironmentConfigurationNaturalTasks(t *testing.T) {
	for _, task := range []string{"Load configuration from environment variables before startup.", "Set up a typed config provider with a worker count.", "Require startup settings and document process/file precedence."} {
		if got := Classify(task); got != "goforj-configure-environment" {
			t.Errorf("Classify(%q) = %q", task, got)
		}
	}
}
