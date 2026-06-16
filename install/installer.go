package install

import (
	"context"
	"fmt"
	"slices"

	"github.com/goforj/atlas/agents"
	"github.com/goforj/atlas/config"
	"github.com/goforj/atlas/files"
	"github.com/goforj/atlas/guidelines"
	"github.com/goforj/atlas/project"
	"github.com/goforj/atlas/skills"
)

// Options controls an Atlas install or update run.
type Options struct {
	Root          string
	Project       project.Project
	Agents        []string
	AllAgents     bool
	Guidelines    bool
	Skills        bool
	MCP           bool
	NoInteraction bool
}

// Result describes the files and agents touched during install.
type Result struct {
	Agents []string
	Files  []string
}

// Installer installs Atlas project guidance and agent configuration.
type Installer struct {
	Agents []agents.Agent
}

// NewInstaller creates an Installer with built-in agents when none are provided.
func NewInstaller(adapters ...agents.Agent) Installer {
	if len(adapters) == 0 {
		adapters = agents.Builtins()
	}
	return Installer{Agents: adapters}
}

// Install writes Atlas guidance, MCP config, and project config.
func (i Installer) Install(ctx context.Context, opts Options) (Result, error) {
	opts = normalizeOptions(opts)

	selected, err := i.selectAgents(ctx, opts)
	if err != nil {
		return Result{}, err
	}

	result := Result{}
	guidelineContent := guidelines.Compose(opts.Project)
	server := agents.DefaultMCPServerConfig(opts.Root)

	for _, agent := range selected {
		result.Agents = append(result.Agents, agent.Name())
		if opts.Guidelines {
			path := agent.GuidelinesPath(opts.Root)
			if err := files.WriteMarkerFile(path, files.DefaultMarker, guidelineContent); err != nil {
				return Result{}, err
			}
			result.Files = append(result.Files, path)
		}
		if opts.MCP {
			if err := agent.WriteMCPConfig(ctx, opts.Root, server); err != nil {
				return Result{}, err
			}
			result.Files = append(result.Files, agent.MCPConfigPath(opts.Root))
		}
		if opts.Skills {
			paths, err := skills.Write(skills.WriteOptions{
				Root:           opts.Root,
				Agent:          agent,
				IncludePrompts: true,
			})
			if err != nil {
				return Result{}, err
			}
			result.Files = append(result.Files, paths...)
		}
	}

	cfg := config.Default()
	cfg.Features.Guidelines = opts.Guidelines
	cfg.Features.Skills = opts.Skills
	cfg.Features.MCP = opts.MCP
	cfg.Agents = result.Agents
	cfg.Skills = skills.Names()
	cfg.LastDiscovered.Apps = appNames(opts.Project)
	cfg.LastDiscovered.Components = slices.Clone(opts.Project.Components)

	if err := config.Save(opts.Root, cfg); err != nil {
		return Result{}, err
	}
	result.Files = append(result.Files, config.FilePath(opts.Root))

	return result, nil
}

// selectAgents keeps non-interactive installs deterministic while still honoring existing project files.
func (i Installer) selectAgents(ctx context.Context, opts Options) ([]agents.Agent, error) {
	if opts.AllAgents {
		return slices.Clone(i.Agents), nil
	}

	if len(opts.Agents) > 0 {
		selected := make([]agents.Agent, 0, len(opts.Agents))
		for _, name := range opts.Agents {
			agent, ok := agents.ByName(name)
			if !ok {
				return nil, fmt.Errorf("unknown agent %q", name)
			}
			selected = append(selected, agent)
		}
		return selected, nil
	}

	selected := []agents.Agent{}
	for _, agent := range i.Agents {
		if agent.DetectProject(opts.Root) || agent.DetectSystem(ctx) {
			selected = append(selected, agent)
		}
	}
	if len(selected) == 0 {
		selected = append(selected, agents.Codex{})
	}
	return selected, nil
}

// normalizeOptions applies Atlas defaults only when the caller did not select explicit surfaces.
func normalizeOptions(opts Options) Options {
	if opts.Project.Root == "" {
		opts.Project.Root = opts.Root
	}
	if opts.Root == "" {
		opts.Root = opts.Project.Root
	}
	opts.Project = opts.Project.WithDiscoveredDefaults()
	if !opts.Guidelines && !opts.Skills && !opts.MCP {
		opts.Guidelines = true
		opts.Skills = true
		opts.MCP = true
	}
	return opts
}

// appNames snapshots discovered app names into project-owned Atlas config.
func appNames(p project.Project) []string {
	apps := p.WithDiscoveredDefaults().Apps
	names := make([]string, 0, len(apps))
	for _, app := range apps {
		names = append(names, app.Name)
	}
	return names
}
