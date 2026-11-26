package skills

const businessMetrics = `
# GoForj Business Metrics

Use github.com/goforj/metrics when adding application counters or duration histograms to GoForj's existing registry. Inspect imports, the pinned API, the owning App's internal/metrics.Manager, and service Wire providers first. Preserve established instrumentation rather than creating parallel registries. This skill is available before metrics is enabled; inspect supported Project components before enabling an optional component.

Inject manager.Registry() from the selected App's service provider. Register instrument handles in the constructor, then update them at the actual business operation boundary. Increment successful completion counters only after success. ObserveDuration records time.Duration in the native histogram and exports seconds. Preserve errors and caller cancellation. Do not change Lighthouse or runtime capture settings to implement a business counter. There is no make:metric command.

Use bounded labels such as fixed outcome or static app identity. Never use tenant IDs, request IDs, URLs with dynamic segments, or error text as label values. A metric definition without updates is incomplete. Test actual counter values, histogram Count, Sum and buckets, failure exclusion, repeated registration, and the Prometheus Handler output in the same registry. Use an injected clock and operation fake rather than sleeping.

Start with workflow-plan and docs-section-pack. Read operations/metrics.md and the pinned native API at https://github.com/goforj/metrics/tree/v0.2.0. Keep application instruments in internal/<domain> and register their service provider in the owning App's inject_services_app.go. Run forj build to regenerate Wire and focused Go tests; never hand-edit wire_gen.go or the generated metrics manager.
`
