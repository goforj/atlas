package eval

// featureAppRuntimeProbe verifies the named Checkout resource through actual App startup.
const featureAppRuntimeProbe = `package checkout

import (
 "context"
 "time"
    "os"
    "os/exec"
    "path/filepath"
    "strings"
    "testing"
)

// TestAtlasResourceApp requires the App-owned service provider to resolve the native Checkout file resource.
func TestAtlasResourceApp(t *testing.T) {
    root := atlasAppRoot(t)
    path := filepath.Join(t.TempDir(), "checkout.json")
    document := []byte(` + "`" + `{"schema":1,"flags":{"checkout-v2":{"default":true},"receipt-v2":{"default":true}}}` + "`" + `)
    if err := os.WriteFile(path, document, 0600); err != nil { t.Fatal(err) }
    env := []string{"FEATURE_CHECKOUT_DRIVER=file", "FEATURE_CHECKOUT_PATH=" + path}
    atlasBuildResourceApp(t, root, env)
    output, err := atlasRunResourceApp(t, root, env)
    if err != nil { t.Fatalf("run feature App: %v\n%s", err, output) }
    if !strings.Contains(output, "atlas-feature-app:true:true") { t.Fatalf("App did not resolve Checkout flags: %s", output) }
}
`

// featureAppRuntimeCommand consumes the agent's service through the generated App's existing command registration.
const featureAppRuntimeCommand = `package cmd

import (
    "context"
    "fmt"

    "example.com/invoiceeval/internal/checkout"
)

// HelloWorldCmd exposes the supervised resource through the generated App command registry.
type HelloWorldCmd struct { service *checkout.Service }

// NewHelloWorldCmd requires Wire to resolve the agent's App-owned service provider.
func NewHelloWorldCmd(service *checkout.Service) *HelloWorldCmd { return &HelloWorldCmd{service: service} }

// Signature preserves the generated App's registered command identity.
func (*HelloWorldCmd) Signature() string { return ` + "`" + `name:"hello:world" help:"Resource runtime verification"` + "`" + ` }

// Run reads both flags from a request resolved by the real App resource graph.
func (command *HelloWorldCmd) Run() error {
    ctx := context.Background()
    request, err := command.service.Begin(ctx, "tenant:atlas")
    if err != nil { return err }
    fmt.Printf("atlas-feature-app:%t:%t\n", request.Checkout(ctx), request.Receipt(ctx))
    return nil
}
`
