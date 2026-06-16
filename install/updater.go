package install

import "context"

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
	return u.Installer.Install(ctx, opts)
}
