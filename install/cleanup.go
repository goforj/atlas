package install

import (
	"context"
	"os"
	"path/filepath"
	"sort"
	"strings"

	"github.com/goforj/atlas/agents"
	"github.com/goforj/atlas/config"
	"github.com/goforj/atlas/files"
	"github.com/goforj/atlas/skills"
)

// removeAgent removes Atlas-owned projections for a deselected agent without deleting user content.
func removeAgent(ctx context.Context, root string, agent agents.Agent, cfg config.Config, dryRun bool) ([]string, error) {
	return removeAgentSurfaces(ctx, root, agent, cfg, true, true, true, dryRun)
}

// removeAgentSurfaces removes selected Atlas surfaces while leaving every unselected surface untouched.
func removeAgentSurfaces(ctx context.Context, root string, agent agents.Agent, cfg config.Config, guidelines bool, mcp bool, skillFiles bool, dryRun bool) ([]string, error) {
	paths := []string{}
	guidelinePath := agent.GuidelinesPath(root)
	if content, err := os.ReadFile(guidelinePath); guidelines && err == nil && strings.Contains(string(content), "<!-- "+files.DefaultMarker+":start -->") {
		paths = append(paths, guidelinePath)
		if !dryRun {
			remaining := files.RemoveMarkerBlock(string(content), files.DefaultMarker)
			if remaining == "" {
				if err := os.Remove(guidelinePath); err != nil && !os.IsNotExist(err) {
					return nil, err
				}
			} else if err := os.WriteFile(guidelinePath, []byte(remaining), 0o644); err != nil {
				return nil, err
			}
		}
	} else if guidelines && err != nil && !os.IsNotExist(err) {
		return nil, err
	}

	mcpPath := agent.MCPConfigPath(root)
	if _, err := os.Stat(mcpPath); mcp && err == nil {
		paths = append(paths, mcpPath)
		if !dryRun {
			if remover, ok := agent.(agents.MCPConfigRemover); ok {
				if err := remover.RemoveMCPConfig(ctx, root, "goforj-atlas"); err != nil {
					return nil, err
				}
			}
		}
	} else if mcp && !os.IsNotExist(err) {
		return nil, err
	}

	managed := managedFiles(root, agent, cfg)
	for _, path := range managed {
		if !skillFiles {
			break
		}
		if path == guidelinePath || path == mcpPath {
			continue
		}
		if _, err := os.Stat(path); os.IsNotExist(err) {
			continue
		} else if err != nil {
			return nil, err
		}
		paths = append(paths, path)
		if !dryRun {
			if err := os.Remove(path); err != nil && !os.IsNotExist(err) {
				return nil, err
			}
			removeEmptyParents(filepath.Dir(path), root)
		}
	}
	if mcp && !dryRun {
		removeEmptyParents(filepath.Dir(mcpPath), root)
	}
	return paths, nil
}

// managedFiles resolves recorded generated files and safely falls back to the v1 skill catalog.
func managedFiles(root string, agent agents.Agent, cfg config.Config) []string {
	paths := []string{}
	for _, relative := range cfg.GeneratedFiles[agent.Name()] {
		if path, ok := projectPath(root, relative); ok {
			paths = append(paths, path)
		}
	}
	if len(paths) > 0 {
		return paths
	}
	for _, name := range cfg.Skills {
		switch agent.Name() {
		case "copilot":
			paths = append(paths, filepath.Join(agent.SkillsPath(root), name+".instructions.md"))
		case "gemini":
			paths = append(paths,
				filepath.Join(agent.SkillsPath(root), name, "SKILL.md"),
				filepath.Join(agent.SkillsPath(root), name, "GEMINI.md"),
			)
		default:
			paths = append(paths, filepath.Join(agent.SkillsPath(root), name, "SKILL.md"))
		}
	}
	if copilot, ok := agent.(agents.Copilot); ok {
		for _, prompt := range skills.Prompts() {
			paths = append(paths, filepath.Join(copilot.PromptsPath(root), prompt.Name+".prompt.md"))
		}
	}
	return paths
}

// projectPath accepts only generated paths that remain beneath the project root.
func projectPath(root string, relative string) (string, bool) {
	if filepath.IsAbs(relative) {
		return "", false
	}
	clean := filepath.Clean(relative)
	if clean == "." || clean == ".." || strings.HasPrefix(clean, ".."+string(filepath.Separator)) {
		return "", false
	}
	return filepath.Join(root, clean), true
}

// removeEmptyParents removes empty native integration directories but never crosses the project root.
func removeEmptyParents(dir string, root string) {
	root = filepath.Clean(root)
	for dir = filepath.Clean(dir); dir != root && dir != "."; dir = filepath.Dir(dir) {
		if err := os.Remove(dir); err != nil {
			return
		}
	}
}

// resultFilesForAgent returns generated paths belonging to one native adapter.
func resultFilesForAgent(root string, agent agents.Agent, paths []string) []string {
	prefixes := []string{agent.GuidelinesPath(root), agent.MCPConfigPath(root), agent.SkillsPath(root)}
	if copilot, ok := agent.(agents.Copilot); ok {
		prefixes = append(prefixes, copilot.PromptsPath(root))
	}
	matched := []string{}
	for _, path := range paths {
		for _, prefix := range prefixes {
			if path == prefix || strings.HasPrefix(path, prefix+string(filepath.Separator)) {
				matched = append(matched, path)
				break
			}
		}
	}
	return matched
}

// enabledSurfacePaths keeps the ownership manifest aligned with the configured Atlas features.
func enabledSurfacePaths(root string, agent agents.Agent, features config.Features, paths []string) []string {
	guidelinePath := agent.GuidelinesPath(root)
	mcpPath := agent.MCPConfigPath(root)
	skillRoot := agent.SkillsPath(root)
	promptRoot := ""
	if copilot, ok := agent.(agents.Copilot); ok {
		promptRoot = copilot.PromptsPath(root)
	}
	filtered := make([]string, 0, len(paths))
	for _, path := range paths {
		switch {
		case path == guidelinePath:
			if features.Guidelines {
				filtered = append(filtered, path)
			}
		case path == mcpPath:
			if features.MCP {
				filtered = append(filtered, path)
			}
		case strings.HasPrefix(path, skillRoot+string(filepath.Separator)), promptRoot != "" && strings.HasPrefix(path, promptRoot+string(filepath.Separator)):
			if features.Skills {
				filtered = append(filtered, path)
			}
		}
	}
	return filtered
}

// existingRelativePaths stores portable, unique generated file paths that still exist.
func existingRelativePaths(root string, paths []string) []string {
	set := map[string]bool{}
	for _, path := range paths {
		info, err := os.Stat(path)
		if err != nil || info.IsDir() {
			continue
		}
		relative, err := filepath.Rel(root, path)
		if err != nil {
			continue
		}
		if _, ok := projectPath(root, relative); ok {
			set[relative] = true
		}
	}
	result := make([]string, 0, len(set))
	for path := range set {
		result = append(result, path)
	}
	sort.Strings(result)
	return result
}
