package skills

const featureRollouts = `
# GoForj Feature Rollouts

Use github.com/goforj/feature for stable targeted rollouts, typed decisions, or file-backed flags that can refresh while the App is running. Plain configuration is sufficient for a setting that applies to everyone and changes at deployment time. Authorize the tenant separately from the rollout.

For a supported CLI, check the component selection for the owning App in .goforj.yml. When adding feature, enable it there and run forj render from the Project root to install the App's Wire providers. For a named evaluator, declare its FEATURE_<NAME>_* settings in .env or the App overlay and run forj generate --feature to update the typed accessors. The generated internal/features/README.md covers file setup, startup, refresh, and cleanup. On older framework versions, consult the pinned native API before composing a provider yourself: https://github.com/goforj/feature/tree/v0.1.0.

Register the service constructor in the owning App's inject_services_app.go. Wire supplies *features.Manager. Select Default() or the named evaluator once in the constructor and retain the resulting *feature.Features. The App owns the sources and refresh workers. For a routine boolean check, bind a stable identity with flags.For(feature.Target{Key: tenantID}).Enabled(ctx, key). Enabled defaults to false. Evaluate accepts a caller-selected fallback and returns decision metadata.

When related decisions require one configuration revision and evaluation time, capture flags.Snapshot() once, handle its error, then bind the target on that snapshot. Ordinary For scopes can observe newer configurations on successive evaluations. Test that a request keeps its decisions across a refresh while a new request can see the update. Use the complete snapshot example in the generated README.
`
