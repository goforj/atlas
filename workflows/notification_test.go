package workflows

import (
	"slices"
	"strings"
	"testing"
)

// TestNotificationPlanPreservesAcceptanceBoundaries keeps senders separate from generated jobs.
func TestNotificationPlanPreservesAcceptanceBoundaries(t *testing.T) {
	for _, task := range []string{"add incident alerts with primary and fallback senders", "compose notification senders"} {
		if Classify(task) != "goforj-notification-delivery" || !slices.Contains(ClassifyAll(task), "goforj-notification-delivery") {
			t.Fatalf("wrong workflow for %q", task)
		}
	}
	if len(workflowDocs("goforj-notification-delivery")) < 2 {
		t.Fatal("missing notification docs")
	}
	for _, command := range workflowCommands("goforj-notification-delivery", appScope{}) {
		if strings.Contains(command, "make:") {
			t.Fatalf("invented notification generator: %s", command)
		}
	}
	if !strings.Contains(strings.Join(workflowWarnings("goforj-notification-delivery"), " "), "unknown acceptance") {
		t.Fatal("missing acceptance warning")
	}
}
