package agents

import (
	"context"
)

// MCPServerConfig describes the project-level Atlas MCP server for an agent.
type MCPServerConfig struct {
	Name    string `json:"name"`
	Command string `json:"command"`
	CWD     string `json:"cwd"`
}

// Args returns the stable command arguments for running Atlas through GoForj.
func (c MCPServerConfig) Args() []string {
	return []string{"atlas:mcp"}
}

// DefaultMCPServerConfig returns the conventional GoForj Atlas MCP server config.
func DefaultMCPServerConfig(root string) MCPServerConfig {
	return MCPServerConfig{
		Name:    "goforj-atlas",
		Command: "forj",
		CWD:     ".",
	}
}

// Agent writes project-local integration files for one supported local agent.
type Agent interface {
	// Name returns the stable adapter id used in Atlas config.
	Name() string
	// DisplayName returns the user-facing agent name.
	DisplayName() string
	// DetectSystem reports whether the agent is installed or configured on this machine.
	DetectSystem(context.Context) bool
	// DetectProject reports whether project-local files imply this agent should be configured.
	DetectProject(root string) bool
	// GuidelinesPath returns the project-local instruction file path.
	GuidelinesPath(root string) string
	// SkillsPath returns the project-local skill or instruction directory path.
	SkillsPath(root string) string
	// MCPConfigPath returns the project-local MCP configuration file path.
	MCPConfigPath(root string) string
	// WriteMCPConfig writes the agent-specific MCP server configuration.
	WriteMCPConfig(ctx context.Context, root string, server MCPServerConfig) error
}

// ByName returns the supported agent adapter for name.
func ByName(name string) (Agent, bool) {
	for _, agent := range Builtins() {
		if agent.Name() == name {
			return agent, true
		}
	}
	return nil, false
}

// Builtins returns the first-class Atlas agent adapters.
func Builtins() []Agent {
	return []Agent{
		Codex{},
		Claude{},
		Copilot{},
		Gemini{},
	}
}
