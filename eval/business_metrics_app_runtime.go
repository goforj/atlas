package eval

// businessMetricsAppRuntimeProbe checks measurements through the generated App's active providers.
const businessMetricsAppRuntimeProbe = `package checkout
import (
 "context"
 "os"
 "os/exec"
 "path/filepath"
 "strings"
 "testing"
 "time"
)
// TestAtlasResourceApp requires the active Wire graph to resolve the service and its own registry.
func TestAtlasResourceApp(t *testing.T) {
 root := atlasAppRoot(t)
 atlasBuildResourceApp(t, root, nil)
 output, err := atlasRunResourceApp(t, root, nil)
 if err != nil { t.Fatalf("run checkout metrics App: %v\n%s", err, output) }
 if !strings.Contains(output, "atlas-checkout-metrics-ok") { t.Fatalf("missing App measurements: %s", output) }
}
`

// businessMetricsAppRuntimeCommand consumes both collaborators through the generated App graph.
const businessMetricsAppRuntimeCommand = `package cmd
import (
 "context"
 "fmt"
 "example.com/invoiceeval/internal/checkout"
 "example.com/invoiceeval/internal/metrics"
)
// HelloWorldCmd retains the existing command registration while requiring the submitted service.
type HelloWorldCmd struct { service *checkout.Service; manager *metrics.Manager }
// Signature preserves the existing generated command entry point.
func (*HelloWorldCmd) Signature() string { return ` + "`name:\"hello:world\" help:\"Observe checkout metrics\"`" + ` }
// NewHelloWorldCmd requires the active App providers to supply the service and manager.
func NewHelloWorldCmd(service *checkout.Service, manager *metrics.Manager) *HelloWorldCmd {
 return &HelloWorldCmd{service:service,manager:manager}
}
// Run verifies actual business observations in the registry borrowed by the App.
func (command *HelloWorldCmd) Run() error {
 for _, tenant := range []string{"tenant:app-a", "tenant:app-b"} {
  if err := command.service.Complete(context.Background(),tenant,"request:app"); err != nil { return err }
 }
 var completed, observed uint64
 for _, counter := range command.manager.Registry().Snapshot().Counters {
  if counter.Descriptor.Name == "checkout.completed" { completed += counter.Value }
 }
 for _, histogram := range command.manager.Registry().Snapshot().Histograms {
  if histogram.Descriptor.Name == "checkout.duration" { observed += histogram.Count }
 }
 if completed != 2 || observed != 2 { return fmt.Errorf("App measurements completed=%d duration-count=%d",completed,observed) }
 fmt.Println("atlas-checkout-metrics-ok")
 return nil
}
`
