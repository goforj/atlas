package agents

import (
	"context"
	"encoding/json"
	"errors"
	"os"
	"os/exec"
	"path/filepath"

	"github.com/goforj/atlas/files"
)

// Gemini writes project-local integration files for Gemini CLI.
type Gemini struct{}

// Name returns the stable adapter id.
func (Gemini) Name() string { return "gemini" }

// DisplayName returns the user-facing adapter name.
func (Gemini) DisplayName() string { return "Gemini CLI" }

// DetectSystem reports whether Gemini CLI appears to be configured or installed.
func (Gemini) DetectSystem(context.Context) bool {
	home, err := os.UserHomeDir()
	if err == nil && dirExists(filepath.Join(home, ".gemini")) {
		return true
	}
	_, err = exec.LookPath("gemini")
	return err == nil
}

// DetectProject reports whether the project already has Gemini CLI files.
func (Gemini) DetectProject(root string) bool {
	return fileExists(filepath.Join(root, "GEMINI.md")) || dirExists(filepath.Join(root, ".gemini"))
}

// GuidelinesPath returns the Gemini guideline file path.
func (Gemini) GuidelinesPath(root string) string { return filepath.Join(root, "GEMINI.md") }

// SkillsPath returns the Gemini skill context directory path.
func (Gemini) SkillsPath(root string) string { return filepath.Join(root, ".gemini", "skills") }

// MCPConfigPath returns the Gemini project settings path.
func (Gemini) MCPConfigPath(root string) string {
	return filepath.Join(root, ".gemini", "settings.json")
}

// WriteMCPConfig writes the Gemini MCP server configuration.
func (g Gemini) WriteMCPConfig(_ context.Context, root string, server MCPServerConfig) error {
	settings := map[string]any{}
	path := g.MCPConfigPath(root)
	content, err := os.ReadFile(path)
	if err == nil && len(content) > 0 {
		if err := json.Unmarshal(content, &settings); err != nil {
			return err
		}
	} else if err != nil && !errors.Is(err, os.ErrNotExist) {
		return err
	}

	servers, ok := settings["mcpServers"].(map[string]any)
	if !ok {
		servers = map[string]any{}
		settings["mcpServers"] = servers
	}
	servers[server.Name] = map[string]any{
		"command": server.Command,
		"args":    server.Args(),
		"cwd":     server.CWD,
	}

	updated, err := json.MarshalIndent(settings, "", "  ")
	if err != nil {
		return err
	}
	updated = append(updated, '\n')
	return files.WriteFile(path, updated)
}
