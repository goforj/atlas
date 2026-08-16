# Live Agent Evaluation Implementation Plan

## Authority

The normative architecture is the GoForj design at
`docs/designs/atlas-live-agent-evaluation-design.md`, committed on the
`docs/atlas-live-agent-evaluation` branch. This plan tracks implementation
order and repository ownership. If this plan and the design disagree, the
design wins until both are updated in review.

The ignored root `IMPLEMENTATION.md` is local scratch material. It is not an
implementation dependency or reviewable source of truth for this work.

## First Deliverable

The first useful vertical slice compares a fresh Codex agent with no guidance
against the same agent with GoForj's baseline `AGENTS.md`. Both attempts begin
from the same prepared invoice HTTP Project and receive the same natural task.
The slice records diagnostic evidence, independently verifies the resulting
Project, and reports framework outcome separately from generator-workflow
conformance.

This slice is diagnostic and local. It does not make authoritative isolation,
security, or release-gate claims.

## Implementation Status

The diagnostic vertical slice is implemented locally across Atlas and GoForj:

- the Codex adapter feasibility boundary is recorded and tested;
- scenario schema v2, prepare-prefix execution, and command-local immutable
  prepared bases are implemented in GoForj;
- the Atlas runner, guidance treatments, verifier, redacted authenticated
  artifacts, focused supervisor diff, and diagnostic report are implemented;
- the `invoice-http-route` scenario and `add-http-controller` evaluation run as
  paired `none` and `agents` treatments; and
- a fresh live paired run completed from the same guidance-neutral prepared
  tree, passed the current verifier and supervisor-owned behavior oracle in
  both treatments, and correctly reported workflow conformance as ineligible
  without trusted command evidence; and
- the current verifier is calibrated locally against two compiled, behaviorally
  valid implementation families plus a compilable response mutant, without
  treating candidate-authored tests as its behavior oracle.

The remaining integration gate is release ordered. Atlas must be published
with the new evaluation packages before GoForj can update its module pin and
pass the intentionally workspace-independent `GOWORK=off` integration build.
After those releases are available, rerun the paired diagnostic from the
released GoForj binary so retained runtime identities are retrievable release
versions rather than local `(devel)` builds.

## Local Readiness Evidence

The local implementation is supported by direct evidence at each completed
gate:

| Gate | Evidence |
| --- | --- |
| Adapter feasibility | `TestLiveCodexAppServerFeasibility`, `TestAdapterRunsFreshAttributedDiagnosticSession`, and the process-group tests prove fresh-thread attribution, isolated agent state, interruption, and descendant termination. |
| GoForj preparation | Scenario decoder, plan, preparation, clone, tree, and documentation-parity tests cover strict v2 YAML, unchanged v1 behavior, dependency ordering, target omission, symlink rejection, independent writable copies, and lexical tree identities. The tagged `TestInvoiceHTTPRouteScenarioIntegration` exercises the complete golden path. |
| Atlas evaluation core | Runner, artifact, manifest, diff, report, diagnostic, workflow, and isolation tests cover capability preflight, baseline timing, cancellation, timeout, cleanup, sealing before verification, redaction, authenticated artifacts, and separate outcome and conformance endpoints. |
| Guidance ownership | Guidance reconciliation tests cover every native target, managed-block ownership, stable target selection, and legacy inference. Tagged `TestBaselineGuidanceSurvivesProjectLifecycle` proves baseline guidance survives render, build, and a representative generator workflow. |
| Diagnostic slice | The promoted manifest, workflow, and verifier tests bind the three versioned contracts. The verifier injects a supervisor-owned 200/404 behavior oracle only after the agent stops and only into its disposable clone; tagged integration coverage proves both the generated-package and independently structured transport-package implementations pass while a compilable response mutant fails independently of candidate-authored tests. Tagged preparer tests also prove the invoice starting state and same-base paired treatments. Trial `trial-20260816t003503-7707b4c6a6b2` passed the current verifier in both live treatments; its complete artifact sets authenticate successfully and record the same prepared tree, scenario plan, environment, executable, model, and prompt identities. |

A prior unassisted live attempt produced a valid transport-package
implementation using `InvoiceController`, `NewInvoiceController`, and an
injected `*invoices.Service`. The then-current verifier rejected that family
because it still assumed the golden `Controller` and `NewController` names.
That attempt remains retained as failed calibration evidence; it was not rerun
until green. The verifier now derives the route-owning type and its constructor
from the candidate AST, accepts both implementation families, and still
rejects direct and qualified repository dependencies, wrong routes, missing
registration, missing context propagation, and build failures.

The current local pair passed all framework checks, including the hidden invoice
behavior probe, while correctly leaving generator conformance ineligible on the
unconfined backend. Provider telemetry showed both candidates used the
generator, but the report did not promote that untrusted observation into a
workflow claim. The release-qualified paired run below must reproduce these
properties with retrievable release identities instead of local `(devel)`
builds.

