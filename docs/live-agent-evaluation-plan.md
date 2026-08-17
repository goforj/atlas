# Live Agent Evaluation Implementation Plan

## Authority

The normative architecture is the GoForj design at
`docs/designs/atlas-live-agent-evaluation-design.md`, committed on the
`docs/atlas-live-agent-evaluation` branch. This plan tracks implementation
order and repository ownership. If this plan and the design disagree, the
design wins until both are updated in review.

The ignored root `IMPLEMENTATION.md` is local scratch material. It is not an
implementation dependency or reviewable source of truth for this work.

## Delivered Diagnostic Slice

The first useful vertical slice compared a fresh Codex agent with no guidance
against the same agent with GoForj's baseline `AGENTS.md`. The implemented
slice now applies that treatment boundary to the original 13-evaluation core
portfolio. Paired attempts begin from the same guidance-neutral prepared
Project, receive the same natural task, retain diagnostic evidence, and report
framework outcome separately from generator-workflow conformance.

This slice is diagnostic and local. It does not make authoritative isolation,
security, or release-gate claims.

## Implementation Status

The diagnostic core portfolio is implemented across Atlas and GoForj:

- the Codex adapter feasibility boundary is recorded and tested;
- scenario schema v2, prepare-prefix execution, and command-local immutable
  prepared bases are implemented in GoForj;
- the Atlas runner, guidance treatments, verifier, redacted authenticated
  artifacts, focused supervisor diff, and diagnostic report are implemented;
- 32 GoForj scenario fixtures establish reusable starting state and exercise
  the 30 promoted controller, command, job, migration, schedule,
  event/subscriber, model, additional-App, named resource, Wire-repair, and
  clarification evaluations, including six application-shaped capstones that
  cross HTTP, repositories, cache, storage, events, jobs, and scheduling, plus
  focused validation, middleware, transaction, mail, Auth, lifecycle, and
  outbound HTTP integration cases;
- `forj atlas:eval list`, `run`, `report`, `compare`, and `suite` expose the
  diagnostic portfolio without requiring a separate Atlas executable;
- every promoted verifier is calibrated against a golden Project and a
  targeted structural mutant; transaction, cache-aside, named-storage,
  conditional-image, validation, route-policy, command, and HTTP-controller
  evaluations additionally execute supervisor-owned behavior probes against
  compiling semantic mutants;
- manifests classify scaffold, feature, repair, and abstention measurements so
  focused runs do not infer intent from names or mix unlike tasks; and
- live paired and targeted treatments exercise the real Codex adapter while
  correctly reporting workflow conformance and authoritative framework outcome
  as ineligible without trusted isolation and command evidence.

The pull requests currently use an exact Atlas pseudo-version so standalone
`GOWORK=off` builds and CI exercise the reviewed cross-repository boundary.
The remaining release gate is still Atlas-first: publish Atlas, replace the
pseudo-version with the release tag, then publish GoForj and rerun the portfolio
from released binaries so retained runtime identities no longer report local
`(devel)` builds.

## Local Readiness Evidence

The local implementation is supported by direct evidence at each completed
gate:

| Gate | Evidence |
| --- | --- |
| Adapter feasibility | `TestLiveCodexAppServerFeasibility`, `TestAdapterRunsFreshAttributedDiagnosticSession`, and the process-group tests prove fresh-thread attribution, isolated agent state, interruption, and descendant termination. |
| GoForj preparation | Scenario decoder, plan, preparation, clone, tree, and documentation-parity tests cover strict v2 YAML, unchanged v1 behavior, dependency ordering, target omission, symlink rejection, independent writable copies, and lexical tree identities. Tagged calibration covers the core golden Projects, targeted mutants, and safe abstention. |
| Atlas evaluation core | Runner, artifact, manifest, diff, authenticated report, diagnostic, workflow, and isolation tests cover capability preflight, baseline timing, cancellation, timeout, cleanup, repairable finalization, sealing before verification, redaction, and HMAC-backed post-run tamper evidence for artifacts, plus separate outcome and conformance endpoints. Verifier phases keep separate writable Go state while using the supervisor's prepared module archives as a local proxy before falling back to the declared upstream. This HMAC evidence is not adversarial authentication when an unconfined candidate and the supervisor share a UID, because that candidate can read the signing key or replace retained files. |
| Guidance ownership | Guidance reconciliation tests cover every native target, managed-block ownership, stable target selection, and legacy inference. Tagged `TestBaselineGuidanceSurvivesProjectLifecycle` proves baseline guidance survives render, build, and a representative generator workflow. |
| Diagnostic portfolio | Promoted manifests, workflows, and verifier tests bind all 30 versioned contracts and classify them as scaffold, feature, repair, or abstention measurements. Twenty-four paired live treatments exercised the original 12-scenario portfolio; corrected guided reruns covered named cache, queue, and storage, and a live migration treatment covered the original thirteenth evaluation. Newer application-shaped cases add golden and targeted-mutant calibration without presenting calibration as live treatment evidence. Behavior-sensitive feature contracts now add verifier-owned runtime probes or observable command output for transactions, caching, storage, image revalidation, payload validation, route policy, commands, controllers, lifecycle readiness, outbound HTTP, mail, uploads, JSON APIs, auth registration, events, jobs, and schedules. Scaffold contracts intentionally remain source, registration, compilation, or route-visibility measurements. Every result remains visibly diagnostic and authoritative endpoints remain ineligible. |

