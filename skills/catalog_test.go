package skills

import (
	"strings"
	"testing"

	"github.com/goforj/atlas/project"
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
		"goforj-react-starter-kit",
		"goforj-templ-htmx-starter-kit",
		"goforj-testing-and-validation",
		"goforj-add-http-route",
		"goforj-add-app-command",
		"goforj-add-job",
		"goforj-add-schedule",
		"goforj-add-event-workflow",
		"goforj-add-data-resource",
		"goforj-wire-repair",
		"goforj-debug-runtime",
		"goforj-multi-app-change",
		"goforj-validate-change",
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
		"can stand on its own",
		"File count, type count, and directory symmetry do not justify a package boundary",
		"migrations/<app>/<connection>/",
		"GOCACHE=/tmp/gocache",
		"render test projects in '/tmp'",
		"workflow-plan",
		"registration-points",
		"wire-diagnostics",
		"application-info",
		"route-list",
		"cmd/<app>/frontend",
		"templ/htmx",
	} {
		if !strings.Contains(all, want) {
			t.Fatalf("catalog missing %q", want)
		}
	}
}

func TestWorkflowSkillQualitySections(t *testing.T) {
	for _, name := range []string{
		"goforj-add-http-route",
		"goforj-add-app-command",
		"goforj-add-job",
		"goforj-add-schedule",
		"goforj-add-event-workflow",
		"goforj-add-data-resource",
		"goforj-wire-repair",
		"goforj-debug-runtime",
		"goforj-multi-app-change",
		"goforj-validate-change",
	} {
		skill, ok := ByName(name)
		if !ok {
			t.Fatalf("missing workflow skill %s", name)
		}
		content := skill.Content
		assertWorkflowSkillContains(t, name, content, "Use this skill")
		assertWorkflowSkillContains(t, name, content, "Atlas")
		assertWorkflowSkillContains(t, name, content, "forj")
		assertWorkflowSkillContains(t, name, content, "Verify")
		if !strings.Contains(content, "Do not") && !strings.Contains(content, "should not") {
			t.Fatalf("%s missing mistake guidance", name)
		}
	}
}

func TestRecommendedSkillsAreProjectTailored(t *testing.T) {
	names := RecommendedNames(project.Project{
		Name:        "demo",
		Components:  []string{"cli", "web-api", "jobs"},
		FrontendKit: "react",
		Apps:        []project.App{{Name: "app", Default: true}, {Name: "marketplace"}},
	})
	for _, want := range []string{"goforj-react-starter-kit", "goforj-add-http-route", "goforj-add-app-command", "goforj-add-job", "goforj-multi-app-change"} {
		if !containsName(names, want) {
			t.Fatalf("recommended skills missing %s in %#v", want, names)
		}
	}
	for _, notWant := range []string{"goforj-vue-starter-kit", "goforj-add-schedule"} {
		if containsName(names, notWant) {
			t.Fatalf("recommended skills should not include %s in %#v", notWant, names)
		}
	}
}

func assertWorkflowSkillContains(t *testing.T, name string, content string, want string) {
	t.Helper()
	if !strings.Contains(content, want) {
		t.Fatalf("%s missing %q in %q", name, want, content)
	}
}

func containsName(names []string, want string) bool {
	for _, name := range names {
		if name == want {
			return true
		}
	}
	return false
}
