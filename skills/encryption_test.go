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

// TestEncryptionSkillAlwaysDiscoverable retains explicit selection rules for every application surface.
func TestEncryptionSkillAlwaysDiscoverable(t *testing.T) {
	for _, component := range []string{"web-api", "cli", "jobs", ""} {
		names := RecommendedNames(project.Project{Name: "example", Components: []string{component}})
		if !slices.Contains(names, "goforj-payload-encryption") {
			t.Fatalf("%q recommendations = %v", component, names)
		}
	}
	skill, ok := ByName("goforj-payload-encryption")
	if !ok {
		t.Fatal("missing encryption skill")
	}
	for _, phrase := range []string{"github.com/goforj/crypt", "v1.2.0", "16- or 32-byte", "APP_PREVIOUS_KEYS", "new-key writer", "authenticated CBC", "not password hashing", "managed secret store", "Keep adequate reviewed encryption code", "does not reencrypt stored data"} {
		if !strings.Contains(skill.Content, phrase) {
			t.Errorf("missing encryption decision %q", phrase)
		}
	}
}

// TestEncryptionSkillNativeProjections preserves key-rotation guidance in every supported agent format.
func TestEncryptionSkillNativeProjections(t *testing.T) {
	for _, agent := range []agents.Agent{agents.Codex{}, agents.Claude{}, agents.Copilot{}, agents.Gemini{}} {
		t.Run(agent.Name(), func(t *testing.T) {
			root := t.TempDir()
			paths, err := Write(WriteOptions{Root: root, Agent: agent, Project: project.Project{Name: "example"}})
			if err != nil {
				t.Fatal(err)
			}
			found := false
			for _, path := range paths {
				if strings.Contains(filepath.ToSlash(path), "goforj-payload-encryption") {
					body, err := os.ReadFile(path)
					if err != nil {
						t.Fatal(err)
					}
					if !strings.Contains(string(body), "APP_PREVIOUS_KEYS") || !strings.Contains(string(body), "current key") {
						t.Fatal("native projection lost rotation policy")
					}
					found = true
				}
			}
			if !found {
				t.Fatal("missing native encryption guidance")
			}
		})
	}
}

// TestPayloadCancellationGuidanceStatesTheNoWorkBoundary preserves cancellation intent without prescribing service implementation.
func TestPayloadCancellationGuidanceStatesTheNoWorkBoundary(t *testing.T) {
	if !strings.Contains(payloadEncryption, "already canceled") || !strings.Contains(payloadEncryption, "before invoking the cipher or repository") {
		t.Fatal("pre-canceled no-work behavior omitted")
	}
}
