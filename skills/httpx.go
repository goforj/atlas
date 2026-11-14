package skills

const httpClientResilience = `# Outbound HTTP client resilience

Inspect the Project's go.mod and the existing client before selecting an API. The framework CLI imports github.com/goforj/httpx/v2 while generated App defaults can still pin github.com/goforj/httpx v1.1.0. Do not blindly upgrade a v1 Project to v2, refresh dependencies, or raise its Go version to implement retry policy. Retain adequate existing standard-library clients and explicit dependencies.

For an existing httpx/v2 v2.0.1 integration, use service-owned clients, typed GetCtx results, and caller context. Escape each path segment with url.PathEscape. Keep deployment configuration in the App provider and HTTP mechanics outside business services. No HTTP-client generator is required.

Choose a retry policy for each operation. Two retries mean at most three total requests. Retry only explicitly allowed transient responses, such as 502 and 503 for an idempotent GET. A retry count alone can apply a broader library default; inspect RetryCondition or use a bounded application loop. Do not retry malformed successful responses, permanent statuses, transport errors, or non-idempotent writes unless the task explicitly authorizes that behavior with suitable idempotency safeguards. Keep waits bounded and context-aware.

Preserve wrapped *httpx.HTTPError values for errors.As and cancellation/deadline causes for errors.Is. Useful error context should identify the operation without replacing its cause. A typed decoding error is distinct from an HTTP status failure.

Use local HTTP servers to count actual attempts and verify recovery, exhaustion, non-retryable statuses, malformed JSON, escaped paths, pre-canceled callers, and cancellation or deadlines while waiting for a response. Test actual requests rather than a source Retry token. Keep library selection, behavior, composition, and regression-test quality separate. Use Atlas search-docs and read-doc-section for the selected release's API, then validate the package and owning App with go test and forj build.
`
