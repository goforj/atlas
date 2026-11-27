package skills

import (
	"os"
	"path/filepath"
	"slices"
	"strings"
	"testing"

	"github.com/goforj/atlas/agents"
	"github.com/goforj/atlas/project"
)

// TestNotificationGuidanceIsDiscoverableBeforeInstallation protects the library selection boundary.
func TestNotificationGuidanceIsDiscoverableBeforeInstallation(t *testing.T) {
	if !slices.Contains(RecommendedNames(project.Project{Name: "incident-app"}), "goforj-notification-delivery") {
		t.Fatal("notification guidance requires an installed component")
	}
	for _, term := range []string{"mail", "events", "queue", "standard-library", "ErrOutcomeUnknown", "Fallback(primary, secondary)", "never proof", "no make:notification", "forj render"} {
		if !strings.Contains(notificationDelivery, term) {
			t.Errorf("missing guidance %q", term)
		}
	}
}

// TestNotificationGuidanceNativeProjections protects the installed sender policy across agent formats.
func TestNotificationGuidanceNativeProjections(t *testing.T) {
	for _, test := range []struct {
		agent agents.Agent
		path  string
	}{
		{agents.Codex{}, ".agents/skills/goforj-notification-delivery/SKILL.md"},
		{agents.Claude{}, ".claude/skills/goforj-notification-delivery/SKILL.md"},
		{agents.Copilot{}, ".github/instructions/goforj-notification-delivery.instructions.md"},
	} {
		root := t.TempDir()
		if _, err := Write(WriteOptions{Root: root, Agent: test.agent}); err != nil {
			t.Fatal(err)
		}
		body, err := os.ReadFile(filepath.Join(root, test.path))
		if err != nil {
			t.Fatal(err)
		}
		if !strings.Contains(string(body), "ErrOutcomeUnknown") {
			t.Fatalf("policy missing in %s", test.path)
		}
	}
}
