package skills

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/goforj/atlas/agents"
)

func TestWriteSkillMDFiles(t *testing.T) {
	root := t.TempDir()
	paths, err := Write(WriteOptions{Root: root, Agent: agents.Codex{}})
	if err != nil {
		t.Fatalf("write skills: %v", err)
	}
	if len(paths) != len(Catalog()) {
		t.Fatalf("expected %d paths, got %d", len(Catalog()), len(paths))
	}
	assertFile(t, filepath.Join(root, ".agents", "skills", "goforj-make-commands", "SKILL.md"))
	content, err := os.ReadFile(filepath.Join(root, ".agents", "skills", "goforj-make-commands", "SKILL.md"))
	if err != nil {
		t.Fatalf("read generated skill: %v", err)
	}
	assertHasPrefix(t, string(content), "---")
	assertContainsInText(t, string(content), "name: goforj-make-commands")
	assertContainsInText(t, string(content), "description: Prefer GoForj make commands for framework scaffolding.")
}

func TestWriteCopilotInstructionsAndPrompts(t *testing.T) {
	root := t.TempDir()
	paths, err := Write(WriteOptions{Root: root, Agent: agents.Copilot{}, IncludePrompts: true})
	if err != nil {
		t.Fatalf("write skills: %v", err)
	}
	if len(paths) != len(Catalog())+len(Prompts()) {
		t.Fatalf("unexpected path count %d", len(paths))
	}
	assertFile(t, filepath.Join(root, ".github", "instructions", "goforj-app-architecture.instructions.md"))
	assertFile(t, filepath.Join(root, ".github", "prompts", "goforj-add-route.prompt.md"))
}

func TestWriteGeminiContextFiles(t *testing.T) {
	root := t.TempDir()
	paths, err := Write(WriteOptions{Root: root, Agent: agents.Gemini{}})
	if err != nil {
		t.Fatalf("write skills: %v", err)
	}
	if len(paths) != len(Catalog()) {
		t.Fatalf("expected %d paths, got %d", len(Catalog()), len(paths))
	}
	assertFile(t, filepath.Join(root, ".gemini", "skills", "goforj-app-architecture", "GEMINI.md"))
}

func TestWriteCopiesUserSkills(t *testing.T) {
	root := t.TempDir()
	userSkill := filepath.Join(root, ".ai", "skills", "local-skill")
	if err := os.MkdirAll(userSkill, 0o755); err != nil {
		t.Fatalf("mkdir user skill: %v", err)
	}
	if err := os.WriteFile(filepath.Join(userSkill, "SKILL.md"), []byte("# Local Skill\n"), 0o644); err != nil {
		t.Fatalf("write user skill: %v", err)
	}

	if _, err := Write(WriteOptions{Root: root, Agent: agents.Codex{}}); err != nil {
		t.Fatalf("write skills: %v", err)
	}

	assertFile(t, filepath.Join(root, ".agents", "skills", "local-skill", "SKILL.md"))
}

func TestWriteCopilotMapsUserSkillsToInstructions(t *testing.T) {
	root := t.TempDir()
	userSkill := filepath.Join(root, ".ai", "skills", "local-skill")
	if err := os.MkdirAll(userSkill, 0o755); err != nil {
		t.Fatalf("mkdir user skill: %v", err)
	}
	if err := os.WriteFile(filepath.Join(userSkill, "SKILL.md"), []byte("# Local Skill\n"), 0o644); err != nil {
		t.Fatalf("write user skill: %v", err)
	}

	if _, err := Write(WriteOptions{Root: root, Agent: agents.Copilot{}}); err != nil {
		t.Fatalf("write skills: %v", err)
	}

	assertFile(t, filepath.Join(root, ".github", "instructions", "local-skill.instructions.md"))
}

func TestWriteGeminiMapsUserSkillsToContextFiles(t *testing.T) {
	root := t.TempDir()
	userSkill := filepath.Join(root, ".ai", "skills", "local-skill")
	if err := os.MkdirAll(userSkill, 0o755); err != nil {
		t.Fatalf("mkdir user skill: %v", err)
	}
	if err := os.WriteFile(filepath.Join(userSkill, "SKILL.md"), []byte("# Local Skill\n"), 0o644); err != nil {
		t.Fatalf("write user skill: %v", err)
	}

	if _, err := Write(WriteOptions{Root: root, Agent: agents.Gemini{}}); err != nil {
		t.Fatalf("write skills: %v", err)
	}

	assertFile(t, filepath.Join(root, ".gemini", "skills", "local-skill", "GEMINI.md"))
}

func TestProjectSkillsListsDirectoriesAndMarkdownFiles(t *testing.T) {
	root := t.TempDir()
	writeSkillTestFile(t, filepath.Join(root, ".ai", "skills", "local-skill", "SKILL.md"), "# Local Skill\n")
	writeSkillTestFile(t, filepath.Join(root, ".ai", "skills", "runbook.md"), "# Runbook\n")
	writeSkillTestFile(t, filepath.Join(root, ".ai", "skills", "draft", "notes.md"), "# Ignored\n")

	projectSkills, err := ProjectSkills(root)
	if err != nil {
		t.Fatalf("project skills: %v", err)
	}
	if len(projectSkills) != 2 {
		t.Fatalf("expected 2 project skills, got %#v", projectSkills)
	}
	if projectSkills[0].Name != "local-skill" || projectSkills[1].Name != "runbook" {
		t.Fatalf("unexpected project skills %#v", projectSkills)
	}
}

func TestScaffoldProjectSkill(t *testing.T) {
	root := t.TempDir()
	path, err := ScaffoldProjectSkill(root, "checkout-rules")
	if err != nil {
		t.Fatalf("scaffold project skill: %v", err)
	}
	assertFile(t, path)
	if _, err := ScaffoldProjectSkill(root, "checkout-rules"); err == nil {
		t.Fatal("expected duplicate skill error")
	}
	if _, err := ScaffoldProjectSkill(root, "CheckoutRules"); err == nil {
		t.Fatal("expected invalid skill name error")
	}
}

func assertContainsInText(t *testing.T, got string, want string) {
	t.Helper()
	if !strings.Contains(got, want) {
		t.Fatalf("missing text %q in %q", want, got)
	}
}

func assertHasPrefix(t *testing.T, got string, want string) {
	t.Helper()
	if !strings.HasPrefix(got, want) {
		t.Fatalf("expected prefix %q in %q", want, got)
	}
}

func assertFile(t *testing.T, path string) {
	t.Helper()
	if _, err := os.Stat(path); err != nil {
		t.Fatalf("expected file %s: %v", path, err)
	}
}

func writeSkillTestFile(t *testing.T, path string, content string) {
	t.Helper()
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatalf("mkdir %s: %v", path, err)
	}
	if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
		t.Fatalf("write %s: %v", path, err)
	}
}
