package install

import (
	"context"
	"fmt"
	"os"
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
	Discover      bool
	Guidelines    bool
	Skills        bool
	MCP           bool
	NoInteraction bool
	DryRun        bool
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
	var prior *config.Config
	if _, err := os.Stat(config.FilePath(opts.Root)); err == nil {
		loaded, err := config.Load(opts.Root)
		if err != nil {
			return Result{}, err
		}
		prior = &loaded
	} else if !os.IsNotExist(err) {
		return Result{}, err
	}
	return i.install(ctx, opts, prior, false)
}

// install writes selected surfaces and optionally preserves an existing update configuration.
func (i Installer) install(ctx context.Context, opts Options, prior *config.Config, preserveFeatures bool) (Result, error) {

	selected, err := i.selectAgents(ctx, opts)
	if err != nil {
		return Result{}, err
	}

	result := Result{}
	guidelineContent := guidelines.Compose(opts.Project)
	server := agents.DefaultMCPServerConfig(opts.Root)
	previousSkills := []string{}
	if prior != nil {
		previousSkills = prior.Skills
	}
	if prior != nil && !preserveFeatures {
		for _, agent := range selected {
			removed, err := removeAgentSurfaces(
				ctx,
				agent,
				*prior,
				agentRemovalOptions{
					Root:       opts.Root,
					Guidelines: prior.Features.Guidelines && !opts.Guidelines,
					MCP:        prior.Features.MCP && !opts.MCP,
					Skills:     prior.Features.Skills && !opts.Skills,
					DryRun:     opts.DryRun,
				},
			)
			if err != nil {
				return Result{}, err
			}
			result.Files = append(result.Files, removed...)
		}
	}

	for _, agent := range selected {
		result.Agents = append(result.Agents, agent.Name())
		if opts.Guidelines {
			path := agent.GuidelinesPath(opts.Root)
			if !opts.DryRun {
				if err := files.WriteMarkerFile(path, files.DefaultMarker, guidelineContent); err != nil {
					return Result{}, err
				}
			}
			result.Files = append(result.Files, path)
		}
		if opts.MCP {
			if !opts.DryRun {
				if err := agent.WriteMCPConfig(ctx, opts.Root, server); err != nil {
					return Result{}, err
				}
			}
			result.Files = append(result.Files, agent.MCPConfigPath(opts.Root))
		}
		if opts.Skills {
			paths := skills.PlannedPaths(opts.Root, agent, true, opts.Project)
			if !opts.DryRun {
				var err error
				paths, err = skills.Write(skills.WriteOptions{
					Root:           opts.Root,
					Agent:          agent,
					Project:        opts.Project,
					IncludePrompts: true,
					PreviousSkills: previousSkills,
					PreviousFiles:  previousGeneratedFiles(prior, agent.Name()),
				})
				if err != nil {
					return Result{}, err
				}
			}
			result.Files = append(result.Files, paths...)
		}
	}

	cfg := config.Default()
	if prior != nil {
		cfg = *prior
	}
	if preserveFeatures {
		cfg.Features.Guidelines = cfg.Features.Guidelines || opts.Guidelines
		cfg.Features.Skills = cfg.Features.Skills || opts.Skills
		cfg.Features.MCP = cfg.Features.MCP || opts.MCP
	} else {
		cfg.Features.Guidelines = opts.Guidelines
		cfg.Features.Skills = opts.Skills
		cfg.Features.MCP = opts.MCP
	}
	if cfg.GeneratedFiles == nil {
		cfg.GeneratedFiles = map[string][]string{}
	}
	if prior != nil {
		for _, name := range prior.Agents {
			if slices.Contains(result.Agents, name) {
				continue
			}
			delete(cfg.GeneratedFiles, name)
			agent, ok := agents.ByName(name)
			if !ok {
				continue
			}
			removed, err := removeAgent(ctx, opts.Root, agent, *prior, opts.DryRun)
			if err != nil {
				return Result{}, err
			}
			result.Files = append(result.Files, removed...)
		}
	}
	cfg.Agents = result.Agents
	cfg.Version = config.CurrentVersion
	cfg.Skills = skills.RecommendedNames(opts.Project)
	cfg.LastDiscovered.Apps = appNames(opts.Project)
	cfg.LastDiscovered.Components = slices.Clone(opts.Project.Components)
	for _, agent := range selected {
		paths := append(cfg.GeneratedFiles[agent.Name()], resultFilesForAgent(opts.Root, agent, result.Files)...)
		paths = enabledSurfacePaths(opts.Root, agent, cfg.Features, paths)
		cfg.GeneratedFiles[agent.Name()] = existingRelativePaths(opts.Root, paths)
	}

	if !opts.DryRun {
		if err := config.Save(opts.Root, cfg); err != nil {
			return Result{}, err
		}
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
	if !opts.Discover {
		for _, agent := range i.Agents {
			if agent.DetectProject(opts.Root) {
				selected = append(selected, agent)
			}
		}
	}
	if len(selected) > 0 {
		return selected, nil
	}
	for _, agent := range i.Agents {
		if agent.DetectSystem(ctx) {
			return []agents.Agent{agent}, nil
		}
	}
	for _, agent := range i.Agents {
		if agent.Name() == "codex" {
			return []agents.Agent{agent}, nil
		}
	}
	if len(i.Agents) > 0 {
		return []agents.Agent{i.Agents[0]}, nil
	}
	return nil, fmt.Errorf("no Atlas agent adapters are available")
}

// normalizeOptions applies Atlas defaults only when the caller did not select explicit surfaces.
func normalizeOptions(opts Options) Options {
	opts = normalizeProjectOptions(opts)
	if !opts.Guidelines && !opts.Skills && !opts.MCP {
		opts.Guidelines = true
		opts.Skills = true
		opts.MCP = true
	}
	return opts
}

// normalizeProjectOptions fills project paths and discovery without changing selected surfaces.
func normalizeProjectOptions(opts Options) Options {
	if opts.Project.Root == "" {
		opts.Project.Root = opts.Root
	}
	if opts.Root == "" {
		opts.Root = opts.Project.Root
	}
	opts.Project = opts.Project.WithDiscoveredDefaults()
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

// previousGeneratedFiles returns a copy of one agent's recorded Atlas projections.
func previousGeneratedFiles(cfg *config.Config, agentName string) []string {
	if cfg == nil {
		return nil
	}
	return append([]string(nil), cfg.GeneratedFiles[agentName]...)
}
