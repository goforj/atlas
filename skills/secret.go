package skills

// secretReads separates secret custody from configuration and payload encryption before a dependency is installed.
const secretReads = `
# GoForj Bound Secret Reads

Use this skill when application code needs a credential from an encrypted file or managed provider. Inspect existing imports, configuration, and credential custody first. Preserve the established reader when it meets the task. Environment configuration (env/v2), payload encryption (crypt), and bound secret lookup (secret) have separate responsibilities.

Inspect github.com/goforj/secret through Atlas search-docs and read-doc-section at libraries/secret.md, or the package's versioned source and README. Verify root and selected driver/test-module tags independently before pinning a dependency. At the initial assessment there were no tags or releases; a candidate badge does not establish publication. Released-module evaluation remains planned. Never guess v0.1.0 or quietly use a local replacement.

Consumers normally need secret.Reader with Get(ctx, name) (string, error). Bind uppercase names such as DATABASE_PASSWORD to provider locations during startup construction. Inject the reader through the owning service constructor and service Wire set. Keep provider SDKs, paths, credentials, key custody, access policies, and provisioning outside leaf services. Inspect the Project's actual framework integration before assuming an accessor or make:secret command exists. The standalone secret contract requires no framework component or generated manager.

Forward caller context into every read. Preserve the exact returned string, including whitespace and an empty successful value. Classify ErrNotFound, ErrPermission, ErrUnavailable, ErrInvalid, and cancellation with errors.Is as the application requires. Do not silently turn errors into an empty credential or fallback value without explicit policy.

Built-in driver errors omit provider details, but secret.Func adapts application code and does not sanitize its errors. Keep arbitrary provider errors, values, endpoints, and binding locations out of public responses, logs, metrics, and inspects. Return stable application errors and safe classifications.

Reader injection narrows the consumer API, not provider authorization. Read access does not authorize Set or Delete. Inject Reader rather than Store for read-only work. ErrIndeterminate concerns uncertain mutations and requires reconciliation, not blind retries. Rotation and deletion are separate tasks.

Verify exact-value success, classified failures, and cancellation with a fake reader and fake credentials. Check diagnostics using a synthetic sentinel, never real secrets. Resolve the actual version before using the separately versioned secrettest module. Cloud credentials are unnecessary for these unit tests. Run managed-provider integration only when the task requires it.
`
