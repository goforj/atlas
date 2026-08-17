package install

import (
	"context"
	"os"
	"slices"

	"github.com/goforj/atlas/config"
	"github.com/goforj/atlas/project"
)

// HostRequest controls an installation whose native guidance files are reconciled by its host.
type HostRequest struct {
	Root          string
	Project       project.Project
	Agents        []string
	AllAgents     bool
	Discover      bool
	Guidelines    *bool
	Skills        *bool
	MCP           *bool
	NoInteraction bool
	DryRun        bool
}

// HostResult describes Atlas-owned files and the native guidance reconciliation requested from the host.
type HostResult struct {
	Agents   []string
	Files    []string
	Guidance GuidanceReconciliation
}

// GuidanceReconciliation describes the native guideline projection that the host must reconcile.
type GuidanceReconciliation struct {
	Version int      `json:"version"`
	Enabled bool     `json:"enabled"`
	Targets []string `json:"targets"`
}

// GuidanceReconciliationVersion identifies the current host reconciliation contract.
const GuidanceReconciliationVersion = 1

// HostInstaller coordinates Atlas state while leaving native guidance projection to the host.
type HostInstaller struct {
	Installer Installer
}

// NewHostInstaller creates a HostInstaller with built-in agents when none are provided.
func NewHostInstaller() HostInstaller {
	return HostInstaller{Installer: NewInstaller()}
}

// Install writes Atlas-owned surfaces and returns native guidance reconciliation intent.
func (i HostInstaller) Install(ctx context.Context, request HostRequest) (HostResult, error) {
	if len(i.Installer.Agents) == 0 {
		i.Installer = NewInstaller()
	}
	opts := hostOptions(request)
	var prior *config.Config
	if _, err := os.Stat(config.FilePath(opts.Root)); err == nil {
		loaded, err := config.Load(opts.Root)
		if err != nil {
			return HostResult{}, err
		}
		prior = &loaded
	} else if !os.IsNotExist(err) {
		return HostResult{}, err
	}
	result, err := i.Installer.install(ctx, opts, prior, false, false)
	if err != nil {
		return HostResult{}, err
	}
	return hostResult(result, opts.Guidelines), nil
}

// HostUpdater refreshes Atlas-owned integration files and returns host reconciliation intent.
type HostUpdater struct {
	Installer Installer
}

// NewHostUpdater creates a HostUpdater with the built-in installer.
func NewHostUpdater() HostUpdater {
	return HostUpdater{Installer: NewInstaller()}
}

// Update refreshes Atlas-owned surfaces without writing native guidance files.
func (u HostUpdater) Update(ctx context.Context, request HostRequest) (HostResult, error) {
	if len(u.Installer.Agents) == 0 {
		u.Installer = NewInstaller()
	}
	opts := hostOptions(request)
	if _, err := os.Stat(config.FilePath(opts.Root)); os.IsNotExist(err) {
		result, err := u.Installer.install(ctx, opts, nil, false, false)
		if err != nil {
			return HostResult{}, err
		}
		return hostResult(result, opts.Guidelines), nil
	} else if err != nil {
		return HostResult{}, err
	}
	cfg, err := config.Load(opts.Root)
	if err != nil {
		return HostResult{}, err
	}
	applyHostSelections(&opts, request, cfg)
	if !opts.Discover && !opts.AllAgents && len(opts.Agents) == 0 {
		opts.Agents = slices.Clone(cfg.Agents)
	}
	result, err := u.Installer.install(ctx, opts, &cfg, false, false)
	if err != nil {
		return HostResult{}, err
	}
	return hostResult(result, opts.Guidelines), nil
}

// hostOptions translates the host's explicit tri-state selections without changing legacy Options.
func hostOptions(request HostRequest) Options {
	opts := Options{
		Root: request.Root, Project: request.Project, Agents: request.Agents, AllAgents: request.AllAgents,
		Discover: request.Discover, NoInteraction: request.NoInteraction, DryRun: request.DryRun,
	}
	if request.Guidelines != nil || request.Skills != nil || request.MCP != nil {
		if request.Guidelines != nil {
			opts.Guidelines = *request.Guidelines
		}
		if request.Skills != nil {
			opts.Skills = *request.Skills
		}
		if request.MCP != nil {
			opts.MCP = *request.MCP
		}
		return normalizeProjectOptions(opts)
	}
	return normalizeOptions(opts)
}

// applyHostSelections preserves omitted host selections from committed Atlas state.
func applyHostSelections(opts *Options, request HostRequest, cfg config.Config) {
	if request.Guidelines == nil {
		opts.Guidelines = cfg.Features.Guidelines
	}
	if request.Skills == nil {
		opts.Skills = cfg.Features.Skills
	}
	if request.MCP == nil {
		opts.MCP = cfg.Features.MCP
	}
}

// hostResult exposes the reconciliation request without expanding Result's legacy layout.
func hostResult(result Result, guidelines bool) HostResult {
	return HostResult{
		Agents: result.Agents,
		Files:  result.Files,
		Guidance: GuidanceReconciliation{
			Version: GuidanceReconciliationVersion,
			Enabled: guidelines,
			Targets: slices.Clone(result.Agents),
		},
	}
}
