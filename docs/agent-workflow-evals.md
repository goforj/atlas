# Agent Workflow Evals

Atlas workflow evals are deterministic fixtures that check whether a GoForj
agent workflow selects the right app, command path, files, docs, Atlas tools,
ownership policy, validation checks, and mistake warnings.

They do not call a live model. They exercise Atlas' planning surface so the
framework guidance can improve without network access.

## Adding A Regression

Use a regression fixture when a real agent transcript shows a repeatable
GoForj mistake, for example:

- editing `wire_gen.go`
- writing named-app routes into `app/routes.go`
- skipping `forj <app> make:*`
- guessing ports instead of using Atlas runtime evidence
- validating with a narrow check that does not prove the changed behavior

Add the smallest fixture to `workflows/metadata.go`:

```go
{
	Name:           "short regression name",
	Task:           "the user task that caused the mistake",
	App:            "marketplace",
	WantWorkflowID: "goforj-add-http-route",
	WantCommandParts: []string{
		"forj marketplace make:controller",
	},
	WantFileParts: []string{
		"app/marketplace/routes.go",
	},
	AvoidFileParts: []string{
		"app/routes.go",
		"wire_gen.go",
	},
}
```

Keep fixture text compact and redacted. Preserve the behavior that failed, not
the user's private data.

## Transcript Capture

`RunEvalFixtures(ctx, true)` includes a compact transcript of the deterministic
steps and scored checks. Use it when diagnosing a failed scorecard or when
turning a real transcript into a regression fixture.

The transcript should stay safe for test output:

- no secrets
- no full source files
- no raw user data
- no model-specific prompt logs unless already redacted

## Expected Failure Shape

A useful failure names the missing behavior directly:

- `command contains forj marketplace make:controller`
- `file avoids wire_gen.go`
- `tool includes wire-diagnostics`
- `validation contains route:list`

If a failure only says that a fixture failed, improve the check before adding
more fixtures.
