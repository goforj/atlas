package install

import (
	"context"
	"os"

	"github.com/goforj/atlas/config"
)

// Updater refreshes previously installed Atlas integration files.
type Updater struct {
	Installer Installer
}

// NewUpdater creates an updater with the built-in installer.
func NewUpdater() Updater {
	return Updater{Installer: NewInstaller()}
}

// Update reruns install with the supplied options.
func (u Updater) Update(ctx context.Context, opts Options) (Result, error) {
	if len(u.Installer.Agents) == 0 {
		u.Installer = NewInstaller()
	}
	opts = normalizeProjectOptions(opts)
	if _, err := os.Stat(config.FilePath(opts.Root)); os.IsNotExist(err) {
		return u.Installer.Install(ctx, opts)
	} else if err != nil {
		return Result{}, err
	}
	cfg, err := config.Load(opts.Root)
	if err != nil {
		return Result{}, err
	}
	applySurfaceSelections(&opts)
	explicitSurfaces := opts.GuidelinesSelection != nil || opts.SkillsSelection != nil || opts.MCPSelection != nil || opts.Guidelines || opts.Skills || opts.MCP
	if !explicitSurfaces {
		opts.Guidelines = cfg.Features.Guidelines
		opts.Skills = cfg.Features.Skills
		opts.MCP = cfg.Features.MCP
	}
	if !opts.Discover && !opts.AllAgents && len(opts.Agents) == 0 {
		opts.Agents = append([]string(nil), cfg.Agents...)
	}
	return u.Installer.install(ctx, opts, &cfg, explicitSurfaces)
}
