package skills

import (
	"sort"
	"strings"
)

// Skill describes one built-in Atlas guidance unit.
type Skill struct {
	Name        string
	Description string
	Content     string
}

// Prompt describes a workflow-oriented Copilot prompt.
type Prompt struct {
	Name        string
	Description string
	Content     string
}

// Catalog returns the built-in GoForj Atlas skills.
func Catalog() []Skill {
	return []Skill{
		skill("goforj-app-architecture", "Project, app, runtime, and package layout.", appArchitecture),
		skill("goforj-app-registration", "App composition and registration points.", appRegistration),
		skill("goforj-make-commands", "Use GoForj make commands for framework scaffolding.", makeCommands),
		skill("goforj-go-package-design", "Package-scoped Go code instead of class-style nesting.", goPackageDesign),
		skill("goforj-migrations", "Raw DDL and app-scoped migration ownership.", migrations),
		skill("goforj-runtime-workflows", "Build, run, dev, app prefixes, and binaries.", runtimeWorkflows),
		skill("goforj-database-and-data-access", "Repository/service boundaries and safe schema inspection.", databaseDataAccess),
		skill("goforj-observability", "Logs, metrics, app/runtime identity, and Lighthouse boundaries.", observability),
		skill("goforj-vue-starter-kit", "Vue starter kit paths, auth defaults, and embedded frontend assets.", vueStarterKit),
		skill("goforj-testing-and-validation", "Validation commands and render hygiene.", testingValidation),
	}
}

// Prompts returns workflow-oriented Copilot prompt templates.
func Prompts() []Prompt {
	return []Prompt{
		prompt("goforj-create-app", "Create a named GoForj app.", "Use 'forj make:app <name>' and explain the generated 'app/<name>' and 'cmd/<name>' files."),
		prompt("goforj-add-route", "Add an HTTP route.", "Prefer 'forj <app> make:controller <name>' when the route belongs to a GoForj app."),
		prompt("goforj-add-job", "Add a queue job.", "Prefer 'forj <app> make:job <name>' and wire through the selected app's job Wire file."),
		prompt("goforj-add-schedule", "Add a schedule.", "Prefer 'forj <app> make:schedule <name>' and register in the selected app's 'schedules.go'."),
		prompt("goforj-review-package-design", "Review Go package structure.", "Look for unnecessary Java/PHP-style nesting and prefer cohesive package scope."),
		prompt("goforj-debug-runtime", "Debug runtime startup.", "Inspect app/runtime identity, logs, routes, schedules, and Atlas read-only tools before guessing."),
		prompt("goforj-review-change", "Review a GoForj change.", "Check app registration points, make-command usage, package scope, tests, and '/tmp' render hygiene."),
	}
}

// Names returns sorted built-in skill names.
func Names() []string {
	catalog := Catalog()
	names := make([]string, 0, len(catalog))
	for _, skill := range catalog {
		names = append(names, skill.Name)
	}
	sort.Strings(names)
	return names
}

// ByName returns a built-in skill by name.
func ByName(name string) (Skill, bool) {
	for _, skill := range Catalog() {
		if skill.Name == name {
			return skill, true
		}
	}
	return Skill{}, false
}

// skill normalizes embedded skill content so writers do not duplicate whitespace rules.
func skill(name string, description string, content string) Skill {
	return Skill{
		Name:        name,
		Description: description,
		Content:     strings.TrimSpace(content) + "\n",
	}
}

// prompt normalizes embedded prompt content for Copilot output.
func prompt(name string, description string, content string) Prompt {
	return Prompt{
		Name:        name,
		Description: description,
		Content:     strings.TrimSpace(content) + "\n",
	}
}

// Built-in skill content intentionally lives close to the catalog so required
// concepts and fixture assertions evolve together.
const appArchitecture = `
# GoForj App Architecture

Use the project/app/runtime model:

- 'cmd/app/main.go' is the default app binary entrypoint.
- 'app/' is the default app composition package.
- 'app/wire/' is default app Wire assembly.
- 'cmd/<name>/main.go' is a named app binary entrypoint.
- 'app/<name>/' is named app composition.
- 'internal/' contains shared implementation and domain packages.

Do not move business logic into 'app/'. The app layer composes and exposes behavior.
`

