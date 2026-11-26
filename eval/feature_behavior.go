package eval

import "context"

// runFeatureLibraryUseProbe records observed library execution as an independent quality signal.
func runFeatureLibraryUseProbe(ctx context.Context, runner CommandRunner, project VerifierProject) EndpointResult {
	result := runIsolatedCommand(ctx, runner, project, commandContract{id: "feature-library-execution", arguments: []string{"go", "test", "./internal/checkout", "-run", "^TestAtlasFeatureLibraryExecution$", "-count=1"}, supervisorFiles: []supervisorFile{{path: "internal/checkout/atlas_eval_feature_test.go", body: featureRolloutBehaviorProbe}, {path: "internal/features/atlas_eval_manager.go", body: featureManagerFixtureSource}}})
	result.Kind = RequirementQuality
	return result
}

// featureRolloutBehaviorProbe exercises the application boundary against native controlled configuration.
const featureRolloutBehaviorProbe = `package checkout

import (
    "context"
    "errors"
    "fmt"
    "sync/atomic"
    "testing"
    "time"

    "github.com/goforj/feature"
    "example.com/invoiceeval/internal/features"
)

// TestAtlasFeatureRolloutBehavior rejects unstable targeting and mixed request revisions.
func TestAtlasFeatureRolloutBehavior(t *testing.T) {
    ctx := context.Background()
    now := time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC)
    start := now.Add(time.Hour)
    document := feature.Document{Schema: feature.SchemaVersion, Flags: map[string]feature.Flag{
        "checkout-v2": {Default: false, Rules: []feature.Rule{{Value: true, Rollout: &feature.Rollout{BasisPoints: 5000, Salt: "checkout-2026"}}}},
        "receipt-v2": {Default: false},
    }}
    source := feature.SourceFunc(func(context.Context) (feature.Document, error) { return document, nil })
    engine, err := feature.Open(ctx, source, feature.WithClock(func() time.Time { return now }))
    if err != nil { t.Fatal(err) }
    flags := feature.New(engine)
    service := NewService(features.AtlasEvaluationManager(engine, flags))
    if _, err := service.Begin(ctx, ""); err == nil { t.Fatal("empty tenant accepted") }
    seen := map[bool]bool{}
    for i := 0; i < 128; i++ {
        tenant := fmt.Sprintf("tenant:%d", i)
        expected := flags.For(feature.Target{Key: tenant}).Evaluate(ctx, "checkout-v2", false).Value
        for attempt := 0; attempt < 8; attempt++ {
            request, err := service.Begin(ctx, tenant)
            if err != nil { t.Fatal(err) }
            got := request.Checkout(ctx)
            if got != expected { t.Fatalf("unstable or incorrect assignment for %s", tenant) }
            seen[got] = true
        }
        if err := engine.Refresh(ctx); err != nil { t.Fatal(err) }
        request, err := service.Begin(ctx, tenant)
        if err != nil || request.Checkout(ctx) != expected { t.Fatalf("refresh changed assignment: %v", err) }
    }
    if len(seen) != 2 { t.Fatal("tenant rollout collapsed into one decision") }

    // A channel orders a concurrent publication between related request decisions.
    document = feature.Document{Schema: feature.TypedSchemaVersion, Flags: map[string]feature.Flag{
        "checkout-v2": {Default: false}, "receipt-v2": {Default: false, Rules: []feature.Rule{{Value: true, Start: &start}}},
    }}
    if err := engine.Refresh(ctx); err != nil { t.Fatal(err) }
    request, err := service.Begin(ctx, "tenant:acme")
    if err != nil { t.Fatal(err) }
    if request.Checkout(ctx) { t.Fatal("unexpected checkout before update") }
    document.Flags = map[string]feature.Flag{"checkout-v2": {Default: true}, "receipt-v2": {Default: true}}
    done := make(chan error, 1)
    go func() { done <- engine.Refresh(ctx) }()
    if err := <-done; err != nil { t.Fatal(err) }
    if request.Receipt(ctx) || request.Checkout(ctx) { t.Fatal("request mixed configuration revisions") }
    fresh, err := service.Begin(ctx, "tenant:acme")
    if err != nil || !fresh.Checkout(ctx) || !fresh.Receipt(ctx) { t.Fatalf("new request missed update: %v", err) }

    document.Schema = -1
    if err := engine.Refresh(ctx); !errors.Is(err, feature.ErrInvalidDocument) { t.Fatalf("invalid refresh: %v", err) }
    retained, err := service.Begin(ctx, "tenant:acme")
    if err != nil || !retained.Checkout(ctx) || !retained.Receipt(ctx) { t.Fatalf("invalid refresh lost valid state: %v", err) }
    document = feature.Document{Schema: feature.SchemaVersion, Flags: map[string]feature.Flag{}}
    if err := engine.Refresh(ctx); err != nil { t.Fatal(err) }
    missing, err := service.Begin(ctx, "tenant:acme")
    if err != nil || missing.Checkout(ctx) || missing.Receipt(ctx) { t.Fatalf("missing flags ignored false fallback: %v", err) }

    document = feature.Document{Schema: feature.TypedSchemaVersion, Flags: map[string]feature.Flag{
        "receipt-v2": {Default: false, Rules: []feature.Rule{{Value: true, Start: &start}}},
    }}
    if err := engine.Refresh(ctx); err != nil { t.Fatal(err) }
    scheduled, err := service.Begin(ctx, "tenant:acme")
    if err != nil { t.Fatal(err) }
    now = start
    if scheduled.Receipt(ctx) { t.Fatal("request did not capture evaluation time") }
    scheduledFresh, err := service.Begin(ctx, "tenant:acme")
    if err != nil || !scheduledFresh.Receipt(ctx) { t.Fatalf("new request missed scheduled launch: %v", err) }
}

// TestAtlasFeatureLibraryExecution records adoption separately from required presentation behavior.
func TestAtlasFeatureLibraryExecution(t *testing.T) {
    ctx := context.Background()
    source, err := feature.NewMemorySource(feature.Document{Schema: feature.SchemaVersion, Flags: map[string]feature.Flag{"checkout-v2": {Default: true}, "receipt-v2": {Default: true}}})
    if err != nil { t.Fatal(err) }
    engine, err := feature.Open(ctx, source)
    if err != nil { t.Fatal(err) }
    var count atomic.Int64
    flags := feature.New(engine, feature.WithObserver(feature.ObserverFunc(func(context.Context, feature.Observation) { count.Add(1) })))
    request, err := NewService(features.AtlasEvaluationManager(engine, flags)).Begin(ctx, "tenant:acme")
    if err != nil { t.Fatal(err) }
    before := count.Load()
    request.Checkout(ctx)
    if count.Load() == before { t.Fatal("checkout did not evaluate the injected library") }
    before = count.Load()
    request.Receipt(ctx)
    if count.Load() == before { t.Fatal("receipt did not evaluate the injected library") }
}
`

// featureManagerFixtureSource supplies controlled native definitions only inside the sealed verifier phase.
const featureManagerFixtureSource = `// Package features owns generated evaluator resources.
package features

import "github.com/goforj/feature"

// AtlasEvaluationManager binds the real generated Checkout accessor to the supervisor-owned native source and clock.
func AtlasEvaluationManager(client *feature.Client, flags *feature.Features) *Manager {
    return &Manager{resources: []managedEvaluator{{name: "checkout", driver: "memory", client: client, flags: flags}}}
}
`
