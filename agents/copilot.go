package agents

import (
	"context"
	"os"
	"path/filepath"
	"strings"
)

// Copilot writes project-local integration files for GitHub Copilot in VS Code.
type Copilot struct{}

// Name returns the stable adapter id.
func (Copilot) Name() string { return "copilot" }

// DisplayName returns the user-facing adapter name.
func (Copilot) DisplayName() string { return "GitHub Copilot" }

// DetectSystem reports whether Copilot should be offered.
func (Copilot) DetectSystem(context.Context) bool {
	home, err := os.UserHomeDir()
	if err != nil {
		return false
	}
	for _, root := range []string{filepath.Join(home, ".vscode", "extensions"), filepath.Join(home, ".vscode-insiders", "extensions")} {
		entries, err := os.ReadDir(root)
		if err != nil {
			continue
		}
		for _, entry := range entries {
			if strings.HasPrefix(strings.ToLower(entry.Name()), "github.copilot-") {
				return true
			}
		}
	}
	return false
}

// DetectProject reports whether the project already has Copilot files.
func (Copilot) DetectProject(root string) bool {
	return fileExists(filepath.Join(root, ".github", "copilot-instructions.md")) || dirExists(filepath.Join(root, ".vscode"))
}

// GuidelinesPath returns the Copilot guideline file path.
func (Copilot) GuidelinesPath(root string) string {
	return filepath.Join(root, ".github", "copilot-instructions.md")
}

// SkillsPath returns the Copilot instruction directory path.
func (Copilot) SkillsPath(root string) string {
	return filepath.Join(root, ".github", "instructions")
}

// PromptsPath returns the Copilot prompt directory path.
func (Copilot) PromptsPath(root string) string {
	return filepath.Join(root, ".github", "prompts")
}

// MCPConfigPath returns the VS Code MCP config path used by Copilot.
func (Copilot) MCPConfigPath(root string) string { return filepath.Join(root, ".vscode", "mcp.json") }

// WriteMCPConfig writes the VS Code MCP server configuration.
func (c Copilot) WriteMCPConfig(_ context.Context, root string, server MCPServerConfig) error {
	return writeJSONServer(c.MCPConfigPath(root), "servers", server)
}

// RemoveMCPConfig removes Atlas's VS Code MCP server while preserving unrelated entries.
func (c Copilot) RemoveMCPConfig(_ context.Context, root string, serverName string) error {
	return removeJSONServer(c.MCPConfigPath(root), "servers", serverName)
}
