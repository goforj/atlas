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

// TestEnvironmentSkillAlwaysDiscoverable keeps startup configuration guidance available beyond HTTP projects.
func TestEnvironmentSkillAlwaysDiscoverable(t *testing.T) {
	for _, component := range []string{"web-api", "cli", "jobs", ""} {
		if names := RecommendedNames(project.Project{Name: "example", Components: []string{component}}); !slices.Contains(names, "goforj-environment-configuration") {
			t.Fatalf("%q recommendations = %v", component, names)
		}
	}
	skill, ok := ByName("goforj-environment-configuration")
	if !ok {
		t.Fatal("missing environment skill")
	}
	for _, phrase := range []string{"github.com/goforj/env/v2", "v2.6.1", "Fallbacks are strings", "explicitly empty process values", "invalid sentinel fallback", "String and GoString", "not a secret manager", "Preserve adequate os.LookupEnv/strconv", "fresh processes"} {
		if !strings.Contains(skill.Content, phrase) {
			t.Errorf("missing decision rule %q", phrase)
		}
	}
}

// TestEnvironmentSkillNativeProjections keeps the decision guidance present in each supported agent format.
func TestEnvironmentSkillNativeProjections(t *testing.T) {
	for _, agent := range []agents.Agent{agents.Codex{}, agents.Claude{}, agents.Copilot{}, agents.Gemini{}} {
		t.Run(agent.Name(), func(t *testing.T) {
			root := t.TempDir()
			paths, err := Write(WriteOptions{Root: root, Agent: agent, Project: project.Project{Name: "example"}})
			if err != nil {
				t.Fatal(err)
			}
			found := false
			for _, path := range paths {
				if strings.Contains(filepath.ToSlash(path), "goforj-environment-configuration") {
					body, err := os.ReadFile(path)
					if err != nil {
						t.Fatal(err)
					}
					if !strings.Contains(string(body), "Fallbacks are strings") {
						t.Fatal("native projection lost the installed API contract")
					}
					found = true
				}
			}
			if !found {
				t.Fatal("missing native environment guidance")
			}
		})
	}
}
