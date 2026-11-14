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
		skill("goforj-catalog-search", "Choose lexical catalog search, typed filters, indexing visibility, and safe highlights.", catalogSearch),
		skill("goforj-feature-rollouts", "Stable tenant rollouts, explicit fallback, and request configuration snapshots.", featureRollouts),
		skill("goforj-business-metrics", "Business counters and duration histograms in the existing App registry.", businessMetrics),
		skill("goforj-app-architecture", "Project, app, runtime, and package layout.", appArchitecture),
		skill("goforj-app-registration", "App composition and registration points.", appRegistration),
		skill("goforj-make-commands", "Prefer GoForj make commands for framework scaffolding.", makeCommands),
		skill("goforj-go-package-design", "Package-scoped Go code and cohesive API shapes instead of class-style nesting.", goPackageDesign),
		skill("goforj-migrations", "Raw DDL and app-scoped migration ownership.", migrations),
		skill("goforj-runtime-workflows", "Build, run, dev, app prefixes, and binaries.", runtimeWorkflows),
		skill("goforj-database-and-data-access", "Repository/service boundaries, schema-derived models, relationships, and safe schema inspection.", databaseDataAccess),
		skill("goforj-observability", "Logs, metrics, app/runtime identity, and Lighthouse boundaries.", observability),
		skill("goforj-vue-starter-kit", "Vue starter kit paths, auth defaults, and embedded frontend assets.", vueStarterKit),
		skill("goforj-react-starter-kit", "React starter kit paths, auth defaults, and embedded frontend assets.", reactStarterKit),
		skill("goforj-templ-htmx-starter-kit", "templ and htmx starter kit paths, server-rendered UI, auth defaults, and embedded frontend assets.", templHTMXStarterKit),
		skill("goforj-testing-and-validation", "Validation commands and render hygiene.", testingValidation),
		skill("goforj-add-http-route", "Workflow for adding HTTP routes, controllers, services, and route verification.", addHTTPRoute),
		skill("goforj-add-app-command", "Workflow for adding app-owned CLI commands.", addAppCommand),
		skill("goforj-notification-delivery", "Choose synchronous notification senders and explicit fallback policies.", notificationDelivery),
		skill("goforj-add-job", "Workflow for adding durable queued jobs.", addJob),
		skill("goforj-add-schedule", "Workflow for adding recurring scheduler work.", addSchedule),
		skill("goforj-add-event-workflow", "Workflow for typed events and subscribers.", addEventWorkflow),
		skill("goforj-add-data-resource", "Workflow for repositories, migrations, cache, storage, and named data resources.", addDataResource),
		skill("goforj-wire-repair", "Workflow for diagnosing and fixing GoForj Wire errors.", wireRepair),
		skill("goforj-debug-runtime", "Workflow for runtime debugging with Atlas evidence.", debugRuntime),
		skill("goforj-multi-app-change", "Workflow for app selection and named app changes.", multiAppChange),
		skill("goforj-validate-change", "Workflow for selecting build, test, and inspection checks.", validateChange),
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

Keep Go packages as flat, self-contained, and portable as practical. Begin with one cohesive package for a responsibility and add files inside it as the implementation grows.

Create a subpackage only when the extracted responsibility has a cohesive API and can stand on its own. A useful boundary should be understandable and testable independently, have a clear owner, remain useful without knowledge of its parent directory, and simplify dependencies rather than merely rearranging files.

Good package-scoped code:

- 'internal/billing/service.go'
- 'internal/billing/repository.go'
- 'internal/billing/controller.go'
- 'internal/billing/jobs.go'
- 'internal/billing/schedules.go'

Avoid unnecessary nesting such as:

- 'internal/billing/services/reporting/sync/service.go'
- 'internal/billing/controllers/http/v1/admin/controller.go'

Avoid Java/PHP-style category nesting such as 'services', 'handlers', 'models', 'types', or 'utils' merely to sort code. File count, type count, and directory symmetry do not justify a package boundary.

Grouped make-command names can express command namespace, for example 'forj make:command billing:reports:sync'. Use a group only when that area is a real package responsibility, not as an organizational label for one implementation.

Package-scoped implementation still registers through the selected app's composition files.

When several returned values describe one application result, prefer a named result struct with meaningful fields over an expanding tuple. Preserve conventional and naturally small returns such as '(value, error)'; do not introduce a struct merely to satisfy a numeric rule.
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

Use 'forj make:model <table> --package <package>' when an existing table needs GoForj's schema-derived model and repository scaffold. Inspect related tables, foreign keys, and nearby repository conventions before modeling relationships. Declare supported relationships in '.db-relationships.yaml' so the generator can validate keys and emit relationship fields; keep preload, join, and repository query policy in App-owned repository methods.

