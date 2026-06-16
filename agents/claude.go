package agents

import (
	"context"
	"encoding/json"
	"os"
	"path/filepath"

	"github.com/goforj/atlas/files"
)

// Claude writes project-local integration files for Claude Code.
type Claude struct{}

// Name returns the stable adapter id.
func (Claude) Name() string { return "claude" }

// DisplayName returns the user-facing adapter name.
func (Claude) DisplayName() string { return "Claude Code" }

// DetectSystem reports whether Claude Code appears to be configured.
func (Claude) DetectSystem(context.Context) bool {
	home, err := os.UserHomeDir()
	return err == nil && dirExists(filepath.Join(home, ".claude"))
}

// DetectProject reports whether the project already has Claude Code files.
func (Claude) DetectProject(root string) bool {
	return fileExists(filepath.Join(root, "CLAUDE.md")) || dirExists(filepath.Join(root, ".claude"))
}

// GuidelinesPath returns the Claude guideline file path.
func (Claude) GuidelinesPath(root string) string { return filepath.Join(root, "CLAUDE.md") }

// SkillsPath returns the Claude skill directory path.
func (Claude) SkillsPath(root string) string { return filepath.Join(root, ".claude", "skills") }

// MCPConfigPath returns the Claude MCP config path.
func (Claude) MCPConfigPath(root string) string { return filepath.Join(root, ".mcp.json") }

// WriteMCPConfig writes the Claude MCP server configuration.
func (c Claude) WriteMCPConfig(_ context.Context, root string, server MCPServerConfig) error {
	payload := map[string]any{
		"mcpServers": map[string]any{
			server.Name: map[string]any{
				"command": server.Command,
				"args":    server.Args(),
				"cwd":     server.CWD,
			},
		},
	}
	content, err := json.MarshalIndent(payload, "", "  ")
	if err != nil {
		return err
	}
	content = append(content, '\n')
	return files.WriteFile(c.MCPConfigPath(root), content)
}
