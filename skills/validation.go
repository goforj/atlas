package skills

// validationLibrary teaches the released presence and failure contracts without replacing adequate existing checks.
const validationLibrary = `
# GoForj Validation

Choose github.com/goforj/validation v0.2.0 when several entrypoints share typed rules, structured field issues, context-aware checks, or PATCH presence semantics. Keep adequate simple standard-library checks for small local invariants and preserve an explicitly selected existing validator unless migration is requested. This library does not replace authorization or persistence constraints.

Inspect the selected module version and nearby service/controller boundaries first. Search Atlas docs for validation and read the relevant library section before guessing APIs. Dependency installation belongs in an authorized preparation step when the execution environment has shell network disabled. There is no make:validation generator.

Use validation.Input[T] for JSON fields that distinguish omission from null. Its zero value is absent; Provided(value) includes zero and false, Null[T]() is explicit null. Get() reports only present non-null values. A plain pointer collapses omission and null. Presence(path, input, func(value T) validation.Field { ... }) skips absent children, rejects null by default, and Nullable() permits clearing a nullable field. Required() on Presence requires a supplied non-null value but accepts zero; String.Required rejects an empty string. Use an empty child path to retain the parent JSON field name.

Normalize at the application boundary before constructing fields. Share a ValidatePatch(ctx, input) method or reusable RuleFunc[T]/ContextRuleFunc[T] with Using or Check, then call validation.ValidateContext with the request context. For example:

    validation.Presence("limit", input.Limit, func(value int) validation.Field {
        return validation.Number("", value).Min(0).Max(100)
    })

Treat constructor-injected policies as required unless their API explicitly declares them optional. Do not silently skip a missing policy: missing collaborators are bad wiring, not an allow-all mode.

Apply state only after validation succeeds. Do not treat omitted values as defaults or skip supplied zero/false. Separate malformed JSON (400 invalid_payload), field violations (422 validation_failed with fields[path] = stable code), and operational failures. Use errors.As into *validation.Errors and Issues() Path/Code for the public adapter; do not serialize rule errors or localized messages as field codes. Return validation.Invalid(code, safeMessage) only for intentional client violations. Ordinary rule errors, cancellation, and *validation.ConfigurationError are operational failures and must retain their identity rather than becoming 422 responses.

Verify omitted values, explicit null, zero/false, normalized strings, exact field paths/codes, direct service reuse, cancellation, and operational rule errors. Keep HTTP route generation, App registration, and Wire composition in the normal route workflow.
`