Keep relationship lookup and persistence-specific eager loading in repositories. Controllers, commands, jobs, and frontend code should call application services rather than navigating the database directly.

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

Load feature-local server data in the owning view/component or a colocated composable. Do not promote feature-local server state into a global store merely to share loading machinery.

Keep fast requests visually stable: delay transient skeletons or spinners briefly so a response that resolves immediately does not flash a loading state. During refresh or revalidation, retain usable content until replacement data is ready. Render deliberate loading, error, empty, and ready states without adding an artificial delay after data has arrived.
`

const reactStarterKit = `
# GoForj React Starter Kit

The React starter kit is app-local.

Default app frontend source lives under:

- 'cmd/app/frontend/'

Named app frontend source lives under:

- 'cmd/<app>/frontend/'

Use Atlas 'resource-inventory' and 'get-absolute-url' before guessing ports. Keep generated frontend assets in the owning app's command tree, not under shared backend packages.

Local generated auth defaults may use 'admin' / 'admin'. For real users, prefer the generated user creation command.

Load feature-local server data in the owning route/component or a colocated hook. Do not promote feature-local server state into global context merely to share loading machinery.

Keep fast requests visually stable: delay transient skeletons or spinners briefly so a response that resolves immediately does not flash a loading state. During refresh or revalidation, retain usable content until replacement data is ready. Render deliberate loading, error, empty, and ready states without adding an artificial delay after data has arrived.
`

const templHTMXStarterKit = `
# GoForj templ htmx Starter Kit

The templ/htmx starter kit keeps server-rendered UI in app-owned frontend and internal starter UI packages.

Default app frontend source lives under:

- 'cmd/app/frontend/'

Named app frontend source lives under:

- 'cmd/<app>/frontend/'

Use Atlas 'workflow-plan', 'resource-inventory', and 'get-absolute-url' before changing UI routes. Keep server-rendered page behavior close to the owning UI package and avoid moving app composition into shared runtime code.

Local generated auth defaults may use 'admin' / 'admin'. For real users, prefer the generated user creation command.

Resolve data required for the initial page contract before rendering that page. For htmx refreshes, keep existing content in place while the request is pending and delay transient progress indicators so fast responses do not visibly flash. Use a stable target and intentional error or empty state instead of clearing the region before replacement content arrives.
`

const testingValidation = `
# GoForj Testing And Validation

Use Go cache env vars for tests:

- 'GOCACHE=/tmp/gocache'
- 'GOMODCACHE=/tmp/gomodcache'

When validating GoForj framework renders, render test projects in '/tmp'. Never render test projects inside the GoForj repo.

Prefer focused tests first, then rendered smoke tests when templates or generated behavior changed.
`

const addHTTPRoute = `
# GoForj Add HTTP Route

Use this skill when adding or changing HTTP routes, controllers, request handling, or JSON responses.

First ask Atlas for local facts:

- 'application-info'
- 'project-layout'
- 'workflow-plan' with the task
- 'registration-points' for the selected app

Prefer the make-command path:

- default app: 'forj make:controller <name>'
- named app: 'forj <app> make:controller <name>'

Keep the controller thin. Put business behavior in an application service under 'internal/<domain>'. Wire service constructors through the selected app's service set. Do not edit 'wire_gen.go'.

For images or other cacheable file responses, choose an explicit browser cache policy. Use an 'ETag' or 'Last-Modified' validator when content can be revalidated, honor conditional requests with '304 Not Modified', and keep storage lookup and authorization in the application boundary rather than coupling controllers to a filesystem or cloud SDK.

Verify with:

- 'forj build'
- 'GOCACHE=/tmp/gocache GOMODCACHE=/tmp/gomodcache go test ./...'
- 'forj route:list' or 'forj <app> route:list'
- Atlas 'route-list'
`

const addAppCommand = `
# GoForj Add App Command

Use this skill when adding operator-facing CLI behavior to a generated App.

Start with Atlas:

- 'application-info'
- 'project-layout'
- 'workflow-plan'
- 'registration-points'

Prefer:

- default app: 'forj make:command <name>'
- named app: 'forj <app> make:command <name>'

Commands should parse input, call application services, and report output. They should not create repositories, managers, clients, or services directly. Register through the selected app's 'commands.go' and 'inject_cmd_app.go' surfaces.

Verify with 'forj build', Go tests, and Atlas 'command-list'.
`

const addJob = `
# GoForj Add Job

Use this skill when work needs durable background execution, retries, worker lifecycle, or a stable operational job name.

Use Atlas before editing:

- 'application-info'
- 'workflow-plan'
- 'registration-points'
- 'scenario-guide' for job examples

Prefer:

- default app: 'forj make:job <name>'
- named app: 'forj <app> make:job <name>'

