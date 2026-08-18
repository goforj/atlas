# Atlas Workflow Skills Task Dump

This checklist tracks the Atlas workflow-skill work that turns GoForj framework
knowledge into agent-usable workflows.

## Core Workflow Layer

- [x] Add deterministic workflow planning package.
- [x] Add task classification for routes, commands, jobs, schedules, events, data resources, Wire repair, runtime debugging, multi-app changes, and validation.
- [x] Add deterministic command, file, docs, warning, and verification output for each workflow.
- [x] Add a deterministic task-to-docs map for workflow IDs.
- [x] Add workflow eval fixtures for common agent tasks.
- [x] Add scorecard output for workflow quality checks.
- [x] Add optional eval transcript capture for workflow success/failure analysis.
- [x] Add project-owned workflow overlays from `.ai/skills`.
- [x] Add regression fixtures from failed workflow-scorecard behavior.

## MCP Tools

- [x] Add `workflow-plan` MCP tool.
- [x] Add `registration-points` MCP tool.
- [x] Add `validation-plan` MCP tool.
- [x] Add `wire-diagnostics` MCP tool.
- [x] Add `scenario-guide` MCP tool.
- [x] Add `resource-inventory` MCP tool.
- [x] Add `generated-file-policy` MCP tool that classifies generated, app-owned, framework-owned, config-owned, and user-owned paths.
- [x] Add `command-advice` MCP tool that returns the preferred `forj` command for a task, app, and resource name.
- [x] Add `docs-section-pack` MCP tool that returns bounded docs sections in workflow order.
- [x] Add `workflow-scorecard` MCP tool for deterministic workflow fixture checks.
- [x] Improve `wire-diagnostics` to extract missing type, consumer, and likely provider set from Wire output.
- [x] Expand Atlas `resource-inventory` contract for named queues, caches, disks, event buses, database connections, frontend kit, and resource links.
- [x] Add GoForj CLI integration so `forj atlas:mcp` supplies real project/app/runtime resource data to the expanded Atlas inventory contract.
- [x] Add stable Lighthouse/operator resource links to `resource-inventory`.

## Built-In Skills

- [x] Add workflow skill for HTTP route changes.
- [x] Add workflow skill for app command changes.
- [x] Add workflow skill for queued job changes.
- [x] Add workflow skill for scheduler changes.
- [x] Add workflow skill for event/subscriber workflows.
- [x] Add workflow skill for data resources.
- [x] Add workflow skill for Wire repair.
- [x] Add workflow skill for runtime debugging.
- [x] Add workflow skill for multi-app changes.
- [x] Add workflow skill for validation planning.
- [x] Require workflow skills to include trigger, Atlas tools, command path, verification, and mistake guidance.
- [x] Add project or product-specific skill overlays for known GoForj starter kits beyond `.ai/skills` discovery.

## Tests

- [x] Add workflow package tests.
- [x] Add MCP handler tests for workflow tools.
- [x] Add skill catalog tests for workflow skill coverage.
- [x] Add skill quality tests for trigger, Atlas tools, command path, verification, and mistake guidance.
- [x] Add workflow scorecard tests for common agent tasks.
- [x] Add end-to-end MCP server smoke test driven by a rendered GoForj project in `/tmp`.
- [x] Add regression fixtures from real failed agent transcripts after the first workflow-scorecard runs are collected.

## Docs And Integrations

- [x] Return docs manifest metadata from `docs-section-pack` so agents can see the active docs bundle.
- [x] Add hosted or cached docs version selection to workflow docs references.
- [x] Consume GoForj's future first-class resource registry once that registry shape is stable.
- [x] Add public GoForj docs page for Atlas workflow skills after the Atlas API stabilizes.
- [x] Add agent-facing examples that show using `workflow-plan`, `docs-section-pack`, `generated-file-policy`, and `validation-plan` together.

## External Shape Resolved

- [x] GoForj resource registry consumption is implemented through `internal/forj/resources` in the GoForj repository. Atlas inventory now consumes that registry from `forj atlas:mcp` instead of the earlier hardcoded resource link helper.

## Cross-Repo Implementation Dump

This section tracks the concrete files and follow-through work across Atlas,
GoForj, and GoForj Docs.

### Atlas Repository

- [x] Add `workflows` package for deterministic task planning.
- [x] Add workflow metadata, docs mapping, overlays, scorecard fixtures, and eval support.
- [x] Add workflow MCP tool handlers in `mcp/tools_workflows.go`.
- [x] Register workflow tools in the Atlas MCP server.
- [x] Extend Atlas MCP tests for workflow tools and server registration.
- [x] Add built-in workflow skills to the skill catalog.
- [x] Add starter-kit-specific overlays for React and templ/htmx.
- [x] Add catalog tests for workflow skill quality and starter-kit overlays.
- [x] Add docs bundle ref/repo selection via `GOFORJ_ATLAS_DOCS_REPO` and `GOFORJ_ATLAS_DOCS_REF`.
- [x] Add this roadmap and status dump.
- [x] Re-run `GOCACHE=/tmp/gocache GOMODCACHE=/tmp/gomodcache go test ./...` after the GoForj registry follow-through lands.

### GoForj Repository

- [x] Wire `forj atlas:mcp` to pass project, diagnostics, and inventory providers into Atlas.
- [x] Add Atlas project discovery from `.goforj.yml`.
- [x] Add Atlas inventory discovery for routes, schedules, commands, queues, caches, disks, event buses, diagnostics metadata, and resource links.
- [x] Add unit tests for GoForj project discovery, inventory, and diagnostics.
- [x] Add rendered-project MCP integration smoke test in `/tmp`.
- [x] Finish the first-class `internal/forj/resources` package.
- [x] Add package documentation for `internal/forj/resources`.
- [x] Wire Atlas inventory resource links to consume `internal/forj/resources` instead of the temporary hardcoded resource link helper.
- [x] Add or update tests proving Atlas inventory consumes the resource registry.
- [x] Run `gofmt` on touched GoForj files.
- [x] Run `GOCACHE=/tmp/gocache GOMODCACHE=/tmp/gomodcache go test ./internal/forj/resources ./internal/forj/atlas`.
- [x] Run `GOCACHE=/tmp/gocache GOMODCACHE=/tmp/gomodcache go test -tags integration ./internal/forj/atlas -run TestAtlasMCPServerUsesRenderedProjectInventory -count=1`.
- [x] Run `GOCACHE=/tmp/gocache GOMODCACHE=/tmp/gomodcache go test ./...` in GoForj for broader package coverage.

### GoForj Docs Repository

- [x] Document Atlas workflow skills in `docs/developer-tools/atlas.md`.
- [x] Add agent-facing examples for `workflow-plan`, `docs-section-pack`, `generated-file-policy`, and `validation-plan`.
- [x] Document docs source selection with `GOFORJ_ATLAS_DOCS_REPO` and `GOFORJ_ATLAS_DOCS_REF`.
- [x] Re-run `npm run build` in the `docs` directory of the `goforj-docs` repository if the docs page changes again.

### Completion Gate

- [x] Remove or close every unchecked task in this dump.
- [x] Confirm `rg -n "\[ \]" docs/workflow-skills-roadmap.md` has no remaining unchecked roadmap tasks.
- [x] Report final validation commands and known unrelated dirty files.
