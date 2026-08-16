# Codex Adapter Feasibility

## Status

Qualified for the Linux local diagnostic backend. Not qualified for
authoritative execution.

The spike used Codex CLI `0.146.0` on 2026-08-15. The adapter must record its
own resolved executable and version because the protocol is versioned with the
installed CLI.

## Proven Surface

Two independent invocations of:

```text
codex exec --ephemeral --ignore-user-config --ignore-rules --sandbox read-only --json
```

completed successfully in the same disposable Git repository. They emitted
different `thread.started` identifiers, returned structured turn and usage
events, and did not write session files into the Project. This is sufficient to
confirm that a fresh, non-resumed diagnostic session is available.

A subsequent app-server test used a private Codex state containing only a
temporary copy of an explicitly supplied disposable login. The first attempt
correctly exposed that a shared state loaded the maintainer's global `AGENTS.md`; the private state
removed that instruction source. The adapter can therefore fail closed when
the server reports guidance outside the resolved profile.

The installed CLI can generate the JSON Schema for its app-server protocol.
The `thread/start` contract supports:

- an explicit model and provider;
- an ephemeral thread;
- an explicit working directory, approval policy, and sandbox;
- a response containing the effective model and provider;
- the exact instruction-source paths loaded for the thread; and
- lifecycle notifications for threads, turns, commands, file changes, MCP
  calls, model reroutes, and terminal state.

Those fields remove guesswork from guidance attribution and model identity.
They also support later scripted clarification without resuming an unrelated
session.

The published app-server reference does not fully describe the `thread/start`
response. The generated `0.146.0` protocol schema requires root-level
`approvalPolicy` and `sandbox` fields, so Atlas decodes and verifies those exact
fields before accepting a session. A missing or mismatched value fails closed
instead of treating requested policy as effective policy.

## Adapter Decision

Use `codex app-server` over its local stdio JSONL transport for the Atlas Codex
adapter. Pin and validate the minimum supported CLI protocol at preflight.

Do not build the adapter by parsing human output or by treating provider events
as authoritative command evidence. `codex exec --json` remains useful for
manual diagnostics, but its compact `thread.started` event does not expose the
effective model or loaded instruction sources required by the evaluation
contract.

The adapter should:

1. Resolve and fingerprint the Codex executable before entering the trial.
2. Start one app-server process in a supervisor-owned process group.
3. Complete `initialize` and `initialized` before any thread request.
4. Start a new ephemeral thread with an explicit model, Project root, approval
   policy, and sandbox policy.
5. Reject a response whose effective model, provider, approval policy,
   sandbox, or instruction sources differs from the resolved run plan.
6. Start one turn, normalize supported events, and retain unknown events as
   inert diagnostic data.
7. Interrupt the active turn before cancellation cleanup when possible.
8. Terminate and verify the complete supervised job on every terminal path.

The app-server reader retains at most 1,024 lifecycle notifications. It always
continues reading request responses; when telemetry exceeds that bound, it
counts discarded notifications and the adapter fails the affected turn with an
explicit telemetry-overflow error rather than waiting for a possibly dropped
terminal notification.

On Linux, app-server interruption and its own process group are not sufficient
for step 8. A live turn created a separate command process group, and a
background `sleep` survived after the command leader and app-server exited.
Atlas now tracks app-server descendants by immutable Linux PID creation
identity and terminates every still-matching observed group during cleanup.
The repeated deterministic process test and the opt-in live Codex test both
prove that a separately grouped descendant is removed. This polling tracker is
best-effort diagnostic containment; it is not authoritative process evidence.

## Unproven And Ineligible Claims

The current diagnostic adapter uses a file-backed Codex credential store. A local
Codex process and its shell descendants share the host read boundary, so an
isolated working directory, `--ephemeral`, or a reduced environment cannot
prove that provider authority is inaccessible to candidate code.

Therefore the local adapter cannot claim:

- brokered provider authority;
- credential non-exposure;
- complete host filesystem isolation;
- network isolation;
- supervisor-authoritative command or file observation; or
- absence of external mutation.

These endpoints must report `ineligible` under `unconfined-local`. The
authoritative backend still requires the external provider broker and isolated
container or VM boundary defined by the main design.

The local command therefore requires an explicit credential path instead of
falling back to the developer's normal Codex login. That credential must be
disposable, revocable, and restricted to the diagnostic. This limits accidental
exposure but does not create a security boundary; unconfined candidate code can
still read the copied credential while the attempt is active.

The current host cannot start Codex's Bubblewrap workspace sandbox because
unprivileged user namespaces are unavailable. The live diagnostic test
therefore uses `danger-full-access` inside a disposable Project and labels the
result unconfined. This is an environment capability failure, not a reason to
silently weaken an authoritative run.

## Implemented Feasibility Test

The opt-in Atlas integration test:

1. launches app-server with a minimal allowlisted environment;
2. creates two ephemeral threads and proves their identifiers differ;
3. verifies the effective model, provider, and instruction-source response;
4. runs a bounded command through one turn;
5. cancels while a separately grouped descendant process is active;
6. verifies app-server and every observed descendant have exited; and
7. confirms global instruction files are absent from the resolved thread.

The same process supervisor and protocol client pass deterministic tests with
a fake app-server, so ordinary `go test ./...` never depends on a live model.
Run the real gate explicitly with:

```text
ATLAS_LIVE_CODEX=1 go test ./internal/codexappserver \
  -run '^TestLiveCodexAppServerFeasibility$' -count=1
```

The remaining authoritative feasibility work belongs to the container or VM
backend: provider transport must stay outside the candidate boundary, and job
ownership must use a qualified primitive rather than `/proc` polling.

## Official Surface References

- Codex non-interactive mode:
  <https://learn.chatgpt.com/docs/non-interactive-mode.md>
- Codex app-server protocol:
  <https://learn.chatgpt.com/docs/app-server.md>
- Codex authentication and automation:
  <https://learn.chatgpt.com/docs/auth.md>
