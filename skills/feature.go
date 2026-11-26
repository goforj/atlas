package skills

const featureRollouts = `
# GoForj Feature Rollouts

Use github.com/goforj/feature for stable targeted rollouts, typed flags, and explicit fallback policy. Inspect existing imports, go.mod versions, the owning App, and supported generators first. Preserve adequate standard-library configuration and established dependencies when a rollout does not warrant replacement. A feature flag is not authorization; authorize the tenant separately.

The framework constructs *features.Manager in the generated App Wire manager set and owns its cleanup. Register your application constructor in app/wire/inject_services_app.go (or app/<name>/wire/inject_services_app.go for a named App). Wire supplies the existing manager. For a declared Checkout evaluator, select it once in the constructor:

    // NewService borrows the App-managed checkout evaluator.
    func NewService(features *features.Manager) *Service {
        return &Service{flags: features.Checkout()}
    }

Do not edit generated inject_managers.go or construct another manager in the service. App.Features() is available to callers that already own the App; it is not a service dependency or a service locator. For ordinary boolean checks, use the injected evaluator's Enabled method. Bind a target when the flag uses tenant targeting:

    flags := service.flags.For(feature.Target{Key: tenantID})
    enabled := flags.Enabled(ctx, "checkout-v2")

Enabled returns a boolean with a false fallback. Use Evaluate(ctx, key, fallback) for an explicit boolean fallback or full decision metadata, and the typed decision methods for other values. Use a stable tenant identity as Target.Key and a stable rollout salt. Random per-request decisions defeat stable assignment.

Features.For(target) creates an ordinary targeting Scope, not a provider configuration snapshot. When related decisions must share one configuration revision and evaluation time, call Features.Snapshot() once, handle unsupported snapshot errors explicitly, then use the captured facade's For(target). Do not reread the live facade between those decisions. Ordinary independent checks do not require snapshots.

Use the owning App's optional feature component when the pinned GoForj version supports it. Declare evaluator resources with FEATURE_* configuration, then run forj generate --feature for generated internal/features.Manager and typed Default()/Checkout() accessors. Named Apps use their existing environment overlay. Resource names identify configured evaluators, not individual flags. Inject the generated manager into services and obtain the appropriate accessor. Keep source ownership, initial loading, Refresh, and cancel/join cleanup in the generated App lifecycle; never construct sources or refresh loops per request. Invalid refresh degrades detached Instances() freshness status while retained valid configuration stays ready. Invalid refresh must retain the last valid snapshot. Use controlled sources and WithClock in tests; verify repeated assignment, tenant separation, missing fallback, invalid-refresh retention, and consistency across updates when required.

Start with workflow-plan and docs-section-pack for the task. Read applications/library-resources.md ('Configure a Feature Evaluator') for framework composition and libraries/feature.md for the native snapshot contract.

There is no make:feature generator. On older framework versions without the feature component, use application-owned composition with the pinned native API rather than inventing a command. Keep the rollout service application-owned under internal/<domain> and register constructors through the owning App's service Wire set. Inspect the pinned package's exported API and README when hosted docs are unavailable: https://github.com/goforj/feature/tree/v0.1.0. Optional vendor adapters have separate modules and lifecycle contracts.
`