const appRegistration = `
# GoForj App Registration

Routes, commands, schedules, lifecycle hooks, and app-specific Wire registration belong to the selected app.

Common default app files:

- 'app/routes.go'
- 'app/commands.go'
- 'app/lifecycle.go'
- 'app/schedules.go'
- 'app/wire/inject_http_controllers_app.go'
- 'app/wire/inject_cmd_app.go'
- 'app/wire/inject_jobs_app.go'
- 'app/wire/inject_repositories_app.go'
- 'app/wire/inject_schedules_app.go'
- 'app/wire/inject_services_app.go'
- 'app/wire/inject_subscribers_app.go'

For named apps, use 'app/<name>/...' and 'app/<name>/wire/...'.
`

const makeCommands = `
# GoForj Make Commands

Prefer GoForj make commands over raw file creation when a matching command exists.

Use unprefixed commands for the default app:

- 'forj make:controller users'
- 'forj make:job sync-users'
- 'forj make:schedule nightly-cleanup'
- 'forj make:command reports:export'
- 'forj make:model invoice'
- 'forj make:migration create_invoices'

Use app-prefixed commands for named apps:

- 'forj <app> make:*'
- 'forj marketplace make:controller checkout'
- 'forj marketplace make:job sync-catalog'
- 'forj marketplace make:command billing:reports:sync'

The app prefix routes generated code into the selected app's registration and Wire files.
`

const goPackageDesign = `
# GoForj Go Package Design

Prefer cohesive Go packages over Java/PHP-style nested class folders.

Good package-scoped code:

- 'internal/billing/service.go'
- 'internal/billing/repository.go'
- 'internal/billing/controller.go'
- 'internal/billing/jobs.go'
- 'internal/billing/schedules.go'

Avoid unnecessary nesting such as:

- 'internal/billing/services/reporting/sync/service.go'
- 'internal/billing/controllers/http/v1/admin/controller.go'

Grouped make-command names can express command namespace without forcing deep folders, for example 'forj make:command billing:reports:sync'.

Package-scoped implementation still registers through the selected app's composition files.
`

const migrations = `
# GoForj Migrations

GoForj migrations use raw DDL by default. Keep migrations explicit and reviewable.

Single-app projects keep the simple migration layout until a named app exists.

Multi-app projects use app-scoped migration ownership:

- 'migrations/<app>/<connection>/'

Do not move single-app migrations into multi-app paths unless the project has added another app.
`

const runtimeWorkflows = `
# GoForj Runtime Workflows

Default app commands:

- 'forj build'
- 'forj run'
- 'forj dev'
- 'forj route:list'
- 'forj migrate'

Named app commands:

- 'forj marketplace build'
- 'forj marketplace run'
- 'forj marketplace route:list'
- 'forj marketplace scheduler'

Built binaries map to app entrypoints:

- './bin/app'
- './bin/<app>'
`

const databaseDataAccess = `
# GoForj Database And Data Access

Keep data access behind repository/service boundaries.

Agents should inspect schema through Atlas read-only tools when available. Do not guess table names when the project can report them.

Never expose database secrets in agent output. Read-only query tools must be bounded by timeout and row limits.
`

const observability = `
# GoForj Observability

Use app/runtime identity when reasoning about logs and metrics:

- 'app'
- 'runtime'
- 'instance'

Metrics and dashboards should group by app where appropriate. Lighthouse is the operator UI; Atlas is the agent-facing guidance and read-only tool layer.
`

const vueStarterKit = `
# GoForj Vue Starter Kit

The Vue starter kit is app-local.

Default app frontend source lives under:

- 'cmd/app/frontend/'

Named app frontend source lives under:

- 'cmd/<app>/frontend/'

Local generated auth defaults may use 'admin' / 'admin'. For real users, prefer the generated user creation command.
`

const testingValidation = `
# GoForj Testing And Validation

Use Go cache env vars for tests:

- 'GOCACHE=/tmp/gocache'
- 'GOMODCACHE=/tmp/gomodcache'

When validating GoForj framework renders, render test projects in '/tmp'. Never render test projects inside the GoForj repo.

Prefer focused tests first, then rendered smoke tests when templates or generated behavior changed.
`
