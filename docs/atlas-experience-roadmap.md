# Atlas Experience Roadmap

This roadmap tracks the next set of Atlas improvements that would materially
raise the GoForj user and agent experience beyond static skills and basic MCP
tools.

The goal is not more prompt text. The goal is a tighter feedback loop where an
agent can understand a GoForj project, choose the right workflow, inspect real
runtime evidence, avoid generated-file mistakes, validate the right behavior,
and keep its local Atlas install aligned with the project.

## Product Direction

- Atlas remains read-only for project inspection and diagnostics.
- GoForj commands remain the write path for framework-owned scaffolding.
- Skills become thin entry points into verified Atlas workflows, not the main
  source of truth.
- Quality is measured with realistic workflow evals, not just unit tests.
- Runtime debugging should prefer evidence from local project state, logs,
  browser output, metrics metadata, routes, resource links, and docs.

## Phase 1: Agent Workflow Evals

Purpose: prove Atlas improves real agent behavior on common GoForj tasks.

- [x] Define a small eval package or command for deterministic agent workflow fixtures.
- [x] Add fixtures for adding an HTTP route with a service and repository.
- [x] Add fixtures for fixing a missing Wire provider.
- [x] Add fixtures for adding a queued job and schedule pair.
- [x] Add fixtures for adding a named-app route without touching the default app.
- [x] Add fixtures for debugging a runtime route or browser failure using Atlas evidence.
- [x] Score whether the agent selected the right app, command path, files, docs, tools, and validation checks.
- [x] Record compact redacted transcripts behind an opt-in flag.
- [x] Add a scorecard summary that can run without network access or live model calls.
- [x] Document how to add a failed real-world transcript as a regression fixture.

Acceptance gate:

- [x] `go test ./...` covers deterministic eval scoring.
- [x] At least five realistic workflow fixtures pass.
- [x] A failed fixture reports the missing behavior in a way a maintainer can act on.

## Phase 2: MCP Protocol Reliability

Purpose: catch breakage in the actual stdio MCP path used by Codex, Claude,
Gemini, and other local agents.

- [x] Add stdio MCP protocol tests that start an Atlas server process.
- [x] Test tool discovery returns all stable Atlas tools.
- [x] Test `workflow-plan`, `docs-section-pack`, `resource-inventory`, and `validation-plan` over real JSON-RPC.
- [x] Test malformed requests return bounded errors without crashing the server.
- [x] Test stdout remains MCP-only and diagnostics go to stderr or logs.
- [x] Add a smoke test for `forj atlas:mcp` from a rendered project in `/tmp`.

Acceptance gate:

- [x] Protocol tests run in CI without requiring a live editor or agent.
- [x] A missing or renamed MCP tool fails a protocol-level test.
- [x] `forj atlas:mcp` remains compatible with the Atlas server contract.

## Phase 3: Runtime Evidence Loop

Purpose: make Atlas excellent at answering "what is actually happening locally?"

- [x] Add a composed `runtime-snapshot` MCP tool that combines app info, routes, resources, logs, last error, URLs, browser logs, and metrics metadata.
- [x] Add a `debug-plan` MCP tool that turns runtime evidence into next inspection steps without executing shell commands.
- [x] Add request fields for app, runtime, path, route name, and time window.
- [x] Include confidence and missing-evidence fields so agents know when not to guess.
- [x] Connect resource registry links to runtime tools where URLs are available.
- [x] Add tests for no-Lighthouse, no-browser, no-logs, and partial-evidence cases.
- [x] Add docs examples for route failure, browser console failure, and metrics-target failure workflows.

Acceptance gate:

- [x] Runtime tools return useful output with partial local evidence.
- [x] Runtime tools do not require Lighthouse UI to be open.
- [x] Tests prove tools avoid fabricating ports, routes, metrics targets, or app names.

## Phase 4: Ownership And Generated-File Model

Purpose: prevent agents from editing the wrong files and make ownership rules
machine-checkable.

- [x] Replace path-only `generated-file-policy` logic with a formal ownership model.
- [x] Model generated, app-owned, named-app-owned, framework-owned, config-owned, migration-owned, frontend-owned, and user-owned paths.
- [x] Include preferred edit mechanism: direct edit, `forj make:*`, regenerate, or do not edit.
- [x] Add Wire-specific ownership guidance for provider sets versus `wire_gen.go`.
- [x] Add starter-kit-specific ownership for Vue, React, and templ/htmx frontend trees.
- [x] Add project overlay support for custom ownership rules in `.ai/skills` or `.goforj/atlas.json`.
- [x] Add tests for default app, named app, generated files, migrations, frontend files, docs files, and unknown paths.

Acceptance gate:

- [x] `generated-file-policy` can explain both classification and preferred action.
- [x] Workflow plans consume ownership output for warnings and verification.
- [x] Regression tests cover common bad-edit paths.

