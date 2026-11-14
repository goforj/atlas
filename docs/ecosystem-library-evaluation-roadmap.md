# GoForj library guidance and evaluation roadmap

Prioritize validation and framework resource integration for search, notification, and feature support. Give each framework resource its own PR, then teach and evaluate its generated manager/accessor in a separate Atlas PR. Follow with deeper httpx, cache, configuration, and metrics cases. Prepare secret guidance now, but keep its released-module evaluation pending publication.

Assessment date: November 13, 2026, America/Chicago. Initial research used Atlas `17d7436` on `fix/codex-mcp-readiness` and GoForj `v0.37.0`. Implementation PRs start from the current remote default branches and recheck the relevant contracts. The queue below describes proposed coverage, not live evaluation results.

## Framework resources before their evaluations

The implementation direction is framework resources first for feature, search, and notification. These are configured infrastructure dependencies, like cache, storage, queues, events, and mail. Their framework PRs should establish optional components, supported-driver selection, default and named accessors, explicit injection, App ownership, and lifecycle cleanup before Atlas promotes application guidance for them.

Keep the responsibilities distinct: feature resources own evaluators and refresh; search resources own configured indexes while schemas and indexing workflows stay application-owned; notification resources own configured senders while delivery policy and durable jobs remain explicit. Component-off Projects must not acquire their packages, environment settings, or dependencies.

Retain the standalone library behavior probes as useful calibration, but compose promoted framework tasks through actual generated resources. Each resource PR, Atlas evaluation PR, GoForj calibration companion, and public-docs PR remains a separate review boundary. Validation and ordinary HTTP clients do not require new resource managers merely to participate in this work.

## Current coverage

Atlas has 32 evaluation manifests, 57 capability entries (40 mapped to evaluations and 17 planned), and 22 built-in skills. Those counts describe framework coverage, not library-selection coverage.

The main gaps are concrete:

- `add-validated-write-endpoint` measures request behavior, but its golden scenario implements handwritten checks. Its ownership budget excludes `go.mod` and `go.sum`, so introducing validation as a new dependency requires a separate or versioned contract.
- `add-outbound-http-integration` has an httpx golden implementation, pinned to `github.com/goforj/httpx/v2@v2.0.1`. Atlas checks the typed client and HTTP behavior without requiring actual httpx use. This is useful behavior coverage, but does not establish selection of httpx.
- `choose-storage-for-files` explicitly requests a named storage resource and manager signature. It measures execution after the choice has largely been disclosed.
- `runtime-observability` repairs Lighthouse capture and checks existing metrics surfaces. It does not measure adding application metrics or controlling label cardinality.
- Validation, search, notification, secret, and feature have no dedicated entries in the capability catalog or dedicated built-in skills. `goforj-testing-and-validation` concerns development checks, not the validation library.
- Skill recommendations use framework components and starter kits. There is no explicit decision guide for these newer standalone libraries. A library need can arise before its dependency or a matching framework component exists.
- `workflows.Classify` uses broad keyword precedence and defaults to the HTTP-route workflow. Natural requests about search, secrets, notifications, and rollouts have no dedicated routing. Adding every library name to the switch would still miss problem descriptions that do not name a library.

Sources: [capability catalog](../eval/coverage.yaml), [skill catalog](../skills/catalog.go), [recommendations](../skills/recommend.go), [workflow classifier](../workflows/workflows.go), [surface contracts](../eval/surface_contracts.go), [storage prompt](../eval/evaluations/choose_storage_for_files/prompt.md), and GoForj's [validation scenario](https://github.com/goforj/goforj/blob/v0.37.0/internal/scenarios/specs/create-invoice-validation.yaml) and [HTTP scenario](https://github.com/goforj/goforj/blob/v0.37.0/internal/scenarios/specs/tax-rate-http-integration.yaml).

## Library inventory

