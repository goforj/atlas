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

// TestHTTPClientResilienceGuidancePreservesProjectPins prevents recommendations from silently upgrading existing applications.
func TestHTTPClientResilienceGuidancePreservesProjectPins(t *testing.T) {
	if !slices.Contains(RecommendedNames(project.Project{Name: "billing"}), "goforj-http-client-resilience") {
		t.Fatal("client guidance is not discoverable")
	}
	for _, term := range []string{"v1.1.0", "v2.0.1", "Do not blindly upgrade", "standard-library", "two retries", "non-idempotent", "errors.As", "errors.Is"} {
		if !strings.Contains(strings.ToLower(httpClientResilience), strings.ToLower(term)) {
			t.Errorf("guidance omits %q", term)
		}
	}
	for _, test := range []struct {
		agent agents.Agent
		path  string
	}{
		{agents.Codex{}, ".agents/skills/goforj-http-client-resilience/SKILL.md"},
		{agents.Claude{}, ".claude/skills/goforj-http-client-resilience/SKILL.md"},
		{agents.Copilot{}, ".github/instructions/goforj-http-client-resilience.instructions.md"},
	} {
		root := t.TempDir()
		if _, err := Write(WriteOptions{Root: root, Agent: test.agent}); err != nil {
			t.Fatal(err)
		}
		body, err := os.ReadFile(filepath.Join(root, test.path))
		if err != nil {
			t.Fatal(err)
		}
		if !strings.Contains(string(body), "Do not blindly upgrade") {
			t.Fatalf("pin guidance lost in %s", test.path)
		}
	}
}
