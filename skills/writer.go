package skills

import (
	"io"
	"os"
	"path/filepath"
	"strings"

	"github.com/goforj/atlas/agents"
	"github.com/goforj/atlas/files"
	"github.com/goforj/atlas/project"
)

// WriteOptions controls how skills are synchronized for an agent.
type WriteOptions struct {
	Root           string
	Agent          agents.Agent
	Project        project.Project
	IncludePrompts bool
	PreviousSkills []string
	PreviousFiles  []string
}

// PlannedPaths returns the files Write would manage without touching the filesystem.
func PlannedPaths(root string, agent agents.Agent, includePrompts bool, project project.Project) []string {
	opts := WriteOptions{Root: root, Agent: agent, Project: project, IncludePrompts: includePrompts}
	switch agent.Name() {
	case "copilot":
		paths := []string{}
		skillRoot := agent.SkillsPath(root)
		for _, skill := range Recommended(opts.Project) {
			paths = append(paths, filepath.Join(skillRoot, skill.Name+".instructions.md"))
		}
		if includePrompts {
			if copilot, ok := agent.(agents.Copilot); ok {
				promptRoot := copilot.PromptsPath(root)
				for _, prompt := range Prompts() {
					paths = append(paths, filepath.Join(promptRoot, prompt.Name+".prompt.md"))
				}
			}
		}
		return paths
	default:
		paths := []string{}
		root := opts.Agent.SkillsPath(opts.Root)
		for _, skill := range Recommended(opts.Project) {
			paths = append(paths, filepath.Join(root, skill.Name, "SKILL.md"))
		}
		return paths
	}
}

// Write writes built-in skills using the selected agent's native shape.
func Write(opts WriteOptions) ([]string, error) {
	if err := cleanRecordedFiles(opts); err != nil {
		return nil, err
	}
	switch opts.Agent.Name() {
	case "copilot":
		return writeCopilot(opts)
	default:
		return writeSkillMD(opts)
	}
}

// cleanRecordedFiles removes exact prior Atlas projections while preserving untracked native content.
func cleanRecordedFiles(opts WriteOptions) error {
	roots := []string{opts.Agent.SkillsPath(opts.Root)}
	if copilot, ok := opts.Agent.(agents.Copilot); ok {
		roots = append(roots, copilot.PromptsPath(opts.Root))
	}
	for _, recorded := range opts.PreviousFiles {
		path := recorded
		if !filepath.IsAbs(path) {
			path = filepath.Join(opts.Root, filepath.Clean(path))
		}
		if !withinAnyRoot(path, roots) {
			continue
		}
		if err := os.Remove(path); err != nil && !os.IsNotExist(err) {
			return err
		}
		_ = os.Remove(filepath.Dir(path))
	}
	return nil
}

// withinAnyRoot rejects recorded paths that escape the native skill directories.
func withinAnyRoot(path string, roots []string) bool {
	for _, root := range roots {
		relative, err := filepath.Rel(root, path)
		if err == nil && relative != "." && relative != ".." && !strings.HasPrefix(relative, ".."+string(filepath.Separator)) {
			return true
		}
	}
	return false
}

// writeSkillMD writes Codex/Claude-style SKILL.md directories.
func writeSkillMD(opts WriteOptions) ([]string, error) {
	root := opts.Agent.SkillsPath(opts.Root)
	if err := os.MkdirAll(root, 0o755); err != nil {
		return nil, err
	}
	if opts.Agent.Name() == "gemini" {
		if err := cleanLegacyGeminiGenerated(root, managedSkillNames(opts)); err != nil {
			return nil, err
		}
	}
	if err := cleanGenerated(root, ".md", managedSkillNames(opts)); err != nil {
		return nil, err
	}

	paths := []string{}
	for _, skill := range Recommended(opts.Project) {
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
	if err := cleanGenerated(root, ".instructions.md", managedSkillNames(opts)); err != nil {
		return nil, err
	}

	paths := []string{}
	for _, skill := range Recommended(opts.Project) {
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
			if err := cleanGenerated(promptRoot, ".prompt.md", promptNames()); err != nil {
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

// cleanGenerated removes only known Atlas projections before writing the current catalog.
func cleanGenerated(root string, suffix string, managedNames []string) error {
	managed := nameSet(managedNames)
	entries, err := os.ReadDir(root)
	if err != nil {
		return err
	}
	for _, entry := range entries {
		path := filepath.Join(root, entry.Name())
		if entry.IsDir() {
			generatedPath := filepath.Join(path, "SKILL.md")
			if managed[entry.Name()] && fileExists(generatedPath) {
				if err := os.Remove(generatedPath); err != nil {
					return err
				}
				_ = os.Remove(path)
			}
			continue
		}
		if strings.HasSuffix(entry.Name(), suffix) && managed[strings.TrimSuffix(entry.Name(), suffix)] {
			if err := os.Remove(path); err != nil {
				return err
			}
		}
	}
	return nil
}

// cleanLegacyGeminiGenerated removes Atlas's obsolete GEMINI.md skill projections during migration.
func cleanLegacyGeminiGenerated(root string, managedNames []string) error {
	managed := nameSet(managedNames)
	entries, err := os.ReadDir(root)
	if err != nil {
		return err
	}
	for _, entry := range entries {
		path := filepath.Join(root, entry.Name())
		generatedPath := filepath.Join(path, "GEMINI.md")
		if entry.IsDir() && managed[entry.Name()] && fileExists(generatedPath) {
			if err := os.Remove(generatedPath); err != nil {
				return err
			}
			_ = os.Remove(path)
		}
	}
	return nil
}

// managedSkillNames combines current, previous, and project-owned names so stale Atlas output can be replaced safely.
func managedSkillNames(opts WriteOptions) []string {
	names := append([]string{}, opts.PreviousSkills...)
	for _, skill := range Recommended(opts.Project) {
		names = append(names, skill.Name)
	}
	projectSkills, err := ProjectSkills(opts.Root)
	if err == nil {
		for _, skill := range projectSkills {
			names = append(names, skill.Name)
		}
	}
	return names
}

// promptNames returns Atlas-owned Copilot prompt names.
func promptNames() []string {
	names := make([]string, 0, len(Prompts()))
	for _, prompt := range Prompts() {
		names = append(names, prompt.Name)
	}
	return names
}

// nameSet supports exact ownership checks without treating every native skill as generated.
func nameSet(names []string) map[string]bool {
	set := map[string]bool{}
	for _, name := range names {
		set[name] = true
	}
	return set
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
	return "---\nname: " + skill.Name + "\ndescription: " + skill.Description + "\n---\n\n# " + title(skill.Name) + "\n\n" + skill.Content
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
