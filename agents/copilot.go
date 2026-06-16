package agents

import (
	"context"
	"encoding/json"
	"path/filepath"

	"github.com/goforj/atlas/files"
)

// Copilot writes project-local integration files for GitHub Copilot in VS Code.
type Copilot struct{}

// Name returns the stable adapter id.
func (Copilot) Name() string { return "copilot" }

// DisplayName returns the user-facing adapter name.
func (Copilot) DisplayName() string { return "GitHub Copilot" }

// DetectSystem reports whether Copilot should be offered.
func (Copilot) DetectSystem(context.Context) bool {
	return true
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
	payload := map[string]any{
		"servers": map[string]any{
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