Use typed payloads with small identifiers. Job handlers bind payloads and delegate to services. Do not put business workflows only in 'HandleTask'. Do not dispatch anonymous or untracked queue work.

Verify with 'forj build', Go tests, Atlas 'resource-inventory', and worker startup when useful.
`

const addSchedule = `
# GoForj Add Schedule

Use this skill when adding recurring work.

Start with Atlas:

- 'application-info'
- 'workflow-plan'
- 'registration-points'
- 'schedule-list'

Prefer:

- default app: 'forj make:schedule <name> --every <duration>'
- named app: 'forj <app> make:schedule <name> --every <duration>'

Keep 'app/schedules.go' or 'app/<app>/schedules.go' declarative. A schedule should call a domain-owned method or dispatch an existing job. Schedules are not durable queues and schedule names are not locking mechanisms. Do not hide production schedules behind anonymous callbacks.

Verify with 'forj build', Go tests, and Atlas 'schedule-list'.
`

const addEventWorkflow = `
# GoForj Add Event Workflow

Use this skill when publishing typed facts or adding subscribers.

Start by deciding whether the work is an event or a job. Use events for fan-out and facts. Use jobs when work must be durable, retried, delayed, or worker-managed.

Ask Atlas for:

- 'workflow-plan'
- 'registration-points'
- 'scenario-guide'

Prefer make commands:

- 'forj make:event <name>'
- 'forj make:subscriber <name>'
- use 'forj <app> make:*' when a named app owns the registration.

Publish from services, not controllers. Subscribers should delegate to services or dispatch jobs. Do not register subscribers in package init functions.

Verify with 'forj build', Go tests, and runtime behavior that proves the event or subscriber path runs.
`

const addDataResource = `
# GoForj Add Data Resource

Use this skill for repositories, models, migrations, database access, cache access, storage disks, or named data resources.

Start with Atlas:

- 'workflow-plan'
- 'database-connections'
- 'database-schema' when table shape matters
- 'resource-inventory'
- 'scenario-guide'

Keep durable data in the database. Use cache for temporary or derived data. Use storage for files and blobs. Do not import backend driver packages into services or repositories when generated managers or named accessors should own that boundary.

When a feature introduces a durable file category such as avatars, invoices, exports, or attachments, inspect existing resources and prefer a purpose-named storage resource when the category has distinct access, retention, or deployment policy. Inject its generated manager/accessor into the owning service; do not write directly to an incidental local directory.

Prefer make commands for generated artifacts:

- 'forj make:model <table> --package <package>'
- 'forj make:migration <name>'

The model generator derives fields from an existing table and reads supported relationship declarations from '.db-relationships.yaml'. Inspect the actual schema first, use the relationship config for generated fields, then keep relationship loading and repository query policy in App-owned repository code.

Verify with 'forj build', Go tests, safe Atlas schema inspection, and app-specific migration or route commands when relevant.
`

const wireRepair = `
# GoForj Wire Repair

Use this skill when 'forj build' or Wire reports missing providers, duplicate providers, type mismatches, or stale generated output.

First classify the failure with Atlas 'wire-diagnostics'.

Then inspect:

- 'registration-points'
- 'project-layout'
- relevant 'read-doc-section' output from 'core/reading-wire-errors.md'

Fix the owning provider set, not generated output. Do not edit 'wire_gen.go'. Do not add nil guards around required constructor-injected dependencies. Required dependencies should fail fast when wiring is wrong.

Verify with 'forj build' and Go tests.
`

const debugRuntime = `
# GoForj Debug Runtime

Use this skill when investigating local runtime failures, missing routes, bad URLs, browser errors, logs, metrics, or Lighthouse/inspect behavior.

Gather evidence first:

- 'application-info'
- 'route-list', 'schedule-list', or 'command-list'
- 'read-log-entries'
- 'last-error'
- 'get-absolute-url'
- 'browser-logs'
- 'metrics-metadata'
- 'resource-inventory'

Reason with app/runtime identity. Do not guess ports, route registration, metrics labels, or scheduler state when Atlas can report them. Lighthouse is useful, but it is not the only observability surface.

Use normal runtime commands such as 'forj api', 'forj worker', 'forj scheduler', or app-prefixed equivalents only after Atlas evidence points to the runtime to inspect.

Verify fixes with 'forj build', Go tests, and the relevant Atlas runtime tool output.
`

const multiAppChange = `
# GoForj Multi App Change

Use this skill when a project has named apps or the task mentions an app such as marketplace, admin, backstage, or billing.

Call 'application-info' first and choose the owning app before generating or editing files.

Default app surfaces:

- 'cmd/app/main.go'
- 'app/'
- 'app/wire/'

Named app surfaces:

- 'cmd/<app>/main.go'
- 'app/<app>/'
- 'app/<app>/wire/'

