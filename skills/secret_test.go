package skills

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/goforj/atlas/agents"
	"github.com/goforj/atlas/project"
)

// TestSecretReaderGuidanceReachesMinimalProjects verifies native discovery before an optional dependency exists.
func TestSecretReaderGuidanceReachesMinimalProjects(t *testing.T) {
	for _, agent := range []agents.Agent{agents.Codex{}, agents.Claude{}, agents.Copilot{}, agents.Gemini{}} {
		t.Run(agent.Name(), func(t *testing.T) {
			root := t.TempDir()
			if _, err := Write(WriteOptions{Root: root, Agent: agent, Project: project.Project{Name: "minimal"}}); err != nil {
				t.Fatal(err)
			}
			path := filepath.Join(agent.SkillsPath(root), "goforj-secret-reads", "SKILL.md")
			if agent.Name() == "copilot" {
				path = filepath.Join(agent.SkillsPath(root), "goforj-secret-reads.instructions.md")
			}
			content, err := os.ReadFile(path)
			if err != nil {
				t.Fatal(err)
			}
			if !strings.Contains(string(content), strings.TrimSpace(secretReads)) {
				t.Fatalf("native projection lost secret guidance: %s", path)
			}
		})
	}
}