Live calibration found real harness defects rather than being rerun until
green. A valid transport-package controller exposed constructor-name
overfitting; cohesive named-resource implementations exposed type-name
overfitting; and storage tests exposed runtime-created directories being
misclassified as authored source. The corrected verifiers accept those
reviewed implementation families, keep nested storage source files inside the
ownership budget, and retain targeted mutant coverage. A verifier is not
described as behavior-complete merely because its source and build checks pass.

Several unguided controls also completed simpler tasks successfully. The live
runs therefore demonstrate the harness, current guidance behavior, and useful
failure discovery—not a treatment-effect estimate. Guidance efficacy remains
a repeated, randomized, authoritative-trial question. Candidate-reported
commands remain diagnostic telemetry and never become generator-conformance
evidence on the unconfined backend.

Repository-wide local validation also includes:

- the complete Atlas test and vet suites plus race coverage for evaluation,
  adapter, isolation, verifier, process-group, and app-server packages;
- the complete GoForj root test and vet suites from a clean rebased worktree
  with `GOWORK=off` and the exact remote Atlas pseudo-version;
- independent test and vet passes for GoForj's `integration` and
  `tools/renderwarm` modules;
- race coverage for the GoForj scenario, Atlas integration, and preparer
  packages;
- executable calibration of every promoted implementation verifier against a
  golden Project and targeted mutant, behavior-preserving structural evidence
  mutants for each supervisor probe, plus safe-abstention calibration; and
- focused scenario execution for migration and named cache, queue, and storage
  beneath `/tmp` after rebasing onto current GoForj `main`.

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
- One-sided behavior diagnostics plus independent ownership and conformance
  verification.
- Diagnostic reports, comparison semantics, and fake-agent test support.

The suite remains the release-sized portfolio (`core`). `task_kind` is the
orthogonal measurement dimension: `scaffold` asks whether framework workflows
are discovered and used, `feature` evaluates application behavior across one
or more surfaces, `repair` starts from an existing defect, and `abstention`
measures safe refusal. Callers can select either dimension without encoding
policy in directory names or prompt wording.

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
3. Add the `add-http-controller/v1` verifier with calibrated positive fixtures
   and mutants. Install its hidden behavior probe only in the verifier-owned
   clone after the candidate tree is sealed, so candidate tests cannot define
   the behavioral oracle and the oracle cannot leak into measured changes.
   Require a randomized supervisor-owned completion test to execute in the
   same isolated command before accepting the probe result. Candidate tests
   remain excluded from that clone, while immutable tests captured before the
   agent received the Project are restored and continue to run. This marker
   detects ordinary early termination but is not an adversarial isolation
   boundary: unconfined candidate code can inspect its verifier clone, so the
   behavior result remains diagnostic until the authoritative backend exists.
4. Add the natural prompt adjacent to its evaluation manifest.
5. Connect the diagnostic Codex adapter.
6. Add the thin `forj atlas:eval` wrapper.
7. Run paired `none` and `agents` attempts from equivalent private Project
   copies.

Deliverable: retained, redacted artifacts and a report that states diagnostic
limitations explicitly. A successful run is evidence that the vertical slice
works, not yet evidence of authoritative isolation or release readiness.

## Pull Request Sequence

The implementation is consolidated into two cross-linked review boundaries:

1. Atlas owns the tracked plan, adapter, contracts, runner, artifacts,
   verifiers, reports, and canonical guidance composition.
2. GoForj owns scenario schema and preparation, executable fixtures, durable
   native guidance projection, and the thin command surface.

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
- Keep frontend loading-state guidance outside the promoted score until a
  browser-capable verifier can observe timing, retained content, and transient
  indicators. Source tokens cannot prove that fast responses avoid flicker or
  that revalidation preserves already rendered content.
- Keep authoritative sandbox claims disabled until the container or VM backend
  and negative isolation suite exist.
- Treat `limits.commands` as a post-run adapter-telemetry diagnostic under the
  local backend. An overflow fails the diagnostic attempt, but remains
  non-authoritative and cannot establish workflow evidence. An authoritative
  backend must enforce the same limit online from its supervisor-owned command
  stream and terminate the owned job at the boundary; the limit is already
  passed in `BackendRequest` for that purpose.
- Treat failed attempts as artifacts to classify, not flaky tests to rerun
  until green.

## Deferred Until Measured

Do not add persistent Project caching, shared writable dependency caches,
parallel live trials, release thresholds, an LLM judge, or additional provider
adapters in the first slice. Repeated Project preparation may use a private
ephemeral base copied into each trial. Persistent content-addressed caching is
promoted only after representative preparation measurements justify its
security and maintenance cost. Disposable verifier phases may read prepared Go
module archives through a local proxy, but continue to receive independent
writable module and build caches.

The promoted `/v1` workflows record exact successful GoForj generator
invocations. That is sufficient to describe the intended local workflow while
the backend reports conformance as ineligible without trusted command evidence.
It is not the authoritative generator contract described by the design. Before
an authoritative backend is enabled, promote new workflow versions whose typed
predicates include normalized App and resource identity, generated and
protected paths, command ancestry, write attribution, permitted
post-generation edits, and continued use of generated registration outputs.
Calibrate each predicate independently; do not reinterpret `/v1` results as
that stronger claim.

## Current Checkpoint

The diagnostic portfolio is complete when:

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

All six conditions are complete for local diagnostics. The final condition
becomes a release-qualified claim only after Atlas publication, the GoForj pin
update, `GOWORK=off` validation, and a new run using released identities. The
next implementation phase is the authoritative sandbox and its negative
isolation suite, not more unconfined result promotion.