Use 'forj <app> make:*' for generated artifacts owned by a named app. Do not write registration into the default app when a named app owns the route, command, job, schedule, subscriber, or repository.

Verify with 'forj <app> build' and app-scoped Atlas tools such as 'route-list', 'command-list', or 'schedule-list'.
`

const validateChange = `
# GoForj Validate Change

Use this skill before marking GoForj work complete.

Ask Atlas for 'validation-plan' with the task and selected app. Verify by running the smallest checks that prove the change:

- 'GOCACHE=/tmp/gocache GOMODCACHE=/tmp/gomodcache go test ./...'
- 'forj build' or 'forj <app> build'
- 'forj route:list' or Atlas 'route-list' for HTTP changes
- Atlas 'command-list' for command changes
- Atlas 'schedule-list' for scheduler changes
- Atlas database/schema/log/browser/metrics tools for runtime diagnostics

For GoForj framework render validation, render test projects in '/tmp', never inside the GoForj repo.

Do not treat a narrow green check as proof for a broad workflow. Match verification to the behavior changed.
`

const catalogSearch = `
# GoForj Catalog Search

Use this skill when a catalog needs lexical text matching, category filters, facets, pagination, or highlights. Consider 'github.com/goforj/search' for a portable typed index boundary. It provides literal all-terms lexical search, not vector or semantic search. Retain adequate database queries or an explicit existing search dependency when the task does not justify another index.

Inspect the owning App, existing imports, go.mod versions, supported generators, and library documentation before selecting a backend. Search roots and drivers are independently versioned modules: verify both pins and their minimum Go versions. Start with Atlas 'search-docs' for search, then 'read-doc-section' for the selected API. The released v0.1.1 root and 'github.com/goforj/search/driver/blevesearch' v0.1.1 support a real embedded backend; do not assume another driver's version matches.

Keep the database as durable truth. The index is a rebuildable projection, with an explicit backfill, update, delete, retry, and recovery lifecycle. Commit durable writes first, then update the projection according to an explicit consistency policy. An accepted Upsert receipt does not prove visibility: call Index.Wait with the caller context and inspect Outcome.State, including terminal failure and unknown outcomes, before promising a searchable update. Do not blindly retry an uncertain accepted write.

Declare searchable TextField values separately from filterable KeywordField values. Use typed Equal filters instead of concatenating category input into a backend query string. Apply authorization and visibility constraints on every query, including facets; a category filter is not authorization. Keep total precision and facet truncation metadata intact. Facet counts precede hit pagination. Use bounded offsets and limits plus explicit stable sort fields when pagination must be deterministic. Relevance scores are backend-specific and must not become portable application guarantees.

Highlight fragments contain untrusted Segment.Text even when Matched is true. Escape every segment using the UI's text rendering; add trusted markup only around escaped matched text. Never render backend fragments as raw HTML.

If the Project does not declare Search, inspect local CLI generate --help to confirm resource support before proposing component or generated-file changes. Do not guess a minimum version; preserve application-owned standalone package composition when the installed CLI lacks this resource. When adding lexical search to a GoForj Project with confirmed resource support, first enable the optional Search component for the owning App in .goforj.yml and use the resource generation path. For a GoForj Project with Search enabled, prefer the generated 'searches.Manager' and purpose-named accessor such as 'Catalog()' over opening a driver in application services. Declare the owning App's schema in 'SearchDefinitions()' under 'app/search.go' or 'app/<name>/search.go'; environment settings choose backend/path/endpoint/auth. Run 'forj generate --search' after named resource declarations. Empty App definitions are inert. Provision explicitly with 'searches.Provision(ctx, app.SearchDefinitions())' from a standalone bootstrap before App initialization; startup validates schemas and rejects drift. Keep rebuilds application-owned and stop the single Bleve owner before provisioning its path.

The generated inject_managers.go constructs *searches.Manager from the owning App's SearchDefinitions() and owns backend cleanup. Register your application constructor in app/wire/inject_services_app.go (or app/<name>/wire/inject_services_app.go for a named App). Wire supplies the existing manager. For a declared Catalog index, select it once:

    // NewService borrows the App-managed catalog index.
    func NewService(searches *searches.Manager) *Service {
        return &Service{index: searches.Catalog()}
    }

Do not edit generated inject_managers.go, open another manager in the service, or pass the App into the service to reach Searches(). App.Searches() is for callers that already own the App, not the service injection path. Keep explicit provisioning in the standalone bootstrap and closing in framework cleanup. Bleve persistent indexes have a single process owner. Inspect supported make commands; do not invent 'make:search' or edit wire_gen.go.

Verify exact inclusions and exclusions, totals across multiple pages, category isolation, cancellation, and visible replacements after successful Wait using a real local backend. Test unsafe highlight input if rendering is added. A recording fake alone cannot prove backend matching or visibility.
`