## Phase 5: Install And Update UX

Purpose: make Atlas feel reliable and self-maintaining inside GoForj projects.

- [x] Add an `atlas doctor` or equivalent status surface behind GoForj CLI.
- [x] Report installed agents, configured MCP servers, skill versions, docs ref, and GoForj version.
- [x] Detect stale generated agent files and stale installed skills.
- [x] Make `forj atlas:update` idempotent and explicit about what changed.
- [x] Add dry-run output for install/update.
- [x] Add project-tailored skill enablement based on components, apps, starter kit, and resource registry.
- [x] Add tests that preserve user-authored content while updating generated Atlas blocks.
- [x] Add public docs for install, update, doctor, and troubleshooting output.

Acceptance gate:

- [x] A user can tell whether Atlas is installed, current, and aligned with the project.
- [x] Update output is deterministic and test-covered.
- [x] Existing user-authored agent instructions are preserved.

## Phase 6: Docs And Version Alignment

Purpose: reduce confusion when project code, rendered templates, docs, and Atlas
skills are from different versions.

- [x] Add docs manifest fields for docs commit, docs ref, GoForj version, and generated timestamp when available.
- [x] Add a `version-alignment` MCP tool.
- [x] Compare project GoForj version, Atlas version, docs ref, and rendered project metadata.
- [x] Warn when docs are from `main` but project targets a released version.
- [x] Warn when the rendered project version does not match the local GoForj CLI.
- [x] Include alignment metadata in `docs-section-pack`.
- [x] Add tests for exact match, docs-ahead, docs-behind, unknown version, and env-overridden docs ref.

Acceptance gate:

- [x] Agents can see the active docs bundle and mismatch warnings before following docs.
- [x] Version warnings are actionable and do not block normal local work.
- [x] Docs cache selection remains deterministic with `GOFORJ_ATLAS_DOCS_REPO` and `GOFORJ_ATLAS_DOCS_REF`.

## Phase 7: Resource Registry Depth

Purpose: turn the first resource registry into a stronger local operating map.

- [x] Extend GoForj resource registry entries with app, runtime, health, auth, and owner metadata where available.
- [x] Add resource categories for app, api, docs, mail, database, queue, cache, storage, events, observability, and operator.
- [x] Add lookup helpers for resource by app/runtime/category.
- [x] Connect database, queue, cache, disk, and event-bus names to the registry where config/env provides enough evidence.
- [x] Add tests for disabled resources and missing env values.
- [x] Expose richer registry output through Atlas `resource-inventory`.

Acceptance gate:

- [x] Atlas can answer "which local URL or named resource should I inspect for this app/runtime?"
- [x] Registry output remains deterministic and safe to expose to an agent.
- [x] Missing optional services produce clear absence, not invented defaults.

## Phase 8: User-Facing Debug Recipes

Purpose: make high-value workflows understandable to humans as well as agents.

- [x] Add docs recipes for route not found, API error, browser console error, Wire failure, job not running, schedule not firing, migration mismatch, and named-app confusion.
- [x] Each recipe lists Atlas tools, GoForj commands, expected evidence, and validation checks.
- [x] Link recipes from Atlas workflow skills and public GoForj docs.
- [x] Add docs tests or link checks if supported by the docs site.

Acceptance gate:

- [x] A GoForj user can follow the same evidence loop an agent should follow.
- [x] Recipes avoid telling users to edit generated files or guess ports.

## Recommended First Tranche

Start with the smallest set that will most improve quality feedback:

- [x] Phase 1 eval scoring package with five deterministic fixtures.
- [x] Phase 2 stdio MCP protocol tests for tool discovery and four critical tools.
- [x] Phase 4 formal ownership model backing `generated-file-policy`.

Why this order:

- Evals tell whether Atlas actually improves agent outcomes.
- Protocol tests prove the tools work through the surface agents really use.
- Ownership modeling prevents the most expensive class of framework mistakes.

## Validation Commands

Atlas:

```bash
GOCACHE=/tmp/gocache GOMODCACHE=/tmp/gomodcache go test ./...
```

GoForj integration work:

```bash
GOCACHE=/tmp/gocache GOMODCACHE=/tmp/gomodcache go test ./internal/forj/atlas ./internal/forj/resources
GOCACHE=/tmp/gocache GOMODCACHE=/tmp/gomodcache go test -tags integration ./internal/forj/atlas -run TestAtlasMCPServerUsesRenderedProjectInventory -count=1
```

GoForj docs:

```bash
npm run build
```

## Completion Gate

- [x] Every phase implemented or intentionally moved to a future backlog with a reason.
- [x] All acceptance gates pass.
- [x] Atlas, GoForj focused integration, and GoForj docs validation commands pass.
- [x] Public docs explain the user-facing behavior.
- [x] Roadmap is updated with current status and known residual risks.
