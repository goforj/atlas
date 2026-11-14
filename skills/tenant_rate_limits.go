package skills

const tenantRequestRateLimits = `
# GoForj Tenant Request Rate Limits

Use this skill when a task introduces request quotas, tenant budgets, or rate limiting. Inspect Atlas workflow-plan, resource-inventory, and docs-section-pack before choosing the policy.

Choose the deployment policy before writing middleware:

- A process-local middleware limiter controls one process. It cannot enforce one shared budget across replicas.
- A cross-replica tenant budget needs a shared backend with atomic counters, such as Redis, and the same resource endpoint and namespace on each replica. Memory, file, and null backends do not satisfy that scope.
- Borrow the existing named cache through generated accessors, for example manager.Requests(), at the App provider boundary. Services and middleware must not construct backend drivers or secretly replace the injected cache with a local store.

Use identity established by validated authentication. A raw X-Tenant-ID header is a caller assertion unless the task explicitly defines and verifies a trusted upstream contract. Apply authentication before tenant policy and keep the policy on the intended route so health, login, and unrelated routes remain unaffected.

State the quota, window semantics, exhaustion status, Retry-After behavior, and backend error policy. Fail-open and fail-closed choices have different availability and abuse consequences; follow the explicit application requirement and keep backend failure distinct from quota exhaustion.

Use atomic counters rather than Get followed by Set. The compatible cache root v0.4.0 exposes Increment and RateLimit; cachecore and Redis driver modules are independently versioned. RateLimit uses wall time. For an explicitly injected-clock fixed-window policy, derive a bounded key from validated tenant identity and the UTC time bucket, call WithContext(request.Context()).Increment, and use a cleanup TTL beyond the bucket end. Handle counter errors before running the protected operation. Avoid user-controlled cache namespaces, credentials in keys, and tenant IDs in metric labels.

Configure CACHE_REQUESTS_* with the existing Cache component and inspect the local CLI before generating missing structure. Use forj [app] generate --cache and forj [app] build when configuration changes. Do not edit generated manager/accessor or Wire output. Root and nested driver pins must be checked independently; do not upgrade the Go version just to use newer library releases.

Verify two policy instances against the same backend, distinct tenants, concurrent exhaustion, reset through an injected clock, backend failure, rejected identity, and an unaffected route. An embedded Redis protocol server provides deterministic backend coverage; it does not prove production topology, failover, or durability. Cache locks need separate ownership and release-on-failure tests.
`
