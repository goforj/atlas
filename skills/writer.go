package skills

import (
	"io"
	"os"
	"path/filepath"
	"strings"

	"github.com/goforj/atlas/agents"
	"github.com/goforj/atlas/files"
)

// WriteOptions controls how skills are synchronized for an agent.
type WriteOptions struct {
	Root           string
	Agent          agents.Agent
	IncludePrompts bool
}

// Write writes built-in skills using the selected agent's native shape.
func Write(opts WriteOptions) ([]string, error) {
	switch opts.Agent.Name() {
	case "copilot":
		return writeCopilot(opts)
	case "gemini":
		return writeGemini(opts)
	default:
		return writeSkillMD(opts)
	}
}

// writeSkillMD writes Codex/Claude-style SKILL.md directories.
func writeSkillMD(opts WriteOptions) ([]string, error) {
	root := opts.Agent.SkillsPath(opts.Root)
	if err := os.MkdirAll(root, 0o755); err != nil {
		return nil, err
	}
	if err := cleanGenerated(root, ".md"); err != nil {
		return nil, err
	}

	paths := []string{}
	for _, skill := range Catalog() {
		dir := filepath.Join(root, skill.Name)
		path := filepath.Join(dir, "SKILL.md")
		content := skillMarkdown(skill)
		if err := files.WriteFile(path, []byte(content)); err != nil {
			return nil, err
		}
		paths = append(paths, path)
	}
	userPaths, err := copyUserSkills(opts.Root, root)
	if err != nil {
		return nil, err
	}
	paths = append(paths, userPaths...)
	return paths, nil
}

// writeCopilot maps Atlas skills into Copilot instruction and prompt files.
func writeCopilot(opts WriteOptions) ([]string, error) {
	root := opts.Agent.SkillsPath(opts.Root)
	if err := os.MkdirAll(root, 0o755); err != nil {
		return nil, err
	}
	if err := cleanGenerated(root, ".instructions.md"); err != nil {
		return nil, err
	}

	paths := []string{}
	for _, skill := range Catalog() {
		path := filepath.Join(root, skill.Name+".instructions.md")
		if err := files.WriteFile(path, []byte(copilotInstruction(skill))); err != nil {
			return nil, err
		}
		paths = append(paths, path)
	}
	userPaths, err := writeCopilotUserSkills(opts.Root, root)
	if err != nil {
		return nil, err
	}
	paths = append(paths, userPaths...)

	if opts.IncludePrompts {
		copilot, ok := opts.Agent.(agents.Copilot)
		if ok {
			promptRoot := copilot.PromptsPath(opts.Root)
			if err := os.MkdirAll(promptRoot, 0o755); err != nil {
				return nil, err
			}
			if err := cleanGenerated(promptRoot, ".prompt.md"); err != nil {
				return nil, err
			}
			for _, prompt := range Prompts() {
				path := filepath.Join(promptRoot, prompt.Name+".prompt.md")
				if err := files.WriteFile(path, []byte(copilotPrompt(prompt))); err != nil {
					return nil, err
				}
				paths = append(paths, path)
			}
		}
	}

	return paths, nil
}

// writeGemini maps Atlas skills into GEMINI.md context files.
func writeGemini(opts WriteOptions) ([]string, error) {
	root := opts.Agent.SkillsPath(opts.Root)
	if err := os.MkdirAll(root, 0o755); err != nil {
		return nil, err
	}
	if err := cleanGeminiGenerated(root); err != nil {
		return nil, err
	}

	paths := []string{}
	for _, skill := range Catalog() {
		dir := filepath.Join(root, skill.Name)
		path := filepath.Join(dir, "GEMINI.md")
		if err := files.WriteFile(path, []byte(skillMarkdown(skill))); err != nil {
			return nil, err
		}
		paths = append(paths, path)
	}
	userPaths, err := writeGeminiUserSkills(opts.Root, root)
	if err != nil {
		return nil, err
	}
	paths = append(paths, userPaths...)
	return paths, nil
}

// copyUserSkills preserves project-owned skills alongside generated Atlas skills.
func copyUserSkills(projectRoot string, targetRoot string) ([]string, error) {
	projectSkills, err := ProjectSkills(projectRoot)
	if err != nil {
		return nil, err
	}

	paths := []string{}
	for _, skill := range projectSkills {
		if skill.File {
			target := filepath.Join(targetRoot, filepath.Base(skill.Path))
			if err := copyFile(skill.Path, target); err != nil {
				return nil, err
			}
			paths = append(paths, target)
			continue
		}
		source := filepath.Dir(skill.Path)
		target := filepath.Join(targetRoot, skill.Name)
		copied, err := copyDir(source, target)
		if err != nil {
			return nil, err
		}
		paths = append(paths, copied...)
	}
	return paths, nil
}