Both treatments passed this single local pair, so it demonstrates the harness
and current verifier rather than an effect from baseline guidance. Guidance
efficacy remains a repeated-trial question under the randomized protocol; this
diagnostic must not be presented as comparative evidence.

Repository-wide local validation also includes:

- the complete Atlas test and vet suites plus race coverage for evaluation,
  adapter, isolation, verifier, process-group, and app-server packages;
- the complete GoForj root test and vet suites with `GOWORK=off` and a temporary
  local Atlas replacement;
- independent test and vet passes for GoForj's `integration` and
  `tools/renderwarm` modules;
- race coverage for the GoForj scenario, Atlas integration, and preparer
  packages; and
- all ten GoForj smoke render compositions built and tested beneath `/tmp`.

These checks qualify the local source relationship. They do not replace the
released-module and released-binary checks below.

## Repository Ownership

### GoForj

- Versioned Project scenario decoding and validation.
- One immutable scenario plan used by execution, preparation, and generated
  scenario documentation.
- Project rendering and preparation through a narrow exported boundary.
- The durable baseline-guidance selection and managed native projections.
- The `invoice-http-route` starting state and its framework verification
  facts.
- The thin `forj atlas:eval` command and GoForj Project context passed to
  Atlas.

### Atlas

- Evaluation manifests, typed workflow expectations, verifier registry, and
  result contracts.
- Guidance profiles and canonical guidance composition.
- Agent adapter contracts, lifecycle orchestration, evidence capture, and
  artifact retention.
- Independent behavioral, ownership, and conformance verification.
- Diagnostic reports, comparison semantics, and fake-agent test support.

### Shared Boundary

GoForj implements an Atlas-owned `ProjectPreparer` interface. Atlas requests a
scenario and preparation options; GoForj returns a resolved, immutable plan and
a private prepared Project. Atlas must not interpret GoForj scenario YAML or
run a second dependency walker.

## Execution Order

### Gate 0: Adapter Feasibility

Before introducing production evaluation contracts, prove one disposable
Codex run can satisfy the diagnostic adapter boundary:

1. Start a fresh, non-resumed session with an isolated agent home.
2. Capture the resolved CLI and model identity.
3. Keep provider credentials and delegated transport out of child shell
   environments.
4. Observe process lifecycle and terminate the complete descendant job.
5. Retain stdout, stderr, exit state, duration, and final filesystem changes.
6. Record which properties remain unavailable without an authoritative
   sandbox.

Deliverable: a small Atlas spike test and a checked-in feasibility note. Do not
promote Codex to the first adapter if freshness or lifecycle ownership cannot be
demonstrated.

### Gate 1: GoForj Preparation Contract

1. Add strict v1 and v2 scenario wire schemas.
2. Normalize both schemas into one immutable `ScenarioPlan`.
3. Preserve all existing v1 behavior and generated documentation.
4. Add v2 `prepare.steps`, `prepare.checks`, target `steps`, and target
   `checks` with stable IDs.
5. Require exactly one action per v2 step and structured argv for commands.
6. Add a prepare-prefix API that never applies target work or target checks.
7. Keep one dependency walker, action executor, and command runner.

Deliverable: GoForj tests proving v1 parity, strict decoding, dependency
semantics, target omission, oracle isolation, and documentation parity.

### Gate 2: Atlas Evaluation Core

1. Add strict evaluation-manifest decoding and promoted workflow/verifier
   registries.
2. Define terminal attempt states and separate framework-outcome from
   workflow-conformance results.
3. Add a fake preparer, fake agent, fake verifier, and fake backend.
4. Exercise cancellation, timeout, evidence capture, redaction, cleanup, and
   artifact-manifest behavior without a live model.

Deliverable: deterministic Atlas unit tests that cover the entire runner
lifecycle before the real adapter is connected.

### Gate 3: Baseline Guidance Ownership

1. Add the durable GoForj baseline-guidance setting.
2. Move managed native guideline projection and reconciliation behind the
   GoForj integration boundary while Atlas remains the canonical content
   composer.
3. Preserve user-authored content and every existing supported agent target.
4. Prove `none` leaves guidance absent and `agents` remains present after
   render, build, and representative generators.

Deliverable: compatibility tests for new Project, legacy Project, install,
update, custom, minimal, and skip modes.

### Gate 4: Diagnostic Vertical Slice

1. Add the `invoice-http-route` GoForj scenario.
2. Add `goforj-add-http-route/v1` typed workflow requirements.
3. Add the independent `add-http-controller/v1` verifier with calibrated
   positive fixtures and mutants. Install its hidden behavior probe only in
   the verifier-owned clone after the candidate tree is sealed, so candidate
   tests cannot define the behavioral oracle and the oracle cannot leak into
   measured changes.
