package agents

import (
	"context"
	"os"
	"path/filepath"
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
	return writeJSONServer(c.MCPConfigPath(root), "mcpServers", server)
}

// RemoveMCPConfig removes Atlas's Claude MCP server while preserving unrelated entries.
func (c Claude) RemoveMCPConfig(_ context.Context, root string, serverName string) error {
	return removeJSONServer(c.MCPConfigPath(root), "mcpServers", serverName)
}