The 23 libraries below cover the current application and developer library scope. Atlas is the guidance layer being enhanced; GoForj is the framework host. Harbor and Ship are separate products. Design-only proposals such as localization and audit are not released library targets.

"Existing" means related evaluations exist. It does not imply complete library-specific selection or API coverage. Priorities weigh likely application use, consequences of choosing incorrectly, the size of the current gap, and whether deterministic behavior can be verified economically.

| Library | Current Atlas coverage | Highest value next enhancement | Priority |
| --- | --- | --- | --- |
| validation | Write-endpoint behavior; handwritten golden checks | Reusable input rules, stable field errors, PATCH presence semantics | First |
| search | No dedicated contract | Recognize lexical search, typed filters, visibility, and safe highlights | First |
| notification | No dedicated contract | Select a sender boundary, explicit fallback, and honest acceptance semantics | First |
| feature | No dedicated contract | Stable tenant rollout, explicit fallback, and request snapshot consistency | First |
| secret | No dedicated contract | Bound secret reads, safe failures, and cancellation; publication pending | Prepare now |
| httpx | Typed outbound client behavior | Actual v2 adoption plus status handling, bounded retries, and cancellation | Next |
| cache | Named resources and cache-aside | Rate limits or duplicate-work locks with explicit scope and failure policy | Next |
| web | Controllers, middleware, auth, and cache revalidation | Route-local rate-limit policy and trusted request identity | Next |
| env | Incidental configuration in other scenarios | Typed startup configuration, required values, and precedence | Next |
| crypt | Incidental generated Auth use | Authenticated payload encryption, tamper rejection, and key rotation | Next |
| metrics | Existing metrics surface and inspect repair | Business instrumentation through the existing registry, bounded labels | Next |
| events | Subscriber, publication, and job handoff | Context signature changes across callers and tests; lifecycle failures | Next |
| queue | Typed jobs, named queues, retry-safe reports | Choose durable work versus synchronous reactions; failed-work recovery | Later |
| scheduler | Generated schedules and existing-job dispatch | Timezone, overlap policy, and distinguishing dispatch from execution | Later |
| storage | Several named-storage and upload cases | Streaming large objects and verifying behavior across driver changes | Later |
| mail | Generated mail-manager workflow | Composition, failure propagation, and cancellation without provider coupling | Later |
| null | Incidental generated model use | SQL/JSON null versus zero; distinguish from request field presence | Later |
| execx | No dedicated contract | Explicit argv, cancellation, exit status, and stdout/stderr separation | Later |
| console | Command behavior, no output-library contract | Redirect-safe output, machine stdout, status stderr, and no forced prompts | Later |
| collection | No dedicated contract | Nontrivial aggregation with explicit mutation and cloning decisions | Later |
| str | No dedicated contract | Unicode-aware transformations and byte versus rune semantics | Later |
| godump | No dedicated contract | Bounded redacted diagnostics without terminating application execution | Later |
| wire | Provider repair and registration across many evals | Interface binding, duplicate providers, and smallest graph repair | Later |