4. Add the natural prompt adjacent to its evaluation manifest.
5. Connect the diagnostic Codex adapter.
6. Add the thin `forj atlas:eval` wrapper.
7. Run paired `none` and `agents` attempts from equivalent private Project
   copies.

Deliverable: retained, redacted artifacts and a report that states diagnostic
limitations explicitly. A successful run is evidence that the vertical slice
works, not yet evidence of authoritative isolation or release readiness.

## Pull Request Sequence

Keep changes independently reviewable and merge prerequisites before their
consumers:

1. Atlas: tracked plan and adapter feasibility spike.
2. GoForj: scenario schema v2 and preparation API.
3. Atlas: evaluation contracts, registries, fake runner, and artifacts.
4. GoForj and Atlas: baseline guidance reconciliation boundary, cross-linked.
5. GoForj: invoice scenario and thin command surface.
6. Atlas: route expectation, verifier, Codex diagnostic adapter, and first
   evaluation.

Cross-repository pull requests must link the exact dependency and must not
assume an unpublished sibling version is available outside the repository
workspace.

## Publication Handoff

Publication is deliberately Atlas-first because GoForj imports the new Atlas
evaluation packages. Reconfirm remote tags immediately before release; from
the currently observed `v0.3.1` Atlas and `v0.24.1` GoForj baselines, both
additive feature sets require the next minor versions:

1. Review and merge the Atlas implementation, then publish Atlas `v0.4.0`.
2. Verify `github.com/goforj/atlas@v0.4.0` through the Go module proxy with
   `GOWORK=off`.
3. Update GoForj's Atlas pin to `v0.4.0`, tidy without a workspace, and verify
   that no local replacement remains.
4. Repeat the GoForj root, nested-module, tagged integration, race, and smoke
   render checks with `GOWORK=off`.
5. Review and merge the GoForj implementation, then publish GoForj `v0.25.0`.
6. Resolve both released modules through the proxy and build the released
   GoForj command without a workspace.
7. Run one new paired `none` versus `agents` diagnostic through that released
   binary. Verify authenticated manifests, equal preparation identities,
   treatment-specific baselines, absent guidance files in measured diffs,
   expected diagnostic evidence ineligibility, and released runtime versions
   instead of `(devel)`.

Do not tag GoForj while its module still selects Atlas `v0.3.1`, and do not
use the local workspace run as the final release-qualified evidence.

## Validation Discipline

- Run normal deterministic tests without invoking a live model.
- Validate every relevant nested Go module independently.
- Render GoForj Projects only beneath `/tmp`.
- Regenerate checked-in outputs from their authoritative templates and verify
  a second generation is clean.
- Run live evaluations only through an explicit command or test tag.
- Keep authoritative sandbox claims disabled until the container or VM backend
  and negative isolation suite exist.
- Treat `limits.commands` as post-run diagnostic validation under the local
  backend. An authoritative backend must enforce the same limit online from
  its supervisor-owned command stream and terminate the owned job at the
  boundary; the limit is already passed in `BackendRequest` for that purpose.
- Treat failed attempts as artifacts to classify, not flaky tests to rerun
  until green.

## Deferred Until Measured

Do not add persistent Project caching, shared writable dependency caches,
parallel live trials, release thresholds, an LLM judge, or additional provider
adapters in the first slice. Repeated Project preparation may use a private
ephemeral base copied into each trial. Persistent content-addressed caching is
promoted only after representative preparation measurements justify its
security and maintenance cost.

The promoted `goforj-add-http-route/v1` workflow records only an exact,
successful GoForj generator invocation. That is sufficient to keep the local
diagnostic honest because its backend cannot supply trusted command evidence
and reports conformance as ineligible. It is not the authoritative generator
contract described by the design. Before an authoritative backend is enabled,
promote a new workflow version whose typed predicates include normalized App
and resource identity, generated and protected paths, command ancestry,
write attribution, permitted post-generation edits, and continued use of the
generated registration outputs. Calibrate each predicate independently; do
not reinterpret the existing `/v1` result as that stronger claim.

## Completion Checkpoint

The design is ready for broader scenario work when:

- the adapter feasibility gate is recorded;
- v1 scenario behavior remains compatible while v2 preparation is available;
- the Atlas fake runner proves lifecycle and artifact invariants;
- baseline guidance is durable and attributable;
- the controller verifier rejects known incorrect mutants; and
- a fresh paired diagnostic can be reconstructed from retained identities
  without relying on a maintainer's normal home, conversation, or Project
  tree. The first slice does not retain a sealed Project bundle, so it promises
  provenance-guided diagnosis and a fresh stochastic rerun, not deterministic
  verifier replay of the original candidate tree.

The first five conditions are complete locally. The final condition becomes a
release-qualified claim only after the Atlas publication, GoForj pin update,
`GOWORK=off` validation, and one paired run using those released identities.
