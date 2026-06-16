package skills

import (
	"strings"
	"testing"
)

func TestCatalogContainsRequiredSkills(t *testing.T) {
	for _, name := range []string{
		"goforj-app-architecture",
		"goforj-app-registration",
		"goforj-make-commands",
		"goforj-go-package-design",
		"goforj-migrations",
		"goforj-runtime-workflows",
		"goforj-database-and-data-access",
		"goforj-observability",
		"goforj-vue-starter-kit",
		"goforj-testing-and-validation",
	} {
		if _, ok := ByName(name); !ok {
			t.Fatalf("missing skill %s", name)
		}
	}
}

func TestSkillEffectivenessPhrases(t *testing.T) {
	all := ""
	for _, skill := range Catalog() {
		all += skill.Content + "\n"
	}

	for _, want := range []string{
		"cmd/app/main.go",
		"app/<name>/",
		"forj <app> make:*",
		"Java/PHP-style",
		"migrations/<app>/<connection>/",
		"GOCACHE=/tmp/gocache",
		"render test projects in '/tmp'",
	} {
		if !strings.Contains(all, want) {
			t.Fatalf("catalog missing %q", want)
		}
	}
}
