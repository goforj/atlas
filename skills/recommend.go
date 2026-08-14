package skills

import (
	"strings"

	"github.com/goforj/atlas/project"
)

// Recommended returns the built-in skills enabled for a project.
func Recommended(p project.Project) []Skill {
	if p.Name == "" && len(p.Components) == 0 && p.FrontendKit == "" {
		return Catalog()
	}
	enabled := map[string]bool{
		"goforj-app-architecture":       true,
		"goforj-app-registration":       true,
		"goforj-make-commands":          true,
		"goforj-go-package-design":      true,
		"goforj-library-selection":      true,
		"goforj-runtime-workflows":      true,
		"goforj-testing-and-validation": true,
		"goforj-wire-repair":            true,
		"goforj-debug-runtime":          true,
		"goforj-multi-app-change":       len(p.Apps) > 1,
		"goforj-validate-change":        true,
	}
	if hasComponent(p, "web-api") || hasComponent(p, "web-ui") {
		enabled["goforj-add-http-route"] = true
	}
	if hasComponent(p, "cli") {
		enabled["goforj-add-app-command"] = true
	}
	if hasComponent(p, "jobs") {
		enabled["goforj-add-job"] = true
	}
	if hasComponent(p, "scheduler") {
		enabled["goforj-add-schedule"] = true
	}
	if hasComponent(p, "events") {
		enabled["goforj-add-event-workflow"] = true
	}
	if hasAnyComponent(p, "database", "cache", "storage") {
		enabled["goforj-database-and-data-access"] = true
		enabled["goforj-migrations"] = true
		enabled["goforj-add-data-resource"] = true
	}
	if hasAnyComponent(p, "metrics", "observability") {
		enabled["goforj-observability"] = true
	}
	switch strings.ToLower(strings.TrimSpace(p.FrontendKit)) {
	case "vue":
		enabled["goforj-vue-starter-kit"] = true
	case "react":
		enabled["goforj-react-starter-kit"] = true
	case "templ", "templ-htmx", "templ_htmx", "htmx":
		enabled["goforj-templ-htmx-starter-kit"] = true
	}

	out := []Skill{}
	for _, skill := range Catalog() {
		if enabled[skill.Name] {
			out = append(out, skill)
		}
	}
	return out
}

// RecommendedNames returns the project-enabled skill names.
func RecommendedNames(p project.Project) []string {
	skills := Recommended(p)
	names := make([]string, 0, len(skills))
	for _, skill := range skills {
		names = append(names, skill.Name)
	}
	return names
}

func hasAnyComponent(p project.Project, parts ...string) bool {
	for _, part := range parts {
		if hasComponent(p, part) {
			return true
		}
	}
	return false
}

func hasComponent(p project.Project, part string) bool {
	part = strings.ToLower(part)
	for _, component := range p.Components {
		if strings.Contains(strings.ToLower(component), part) {
			return true
		}
	}
	return false
}
