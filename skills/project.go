package skills

import (
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strings"
)

var projectSkillNamePattern = regexp.MustCompile(`^[a-z][a-z0-9]*(?:-[a-z0-9]+)*$`)

// ProjectSkill describes a repo-owned Atlas skill under .ai/skills.
type ProjectSkill struct {
	Name string
	Path string
	File bool
}

// ProjectSkills returns repo-owned Atlas skills from .ai/skills.
func ProjectSkills(root string) ([]ProjectSkill, error) {
	sourceRoot := filepath.Join(firstNonEmpty(root, "."), ".ai", "skills")
	entries, err := os.ReadDir(sourceRoot)
	if os.IsNotExist(err) {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}

	projectSkills := []ProjectSkill{}
	for _, entry := range entries {
		path := filepath.Join(sourceRoot, entry.Name())
		if entry.IsDir() {
			skillPath := filepath.Join(path, "SKILL.md")
			if !fileExists(skillPath) {
				continue
			}
			projectSkills = append(projectSkills, ProjectSkill{
				Name: entry.Name(),
				Path: skillPath,
			})
			continue
		}
		if strings.HasSuffix(entry.Name(), ".md") {
			projectSkills = append(projectSkills, ProjectSkill{
				Name: strings.TrimSuffix(entry.Name(), ".md"),
				Path: path,
				File: true,
			})
		}
	}

	sort.Slice(projectSkills, func(i int, j int) bool {
		return projectSkills[i].Name < projectSkills[j].Name
	})
	return projectSkills, nil
}

// ScaffoldProjectSkill creates a repo-owned Atlas skill template.
func ScaffoldProjectSkill(root string, name string) (string, error) {
	name = strings.TrimSpace(name)
	if !projectSkillNamePattern.MatchString(name) {
		return "", fmt.Errorf("invalid skill name %q; use lowercase kebab-case such as checkout-rules", name)
	}

	path := filepath.Join(firstNonEmpty(root, "."), ".ai", "skills", name, "SKILL.md")
	if fileExists(path) {
		return "", fmt.Errorf("project skill %q already exists at %s", name, path)
	}
	content := projectSkillTemplate(name)
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return "", err
	}
	if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
		return "", err
	}
	return path, nil
}

// firstNonEmpty returns the first non-blank value.
func firstNonEmpty(values ...string) string {
	for _, value := range values {
		if strings.TrimSpace(value) != "" {
			return value
		}
	}
	return ""
}

func projectSkillTemplate(name string) string {
	title := title(name)
	return fmt.Sprintf(`# %s

Describe the durable repo-specific convention, workflow, command, or review expectation this agent should remember.

Use this skill when:

- the task touches this project's specific rules
- the user asks for behavior covered by this convention
- the agent is about to generate, review, or modify related code

Guidance:

- Keep changes consistent with the convention.
- Prefer concrete project commands and paths over broad advice.
- If the convention no longer applies, ask before following it.
`, title)
}
