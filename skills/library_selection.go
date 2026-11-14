package skills

// librarySelection keeps dependency decisions discoverable before a Project enables a matching component.
const librarySelection = `
# GoForj Library Selection

Use this skill when an application need could use an ecosystem library: reusable input rules, searchable catalogs, channel notifications, targeted rollouts, managed secrets, HTTP clients, durable work, or named infrastructure resources.

## Start With Project Evidence

Inspect 'go.mod', existing imports, '.goforj.yml', the owning App, and nearby services before selecting a dependency. With Atlas, use 'application-info', 'resource-inventory', 'search-docs', and 'read-doc-section'; inspect 'version-alignment' when docs and code disagree. Without Atlas MCP, read local source and the documentation matching the installed module version.

Preserve an explicit existing dependency and adequate standard-library code unless the requested behavior warrants a change. Do not rewrite working code just because an ecosystem alternative exists. Resolve minimum Go versions, module major suffixes, optional driver modules, and published tags before changing 'go.mod'. Never guess an API from the library's name or use '@latest' as a compatibility policy.

## Choose By Application Need

- Reusable decoded-input rules, stable field errors, and omitted/null/zero distinctions: validation ('github.com/goforj/validation'). This differs from running tests and from nullable database values.
- Lexical catalog search, typed filters, facets, and highlights: search ('github.com/goforj/search'). Keep indexing and write visibility explicit, preserve the database as the durable source of truth, and inspect separately versioned drivers. Do not infer semantic or vector search support.
- Channel messages and deliberate fanout or fallback: notification ('github.com/goforj/notification'). A send is synchronous; acceptance is not proof of recipient delivery, automatic retries, or durable background work.
- Targeted release controls, typed fallbacks, and stable tenant rollouts: feature ('github.com/goforj/feature'). Use the documented snapshot boundary when one request needs a consistent revision. A flag does not replace authorization.
- Credentials bound to managed stores or encrypted files: secret ('github.com/goforj/secret'). Inject the narrow reader into application services and keep provider locations in startup composition. Verify publication before adding a version pin; a candidate badge is not a released module.
- Typed environment parsing and startup requirements: env ('github.com/goforj/env/v2'). Authenticated application payload encryption and old-key decryption during rotation: crypt ('github.com/goforj/crypt'). Neither replaces managed secret lookup.
- Outbound typed HTTP integrations: httpx ('github.com/goforj/httpx/v2'). Inspect the pinned version for caller-context APIs, bounded retry policy, and status-error behavior. Do not turn non-idempotent calls into automatic retries without a policy.
- Inbound HTTP handlers, routing, and middleware: web ('github.com/goforj/web'). Keep handlers behind existing contracts and verify identity and route scope for policies such as rate limits.
- Repeated reads, counters, expiring rate limits, and locks: cache ('github.com/goforj/cache'). Decide whether the behavior must coordinate across processes before choosing a local backend. Cache is not durable business storage.
- Files, uploads, and streaming objects: storage ('github.com/goforj/storage'). Reuse or generate a purpose-named resource when access, retention, or deployment policy differs; inspect driver capabilities before assuming streaming or URL support.
- Email composition and delivery: mail ('github.com/goforj/mail'). Use the generated manager in GoForj Projects. Choose notification when the application needs channel policy rather than email-specific composition.
- Business counters, gauges, duration histograms, and Prometheus exposition: metrics ('github.com/goforj/metrics'). Reuse the Project registry, register during startup, and keep label cardinality bounded.

## Separate Reactions From Durable Work

Use events ('github.com/goforj/events') for typed reactions and fanout. Delivery and failure guarantees depend on the selected driver. Use queue ('github.com/goforj/queue') for deferred work that needs an explicit backend, retries, job identity, and worker lifecycle. Use scheduler ('github.com/goforj/scheduler/v2') to decide when work is dispatched, with explicit timezone and overlap policy. A scheduler is not itself a durability guarantee.

For framework participation, inspect the Project version, 'forj' help, and generated App-scoped managers first. Prefer supported resource managers and their named accessors for configured infrastructure so driver selection and lifecycle stay framework-owned. Where that Project version has no resource integration, compose the standalone dependency through an explicit provider and keep setup outside domain behavior. Do not invent 'make:search', 'make:feature', or other commands from a library name.

## Smaller Supporting Libraries

Use console for redirect-safe CLI presentation and execx for explicit subprocess argv, cancellation, and exit errors. Use collection/v4 for substantial typed slice pipelines after checking mutation/clone semantics; use str/v3 for string transformations after checking byte versus rune contracts. Use null/v6 for SQL/JSON nullable values, not as a substitute for omitted request-field detection. Use godump for bounded redacted development diagnostics, not request-path termination. Use wire for compile-time composition and regenerate its output rather than editing 'wire_gen.go'. Verify these module suffixes against the Project before upgrading an older major.

## Verify The Choice

Read focused library docs with 'search-docs' and 'read-doc-section', or the versioned upstream README and examples. Keep a search HTTP endpoint on its HTTP workflow while selecting search for application behavior; a 'secret provider' is not automatically a missing Wire provider, and 'validation errors' are not automatically a runtime incident.

Test the requested behavior and failures at the application boundary, including cancellation and ownership. Confirm that the chosen library actually executes on that path. An unused import or a passing build does not prove the integration. Keep library preference separate from whether a valid application implementation satisfies its behavioral contract.
`
