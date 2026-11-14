package eval

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
    var observations atomic.Int64
    flags := feature.New(engine, feature.WithObserver(feature.ObserverFunc(func(context.Context, feature.Observation) { observations.Add(1) })))
    service := NewService(flags)
    if _, err := service.Begin(ctx, ""); err == nil { t.Fatal("empty tenant accepted") }
    seen := map[bool]bool{}
    for i := 0; i < 128; i++ {
        tenant := fmt.Sprintf("tenant:%d", i)
        expected := flags.For(feature.Target{Key: tenant}).Evaluate(ctx, "checkout-v2", false).Value
        for attempt := 0; attempt < 8; attempt++ {
            request, err := service.Begin(ctx, tenant)
            if err != nil { t.Fatal(err) }
            before := observations.Load()
            got := request.Checkout(ctx)
            if observations.Load() == before { t.Fatal("feature library was not evaluated on the application path") }
            if got != expected { t.Fatalf("unstable or incorrect assignment for %s", tenant) }
            seen[got] = true
        }
        if err := engine.Refresh(ctx); err != nil { t.Fatal(err) }
        request, err := service.Begin(ctx, tenant)
        if err != nil || request.Checkout(ctx) != expected { t.Fatalf("refresh changed assignment: %v", err) }
    }
    if len(seen) != 2 { t.Fatal("tenant rollout collapsed into one decision") }

    // A channel orders a concurrent publication between related request decisions.
    document = feature.Document{Schema: feature.SchemaVersion, Flags: map[string]feature.Flag{
        "checkout-v2": {Default: false}, "receipt-v2": {Default: false, Rules: []feature.Rule{{Value: true, Start: &start}}},
    }}
    if err := engine.Refresh(ctx); err != nil { t.Fatal(err) }
    request, err := service.Begin(ctx, "tenant:acme")
    if err != nil { t.Fatal(err) }
    if request.Checkout(ctx) { t.Fatal("unexpected checkout before update") }
    now = start
    if request.Receipt(ctx) { t.Fatal("request did not capture evaluation time") }
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
}
`