// writeCopilotUserSkills adapts project-owned SKILL.md files to Copilot instructions.
func writeCopilotUserSkills(projectRoot string, targetRoot string) ([]string, error) {
	projectSkills, err := ProjectSkills(projectRoot)
	if err != nil {
		return nil, err
	}

	paths := []string{}
	for _, skill := range projectSkills {
		target := filepath.Join(targetRoot, skill.Name+".instructions.md")
		if err := copyFile(skill.Path, target); err != nil {
			return nil, err
		}
		paths = append(paths, target)
	}
	return paths, nil
}

// writeGeminiUserSkills adapts project-owned SKILL.md files to Gemini context files.
func writeGeminiUserSkills(projectRoot string, targetRoot string) ([]string, error) {
	projectSkills, err := ProjectSkills(projectRoot)
	if err != nil {
		return nil, err
	}

	paths := []string{}
	for _, skill := range projectSkills {
		target := filepath.Join(targetRoot, skill.Name, "GEMINI.md")
		if err := copyFile(skill.Path, target); err != nil {
			return nil, err
		}
		paths = append(paths, target)
	}
	return paths, nil
}

// cleanGenerated removes stale generated files before writing the current catalog.
func cleanGenerated(root string, suffix string) error {
	entries, err := os.ReadDir(root)
	if err != nil {
		return err
	}
	for _, entry := range entries {
		path := filepath.Join(root, entry.Name())
		if entry.IsDir() {
			if fileExists(filepath.Join(path, "SKILL.md")) {
				if err := os.RemoveAll(path); err != nil {
					return err
				}
			}
			continue
		}
		if strings.HasSuffix(entry.Name(), suffix) {
			if err := os.Remove(path); err != nil {
				return err
			}
		}
	}
	return nil
}

// cleanGeminiGenerated removes generated Gemini context directories before syncing current skills.
func cleanGeminiGenerated(root string) error {
	entries, err := os.ReadDir(root)
	if err != nil {
		return err
	}
	for _, entry := range entries {
		path := filepath.Join(root, entry.Name())
		if entry.IsDir() && fileExists(filepath.Join(path, "GEMINI.md")) {
			if err := os.RemoveAll(path); err != nil {
				return err
			}
		}
	}
	return nil
}

// copyDir copies user-owned skill directories without interpreting their contents.
func copyDir(source string, target string) ([]string, error) {
	paths := []string{}
	err := filepath.WalkDir(source, func(path string, entry os.DirEntry, err error) error {
		if err != nil {
			return err
		}
		rel, err := filepath.Rel(source, path)
		if err != nil {
			return err
		}
		targetPath := filepath.Join(target, rel)
		if entry.IsDir() {
			return os.MkdirAll(targetPath, 0o755)
		}
		if err := copyFile(path, targetPath); err != nil {
			return err
		}
		paths = append(paths, targetPath)
		return nil
	})
	return paths, err
}

// copyFile keeps user-owned skill sync simple and platform-neutral.
func copyFile(source string, target string) error {
	in, err := os.Open(source)
	if err != nil {
		return err
	}
	defer in.Close()

	if err := os.MkdirAll(filepath.Dir(target), 0o755); err != nil {
		return err
	}
	out, err := os.Create(target)
	if err != nil {
		return err
	}
	defer out.Close()

	_, err = io.Copy(out, in)
	return err
}

// skillMarkdown renders a built-in skill in the common SKILL.md shape.
func skillMarkdown(skill Skill) string {
	return "# " + title(skill.Name) + "\n\n" + skill.Description + "\n\n" + skill.Content
}

// copilotInstruction renders a built-in skill as a Copilot instruction file.
func copilotInstruction(skill Skill) string {
	return "---\napplyTo: \"**/*\"\n---\n\n# " + title(skill.Name) + "\n\n" + skill.Description + "\n\n" + skill.Content
}

// copilotPrompt renders a workflow prompt for Copilot.
func copilotPrompt(prompt Prompt) string {
	return "# " + title(prompt.Name) + "\n\n" + prompt.Description + "\n\n" + prompt.Content
}

// title converts stable skill ids into readable Markdown titles.
func title(name string) string {
	parts := strings.Split(name, "-")
	for i, part := range parts {
		if part == "" {
			continue
		}
		parts[i] = strings.ToUpper(part[:1]) + part[1:]
	}
	return strings.Join(parts, " ")
}

// fileExists keeps user skill sync tolerant of optional SKILL.md files.
func fileExists(path string) bool {
	info, err := os.Stat(path)
	return err == nil && !info.IsDir()
}
