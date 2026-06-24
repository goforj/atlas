package install

import (
	"context"
	"os"
	"path/filepath"
	"slices"

	"github.com/goforj/atlas/agents"
	"github.com/goforj/atlas/config"
	atlasdocs "github.com/goforj/atlas/docs"
	"github.com/goforj/atlas/project"
	"github.com/goforj/atlas/skills"
)

// StatusOptions controls Atlas install status inspection.
type StatusOptions struct {
	Root    string
	Project project.Project
	Docs    atlasdocs.Provider
	Agents  []agents.Agent
}

// StatusResult describes whether Atlas is installed, current, and aligned.
type StatusResult struct {
	Installed     bool          `json:"installed"`
	Project       string        `json:"project,omitempty"`
	GoForjVersion string        `json:"goforj_version,omitempty"`
	DocsVersion   string        `json:"docs_version,omitempty"`
	DocsRevision  string        `json:"docs_revision,omitempty"`
	Agents        []AgentStatus `json:"agents,omitempty"`
	Skills        SkillStatus   `json:"skills"`
	Warnings      []string      `json:"warnings,omitempty"`
}

// AgentStatus describes one agent's Atlas install state.
type AgentStatus struct {
	Name              string `json:"name"`
	Configured        bool   `json:"configured"`
	GuidelinesPresent bool   `json:"guidelines_present"`
	MCPPresent        bool   `json:"mcp_present"`
	SkillsPresent     bool   `json:"skills_present"`
}

// SkillStatus describes generated Atlas skill freshness.
type SkillStatus struct {
	Expected int      `json:"expected"`
	Present  int      `json:"present"`
	Stale    bool     `json:"stale"`
	Missing  []string `json:"missing,omitempty"`
}

// Status inspects local Atlas installation state without modifying files.
func Status(ctx context.Context, opts StatusOptions) (StatusResult, error) {
	if len(opts.Agents) == 0 {
		opts.Agents = agents.Builtins()
	}
	if opts.Root == "" {
		opts.Root = opts.Project.Root
	}
	if opts.Project.Root == "" {
		opts.Project.Root = opts.Root
	}
	opts.Project = opts.Project.WithDiscoveredDefaults()
	cfg, err := config.Load(opts.Root)
	if err != nil {
		return StatusResult{}, err
	}

	manifest := atlasdocs.Manifest{}
	if opts.Docs != nil {
		manifest, _ = opts.Docs.Manifest(ctx)
	}

	result := StatusResult{
		Installed:     configExists(opts.Root),
		Project:       opts.Project.Name,
		GoForjVersion: opts.Project.GoForjVersion,
		DocsVersion:   manifest.Version,
		DocsRevision:  manifest.Revision,
		Skills:        skillStatus(opts.Root, opts.Agents, cfg, opts.Project),
	}
	for _, agent := range opts.Agents {
		configured := slices.Contains(cfg.Agents, agent.Name())
		result.Agents = append(result.Agents, AgentStatus{
			Name:              agent.Name(),
			Configured:        configured,
			GuidelinesPresent: fileExists(agent.GuidelinesPath(opts.Root)),
			MCPPresent:        fileExists(agent.MCPConfigPath(opts.Root)),
			SkillsPresent:     dirExists(agent.SkillsPath(opts.Root)),
		})
	}
	if !result.Installed {
		result.Warnings = append(result.Warnings, "Atlas project config is missing. Run forj atlas:install.")
	}
	if result.Skills.Stale {
		result.Warnings = append(result.Warnings, "Installed skill list differs from the current Atlas catalog. Run forj atlas:update.")
	}
	for _, agent := range result.Agents {
		if agent.Configured && (!agent.GuidelinesPresent || !agent.MCPPresent || !agent.SkillsPresent) {
			result.Warnings = append(result.Warnings, "Configured agent "+agent.Name+" is missing one or more Atlas files.")
		}
	}
	return result, nil
}

func skillStatus(root string, agentList []agents.Agent, cfg config.Config, p project.Project) SkillStatus {
	names := skills.RecommendedNames(p)
	status := SkillStatus{Expected: len(names), Stale: !sameStrings(cfg.Skills, names)}
	if len(agentList) == 0 {
		return status
	}
	agent := agentList[0]
	for _, name := range names {
		if generatedSkillExists(root, agent, name) {
			status.Present++
		} else {
			status.Missing = append(status.Missing, name)
		}
	}
	if len(status.Missing) > 0 {
		status.Stale = true
	}
	return status
}

func generatedSkillExists(root string, agent agents.Agent, name string) bool {
	switch agent.Name() {
	case "copilot":
		return fileExists(filepath.Join(agent.SkillsPath(root), name+".instructions.md"))
	case "gemini":
		return fileExists(filepath.Join(agent.SkillsPath(root), name, "GEMINI.md"))
	default:
		return fileExists(filepath.Join(agent.SkillsPath(root), name, "SKILL.md"))
	}
}

func configExists(root string) bool {
	return fileExists(config.FilePath(root))
}

func fileExists(path string) bool {
	_, err := os.Stat(path)
	return err == nil
}

func dirExists(path string) bool {
	info, err := os.Stat(path)
	return err == nil && info.IsDir()
}

func sameStrings(left []string, right []string) bool {
	if len(left) != len(right) {
		return false
	}
	for i := range left {
		if left[i] != right[i] {
			return false
		}
	}
	return true
}
