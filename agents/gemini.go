package agents

import (
	"context"
	"os"
	"os/exec"
	"path/filepath"
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
	return writeJSONServer(g.MCPConfigPath(root), "mcpServers", server)
}

// RemoveMCPConfig removes Atlas's Gemini MCP server while preserving unrelated settings.
func (g Gemini) RemoveMCPConfig(_ context.Context, root string, serverName string) error {
	return removeJSONServer(g.MCPConfigPath(root), "mcpServers", serverName)
}
