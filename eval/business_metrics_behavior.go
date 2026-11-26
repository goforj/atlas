package eval

// businessMetricsBehaviorProbe observes application results and real registry state with a deterministic clock.
const businessMetricsBehaviorProbe = `// Package checkout owns instrumented checkout operations.
package checkout
import (
 "context"
 "errors"
 "fmt"
 "math"
 "testing"
 "time"
 "example.com/invoiceeval/internal/metrics"
)
// atlasMetricsProcessor advances the operation clock and retains the caller's boundary evidence.
type atlasMetricsProcessor struct { clock *time.Time; duration time.Duration; err error; context context.Context; tenant, request string }
// Complete makes elapsed operation time deterministic without sleeping.
func (processor *atlasMetricsProcessor) Complete(ctx context.Context, tenant, request string) error {
 processor.context, processor.tenant, processor.request = ctx, tenant, request
 *processor.clock = processor.clock.Add(processor.duration)
 if err := ctx.Err(); err != nil { return err }
 return processor.err
}
// TestAtlasBusinessMetricsBehavior checks updates rather than metric registration tokens.
func TestAtlasBusinessMetricsBehavior(t *testing.T) {
 manager := metrics.NewManagerWithConfig(metrics.Config{})
 registry := manager.Registry()
 clock := time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC)
 processor := &atlasMetricsProcessor{clock: &clock}
 service := NewService(registry, processor, func() time.Time { return clock })
 second := NewService(registry, processor, func() time.Time { return clock })
 var durations []time.Duration
 var total time.Duration
 check := func() {
  t.Helper()
  snapshot := registry.Snapshot()
  var counters, histograms int
  for _, counter := range snapshot.Counters {
   if counter.Descriptor.Name != "checkout.completed" { continue }
   counters++
   if counter.Value != uint64(len(durations)) { t.Fatalf("completed=%d want %d", counter.Value, len(durations)) }
   for _, label := range counter.Labels { atlasAssertMetricLabel(t, label.Key, label.Value) }
  }
  for _, histogram := range snapshot.Histograms {
   if histogram.Descriptor.Name != "checkout.duration" { continue }
   histograms++
   for _, label := range histogram.Labels { atlasAssertMetricLabel(t, label.Key, label.Value) }
   if histogram.Count != uint64(len(durations)) || math.Abs(histogram.Sum-float64(total)) > 0.5 { t.Fatalf("histogram count/sum=%d/%g want %d/%d", histogram.Count, histogram.Sum, len(durations), total) }
   if len(histogram.Bounds) == 0 { t.Fatal("duration histogram needs finite bounds") }
   buckets := make([]uint64, len(histogram.Bounds)+1)
   for _, duration := range durations { index := 0; for index < len(histogram.Bounds) && int64(duration) > histogram.Bounds[index] { index++ }; buckets[index]++ }
   if len(histogram.BucketCounts) != len(buckets) { t.Fatal("invalid duration bucket shape") }
   for index, want := range buckets { if histogram.BucketCounts[index] != want { t.Fatalf("bucket %d=%d want %d", index, histogram.BucketCounts[index], want) } }
  }
  if counters != 1 || histograms != 1 { t.Fatalf("business samples=%d/%d, want one bounded sample per family", counters, histograms) }
  atlasAssertPrometheus(t, registry, uint64(len(durations)), total)
 }
 check()
 ctx := context.WithValue(context.Background(), struct{}{}, "caller")
 for index, duration := range []time.Duration{25*time.Millisecond, 250*time.Millisecond, 1500*time.Millisecond} {
  processor.duration = duration
  tenant, request := fmt.Sprintf("tenant:%d", index), fmt.Sprintf("request:%d", index)
  if err := service.Complete(ctx, tenant, request); err != nil { t.Fatal(err) }
  if processor.context != ctx || processor.tenant != tenant || processor.request != request { t.Fatal("operation lost caller context or identity") }
  durations, total = append(durations, duration), total+duration
  check()
 }
 failure := errors.New("declined")
 processor.err, processor.duration = failure, 9*time.Second
 if err := service.Complete(ctx, "failed:tenant", "failed:request"); !errors.Is(err, failure) { t.Fatalf("operation error=%v", err) }
 check()
 canceled, cancel := context.WithCancel(ctx); cancel()
 if err := service.Complete(canceled, "canceled:tenant", "canceled:request"); !errors.Is(err, context.Canceled) { t.Fatalf("canceled operation=%v", err) }
 check()
 processor.err, processor.duration = nil, 25*time.Millisecond
 for index:=0; index<64; index++ {
  if err := second.Complete(ctx, fmt.Sprintf("unique:tenant:%d", index), fmt.Sprintf("unique:request:%d", index)); err != nil { t.Fatal(err) }
  durations, total = append(durations, processor.duration), total+processor.duration
 }
 check()
}
// atlasAssertMetricLabel accepts only the disclosed finite label policy.
func atlasAssertMetricLabel(t *testing.T, key, value string) {
 t.Helper()
 switch key {
 case "outcome": if value != "success" { t.Fatalf("unbounded outcome %q", value) }
 case "app": if value == "" || len(value)>32 { t.Fatalf("invalid static app label %q", value) }
 default: t.Fatalf("unbounded metric label %q=%q", key, value)
 }
}
`

// businessMetricsPrometheusProbe verifies real HTTP exposition values independently from native snapshots.
const businessMetricsPrometheusProbe = `// Package checkout owns instrumented checkout operations.
package checkout
import (
 "math"
 "net/http"
 "net/http/httptest"
 "strconv"
 "strings"
 "testing"
 "time"
 "github.com/goforj/metrics"
)
// atlasAssertPrometheus checks the emitted count and seconds conversion from the same registry.
func atlasAssertPrometheus(t *testing.T, registry *metrics.Registry, completed uint64, duration time.Duration) {
 t.Helper()
 recorder := httptest.NewRecorder()
 metrics.Handler(registry).ServeHTTP(recorder, httptest.NewRequest(http.MethodGet, "/metrics", nil))
 if recorder.Code != http.StatusOK || recorder.Header().Get("Content-Type") != metrics.PrometheusContentType { t.Fatalf("scrape=%d %s", recorder.Code, recorder.Header().Get("Content-Type")) }
 values := map[string]float64{}
 counts := map[string]int{}
 for _, line := range strings.Split(recorder.Body.String(), "\n") {
  for _, family := range []string{"checkout_completed_total", "checkout_duration_seconds_count", "checkout_duration_seconds_sum"} {
   if !strings.HasPrefix(line, family+" ") && !strings.HasPrefix(line, family+"{") { continue }
   index := strings.LastIndexByte(line, ' ')
   value, err := strconv.ParseFloat(line[index+1:], 64); if err != nil { t.Fatal(err) }
   values[family]+=value; counts[family]++
  }
 }
 for family, want := range map[string]float64{"checkout_completed_total":float64(completed), "checkout_duration_seconds_count":float64(completed), "checkout_duration_seconds_sum":duration.Seconds()} {
  if counts[family]!=1 || math.Abs(values[family]-want)>1e-9 { t.Fatalf("scrape %s count/value=%d/%g want 1/%g", family, counts[family], values[family], want) }
 }
}
`
