package project

import "slices"

// DefaultAppName is the conventional name for the default GoForj app.
const DefaultAppName = "app"

// Project contains the project facts Atlas needs without importing GoForj.
type Project struct {
	Root           string   `json:"root"`
	Name           string   `json:"name"`
	GoVersion      string   `json:"go_version"`
	GoForjVersion  string   `json:"goforj_version"`
	DocsRevision   string   `json:"docs_revision,omitempty"`
	Components     []string `json:"components,omitempty"`
	Apps           []App    `json:"apps,omitempty"`
	FrontendKit    string   `json:"frontend_kit,omitempty"`
	DatabaseDriver string   `json:"database_driver,omitempty"`
	QueueDriver    string   `json:"queue_driver,omitempty"`
}

// App describes one runnable GoForj app boundary.
type App struct {
	Name     string   `json:"name"`
	Default  bool     `json:"default"`
	Runtimes []string `json:"runtimes,omitempty"`
}

// DefaultApp returns the default app, synthesizing one when discovery has not
// provided explicit app metadata.
func (p Project) DefaultApp() App {
	for _, app := range p.Apps {
		if app.Default || app.Name == DefaultAppName {
			app.Default = true
			return app
		}
	}

	return App{
		Name:     DefaultAppName,
		Default:  true,
		Runtimes: defaultRuntimes(p.Components),
	}
}

// AppByName returns the selected app and whether it was found.
func (p Project) AppByName(name string) (App, bool) {
	if name == "" {
		return p.DefaultApp(), true
	}

	for _, app := range p.Apps {
		if app.Name == name {
			return app, true
		}
	}

	if name == DefaultAppName && len(p.Apps) == 0 {
		return p.DefaultApp(), true
	}

	return App{}, false
}

// WithDiscoveredDefaults normalizes default app metadata and runtime lists.
func (p Project) WithDiscoveredDefaults() Project {
	if len(p.Apps) == 0 {
		p.Apps = []App{p.DefaultApp()}
		return p
	}

	hasDefault := false
	for i, app := range p.Apps {
		if app.Name == "" {
			p.Apps[i].Name = DefaultAppName
		}
		if p.Apps[i].Name == DefaultAppName {
			p.Apps[i].Default = true
			hasDefault = true
		}
		if len(p.Apps[i].Runtimes) == 0 {
			p.Apps[i].Runtimes = defaultRuntimes(p.Components)
		}
	}
	if !hasDefault {
		p.Apps = append([]App{p.DefaultApp()}, p.Apps...)
	}

	return p
}

// defaultRuntimes infers runtime hints from components without requiring GoForj imports.
func defaultRuntimes(components []string) []string {
	runtimes := []string{"http"}
	if slices.Contains(components, "jobs") {
		runtimes = append(runtimes, "jobs")
	}
	if slices.Contains(components, "scheduler") || slices.Contains(components, "schedules") {
		runtimes = append(runtimes, "scheduler")
	}
	return runtimes
}
