package agents

import (
	"context"
	"fmt"
	"os"
	"path/filepath"

	"github.com/goforj/atlas/files"
)

// Codex writes project-local integration files for OpenAI Codex.
type Codex struct{}

// Name returns the stable adapter id.
func (Codex) Name() string { return "codex" }

// DisplayName returns the user-facing adapter name.
func (Codex) DisplayName() string { return "Codex" }

// DetectSystem reports whether Codex appears to be configured on this machine.
func (Codex) DetectSystem(context.Context) bool {
	home, err := os.UserHomeDir()
	return err == nil && dirExists(filepath.Join(home, ".codex"))
}

// DetectProject reports whether the project already has Codex instructions.
func (Codex) DetectProject(root string) bool {
	return fileExists(filepath.Join(root, "AGENTS.md")) || dirExists(filepath.Join(root, ".codex"))
}

// GuidelinesPath returns the Codex guideline file path.
func (Codex) GuidelinesPath(root string) string { return filepath.Join(root, "AGENTS.md") }

// SkillsPath returns the Codex skill directory path.
func (Codex) SkillsPath(root string) string { return filepath.Join(root, ".agents", "skills") }

// MCPConfigPath returns the Codex MCP config path.
func (Codex) MCPConfigPath(root string) string { return filepath.Join(root, ".codex", "config.toml") }

// WriteMCPConfig writes the Codex MCP server configuration.
func (c Codex) WriteMCPConfig(_ context.Context, root string, server MCPServerConfig) error {
	content := fmt.Sprintf("[mcp_servers.%s]\ncommand = %q\nargs = [%q]\ncwd = %q\n",
		server.Name,
		server.Command,
		server.Args()[0],
		server.CWD,
	)
	return files.WriteFile(c.MCPConfigPath(root), []byte(content))
}
