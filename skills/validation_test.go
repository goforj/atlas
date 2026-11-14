package skills

import (
	"github.com/goforj/atlas/agents"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"testing"

	"github.com/goforj/atlas/project"
)

// TestValidationRecommendationKeepsRequestPolicyDistinctFromBuildValidation verifies selection on actual HTTP projects.
func TestValidationRecommendationKeepsRequestPolicyDistinctFromBuildValidation(t *testing.T) {
	for _, test := range []struct {
		component string
		want      bool
	}{{"web-api", true}, {"web-ui", true}, {"cli", false}, {"jobs", false}} {
		t.Run(test.component, func(t *testing.T) {
			names := RecommendedNames(project.Project{Name: "example", Components: []string{test.component}})
			if slices.Contains(names, "goforj-validation") != test.want {
				t.Fatalf("recommendations = %v", names)
			}
			if !slices.Contains(names, "goforj-testing-and-validation") {
				t.Fatal("build validation guidance disappeared")
			}
		})
	}
}

// TestValidationSkillDocumentsReleasedFailureAndPresenceContracts prevents advice from collapsing distinct library semantics.
func TestValidationSkillDocumentsReleasedFailureAndPresenceContracts(t *testing.T) {
	skill, ok := ByName("goforj-validation")
	if !ok {
		t.Fatal("missing validation skill")
	}
	for _, phrase := range []string{"github.com/goforj/validation v0.2.0", "Input[T]", "Nullable()", "Required()", "zero and false", "errors.As", "*validation.Errors", "Issues() Path/Code", "ContextRuleFunc[T]", "operational failures", "adequate simple standard-library checks", "existing validator", "read the relevant library section"} {
		if !strings.Contains(skill.Content, phrase) {
			t.Errorf("skill missing %q", phrase)
		}
	}
}

// TestValidationSkillProjectsToNativeTargets keeps the dedicated source available to each supported HTTP agent.
func TestValidationSkillProjectsToNativeTargets(t *testing.T) {
	for _, agent := range []agents.Agent{agents.Codex{}, agents.Claude{}, agents.Copilot{}, agents.Gemini{}} {
		t.Run(agent.Name(), func(t *testing.T) {
			root := t.TempDir()
			paths, err := Write(WriteOptions{Root: root, Agent: agent, Project: project.Project{Name: "example", Components: []string{"web-api"}}})
			if err != nil {
				t.Fatal(err)
			}
			found := false
			for _, path := range paths {
				if strings.Contains(filepath.ToSlash(path), "goforj-validation") {
					body, err := os.ReadFile(path)
					if err != nil {
						t.Fatal(err)
					}
					if !strings.Contains(string(body), "github.com/goforj/validation v0.2.0") || !strings.Contains(string(body), "*validation.Errors") {
						t.Fatal("native projection lost policy")
					}
					found = true
				}
			}
			if !found {
				t.Fatal("validation native projection missing")
			}
		})
	}
}
