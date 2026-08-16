package guidelines

import (
	"strings"
	"testing"

	"github.com/goforj/atlas/project"
)

// TestComposeCarriesTheBaselineFrameworkContract protects the standalone guidance treatment from silent dilution.
func TestComposeCarriesTheBaselineFrameworkContract(t *testing.T) {
	content := Compose(project.Project{
		Name:          "invoices",
		GoForjVersion: "0.24.0",
		Apps: []project.App{
			{Name: "app", Default: true, Runtimes: []string{"http"}},
			{Name: "admin", Runtimes: []string{"http", "cli"}},
		},
	})
	for _, expected := range []string{
		"Inspect `.goforj.yml`",
		"use the matching `forj make:*` generator",
		"`forj admin make:controller reports`",
		"Never edit `wire_gen.go`",
		"flat, self-contained, and portable",
		"can stand on its own",
		"avoid Java/PHP-style category nesting",
		"Keep controllers, commands, jobs, and schedules thin",
		"database access behind repositories",
		"documentation matching the Project's GoForj version",
		"ask before inventing a convention",
		"`admin`: runtimes `http`, `cli`",
	} {
		if !strings.Contains(content, expected) {
			t.Fatalf("Compose() missing %q:\n%s", expected, content)
		}
	}
}

// TestComposePermitsIntentionalManualOwnership keeps generator-first guidance from becoming an inflexible prohibition.
func TestComposePermitsIntentionalManualOwnership(t *testing.T) {
	content := Compose(project.Project{Name: "minimal"})
	if !strings.Contains(content, "intentionally should not be registered") || !strings.Contains(content, "manual implementation") {
		t.Fatalf("Compose() omitted the advanced ownership escape hatch:\n%s", content)
	}
}