API sources: the [GoForj organization overview](https://github.com/goforj), individual library READMEs and module manifests, and the new-library sources linked in the PR queue. The overview predates some newer additions, so it is not a complete inventory by itself.

## PR sequence

### Shared selection guidance

Suggested title: `feat: guide agents through GoForj library selection`.

Add a compact ecosystem decision guide with problem cues, the appropriate library, boundaries with adjacent libraries, and links to focused docs. Make the entry point available even when the needed dependency has not been installed. Keep detailed guidance narrow and discoverable rather than loading 23 full API guides into every Project.

Teach agents to inspect existing imports, the Project's module versions, owning App, and supported generators first. Preserve established dependencies and working standard-library code unless the task warrants changing them. Library availability alone is not a reason to rewrite code.

Add deterministic regressions for problem descriptions and misleading keywords, including "secret provider", "validation errors", and "search endpoint". Keep transport workflow selection and library advice distinct: a search endpoint can still need the HTTP-route workflow. Extend the existing planning surface only where useful; a new MCP tool or public schema is not a prerequisite.

This PR supplies shared guidance. Each following PR adds its library's decision rules, docs retrieval regressions, and executable evaluation contract.

### 1 Validation

Suggested title: `feat: teach and evaluate GoForj validation workflows`.

Start here because request validation is common, the current golden case explicitly uses handwritten checks, and the library offers semantics that are easy to lose in ad hoc code. [validation v0.2.0](https://github.com/goforj/validation/tree/v0.2.0) supports reusable typed rules, structured issues, contextual rules, and presence-aware inputs.

Add a natural task to update customer settings while distinguishing an omitted field, explicit null, and supplied zero or false. Require reusable validation and the existing public error contract without naming the target library in the discovery prompt. Introduce the pinned dependency during supervisor preparation, outside candidate network access, with an honest boundary about whether dependency installation is being measured.

The verifier should exercise valid and invalid inputs, normalization, unchanged omitted values, explicit null, zero/false, stable field errors, and an operational rule failure that must not become a client validation error. Reject compiling mutants that collapse field presence or skip a rule. Verify actual library calls on the application path separately from observable request behavior.

Preserve the existing write-endpoint result contract. Prefer a new evaluation, or explicitly version an incompatible verifier change, rather than silently regrading historical results.

### 2 Search

Suggested title: `feat: teach and evaluate portable catalog search`.

[search v0.1.1](https://github.com/goforj/search/tree/v0.1.1) adds a substantial application capability with no dedicated Atlas contract. Teach its use for lexical search, filters, facets, highlights, and an explicit indexing lifecycle. Keep the database as the durable source of truth; do not imply that this library provides vector or semantic search.

Use a catalog task with descriptive text, a category filter, pagination, and highlights. Calibrate first with an actual local backend, such as the separately versioned Bleve driver, rather than a fake that only records method calls. Verify exact inclusion/exclusion, result totals and pagination, accepted-write visibility, and safe rendering of untrusted highlight segments. Include mutants that drop the filter or return stale results after an index update. Do not test backend-specific relevance scores as portable guarantees.

Keep native service provisioning and a broad driver matrix outside the first PR. GoForj has no existing search generator in the inspected templates, so use application-owned composition and Wire registration without inventing `make:search`.

### 3 Notification

Suggested title: `feat: teach and evaluate notification delivery boundaries`.

[notification v0.1.0](https://github.com/goforj/notification/tree/v0.1.0) provides a sender abstraction and composition. Teach the choice among notification for multi-channel messages, mail for email composition/delivery, events for reactions, and queue for durable deferred work. A synchronous send does not provide background execution, retries, or proof that a recipient received a message.

Use an application incident task with a primary channel and explicit fallback policy. A supervisor-owned recording sender should verify message content, context propagation, primary success, classified failure, and cancellation. Include a compiling mutant that reports success while discarding the notification. Add an uncertain-acceptance case so fallback does not automatically duplicate a message that may already have been accepted. Do not send real notifications during calibration.

### 4 Feature

Suggested title: `feat: teach and evaluate targeted feature rollouts`.

[feature v0.1.0](https://github.com/goforj/feature/tree/v0.1.0) supports typed flags, explicit fallbacks, stable targeting, and snapshots. Use a checkout rollout task that must remain stable for the same tenant and preserve one revision across decisions within a request.

Verify stable assignment across repeated evaluations and refresh, tenant separation, missing-flag fallback, invalid-refresh retention, and consistency during a concurrent update. Use controlled sources and clocks. Reject mutants that randomize each request or mix revisions. Teach that an ordinary targeting scope is not itself a provider configuration snapshot, and that a feature flag does not replace authorization.

The inspected hosted docs tree has no feature library page. Add its authoritative documentation source and generated mirror through the existing docs workflow before depending on Atlas docs retrieval for this case. There is no existing feature generator to recommend.

### 5 Httpx

Suggested title: `feat: evaluate resilient httpx integrations`.

Extend the existing tax-rate fixture with a focused client-resilience case. Teach `github.com/goforj/httpx/v2`, typed responses, service-owned clients, caller context, and intentional retry policy. Verify status errors, malformed responses, bounded transient retries, cancellation, and preserving non-idempotent request behavior. Use a local HTTP server and count actual requests.

Track library adoption separately from the existing behavior score. Do not claim that the current passing outbound-client evaluation proves httpx selection. Pin to a version compatible with the prepared Project; the existing fixture's v2.0.1 and the latest v2.1.1 have different release context.

### 6 Cache with web route policy

Suggested title: `feat: evaluate tenant request rate limits`.

`http.rate-limiting` is already planned in the coverage catalog. Use one App task with per-tenant limits and an unrelated route that must remain unaffected. Teach the distinction between local middleware limits and a shared cache-backed policy needed across replicas.

Choose the required deployment scope in the task. Verify tenant isolation, the exhausted response, reset with a controlled clock, unaffected routes, and explicit failure policy. For a shared policy, verify two application instances against the same backend. Use semantic mutants for one global bucket and a process-local substitute. Keep a separate later PR for cache locks, because ownership and release-on-failure require different probes.

### 7 Configuration and encryption

Split this into two library PRs: `feat: evaluate typed environment configuration`, then `feat: evaluate encrypted payload rotation`.

For env, test required startup values, typed parsing, fallback and precedence rules, and safe diagnostics. For crypt, test authenticated round trips, tamper rejection, old-key decryption after rotation, and new-key encryption. Explain that environment configuration, payload encryption, and managed secret lookup solve different needs. Do not invent a secret manager from crypt and env.

### 8 Metrics

Suggested title: `feat: evaluate application business metrics`.

Use a task to count successful checkouts and observe duration through the Project's existing metrics registry. Verify actual counter/histogram changes and exposition, startup registration, bounded label values, and absence of tenant IDs or request IDs as labels. Reject a mutant that registers the metric but never updates it. Preserve the current Lighthouse evaluation as its own operational task.

### Secret after publication

Suggested title: `feat: teach and evaluate bound secret reads`.

[secret](https://github.com/goforj/secret) has implementation and documentation, but the GitHub API returned no tags or releases at assessment time. Prepare its selection guidance alongside env and crypt. Promote a release-backed evaluation after the required root, driver, and test modules are independently available; do not invent a v0.1.0 pin from a candidate badge.

Use an injected `secret.Reader` for a named credential, with provider binding in startup composition. Probe exact value preservation, classified missing/permission failures, cancellation, and safe logs using fake credentials. Keep replacement, deletion, and uncertain writes for a later enhancement. No cloud account access is needed for the first contract.

## Evaluation design for every enhancement

1. Write the discovery prompt as an application problem, without naming the expected import, generator command, or verifier recipe. An additional explicitly named-library task may measure API execution separately.
2. Score library choice, application behavior, composition, and regression-test quality separately. An unused import cannot prove selection, and a valid standard-library solution need not be labeled a behavioral failure. Treat context-sensitive library preference as its own signal.
3. Add a small counterexample to the decision guidance: retain adequate standard-library code, preserve an explicit existing dependency, distinguish notification from queued delivery, or decline to invent a missing library primitive. Promote live negative cases only where a sound verifier exists.
4. Update the skill source, recommendation logic, workflow/docs references, and deterministic regressions together. Exercise the native output projections through existing writer tests.
5. Add a reusable prepared GoForj scenario, a reviewed golden target, a supervisor-owned behavior probe, and at least one compiling semantic mutant that fails for the intended reason. Measure the library on the executed path, not by a source token alone.
6. Add capability mappings such as `libraries.validation-presence` or `libraries.feature-rollout` in `eval/coverage.yaml`. Leave them planned until the verifier and calibration exist. Keep current schema compatibility unless a concrete reporting need requires an additive change.
7. Prepare module dependencies before candidate execution, preserve sibling replacements for repository development, and verify the released composition with `GOWORK=off`. Render only under `/tmp`.
8. Start deterministic calibration without live provider sessions. When live measurement is explicitly authorized, compare adjacent guidance treatments on a small portfolio and retain failures. Keep diagnostic outcomes distinct from authoritative workflow claims.

## Repository boundaries and release constraints

Atlas owns guidance, skill recommendation, planning, evaluation prompts, verifier contracts, behavior probes, and capability mapping. GoForj owns rendered preparation and golden scenarios, calibration integration, and CLI composition. Most new live evaluations therefore need a small coordinated GoForj companion PR even when the visible enhancement is in Atlas. Docs owns hosted library pages and their generated mirrors. Follow the actual source/generation pipeline when fixing documentation.

The new released roots checked here are validation v0.2.0, search v0.1.1, notification v0.1.0, and feature v0.1.0. Their root manifests specify Go 1.24.0; that does not establish the minimum for every selected driver. In particular, the search public-consumer check uses Go 1.25.0 and a separately tagged Bleve module. Search, notification, feature, and secret all have nested modules, so root publication alone is insufficient.

Avoid a blanket dependency refresh: the inspected httpx v2.1.1, cache v0.4.3, and queue v0.3.0 root manifests require Go 1.27.0, while the inspected GoForj root specifies Go 1.26.0. The initial local Atlas research branch specified Go 1.25.0; current remote Atlas main already specifies Go 1.26.0. Select compatible versions deliberately and report any real minimum-Go constraint. Do not increase Atlas's Go version merely to write library guidance or execute a disposable Project's verifier.

Release evidence: [validation](https://github.com/goforj/validation/releases/tag/v0.2.0), [search](https://github.com/goforj/search/releases/tag/v0.1.1), [notification](https://github.com/goforj/notification/releases/tag/v0.1.0), [feature](https://github.com/goforj/feature/releases/tag/v0.1.0), [httpx manifest](https://github.com/goforj/httpx/blob/v2.1.1/go.mod), [cache manifest](https://github.com/goforj/cache/blob/v0.4.3/go.mod), and [queue manifest](https://github.com/goforj/queue/blob/v0.3.0/go.mod). Recheck tags and module resolution when implementing each PR.

## Existing benchmark follow through

GoForj's checked-in benchmark reports 31/32 passing with skills and 31/32 with full Atlas guidance. Its remaining guided failure is `publish-domain-event`: service context changes were not propagated through an existing test. Address that in a separate focused repair PR and retain the old failure in the benchmark history. It is a concrete guidance defect, not evidence that another library is missing.

The Atlas live-evaluation plan still contains an older statement that only `none` versus `agents` was measured. Reconcile that statement with the later [GoForj benchmark](https://github.com/goforj/goforj/blob/v0.37.0/docs/maintainer/atlas-evaluation-benchmark.md) before using it to plan paid runs. The current benchmark remains a diagnostic snapshot rather than an estimated future success rate or causal treatment effect.

## Assessment checks

Library existence, remote default-branch revisions, release records, tags, and selected tagged module manifests were checked through GitHub. Local source inspection established the evaluation and guidance gaps. The remote docs tree includes validation, search, notification, and secret pages, but no feature page. Older local library checkouts were not treated as the authority for current major-version import paths.

A temporary Go diagnostic could not compile because the shared `/tmp/gomodcache` contained missing dependency source files. It did not produce runtime classification or test evidence. No live provider evaluations, releases, or account mutations were performed. Relative links, punctuation, and repository counts in this assessment were checked before review. The shared selection guide now ships separately from the proposed library-specific evaluation contracts.
