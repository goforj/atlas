package eval

// taxRateAppRuntimeProbe observes real outbound calls from the App's configured domain client.
const taxRateAppRuntimeProbe = `package taxrates

import (
 "context"
 "time"
 "fmt"
 "net/http"
 "net/http/httptest"
 "os"
 "os/exec"
 "path/filepath"
 "strings"
 "sync/atomic"
 "testing"
)

// TestAtlasResourceApp requires the active App provider to route its client to the controlled tax service.
func TestAtlasResourceApp(t *testing.T) {
 root := atlasAppRoot(t)
 var calls, status atomic.Int32
 server := httptest.NewServer(http.HandlerFunc(func(response http.ResponseWriter,request *http.Request) {
  count := calls.Add(1)
  if request.Method != http.MethodGet || request.URL.EscapedPath() != "/rates/United%20States%2FWest" { t.Errorf("App request = %s %s",request.Method,request.URL.EscapedPath()) }
  response.Header().Set("Content-Type","application/json")
  if status.Load() != 0 { response.WriteHeader(int(status.Load())); return }
  if count < 3 { response.WriteHeader(http.StatusServiceUnavailable); return }
  fmt.Fprint(response,` + "`" + `{"country":"US","percent":8.75}` + "`" + `)
 }))
 defer server.Close()
 environment := []string{"TAX_RATE_BASE_URL="+server.URL}
 atlasBuildResourceApp(t,root,environment)
 if output,err := atlasRunResourceApp(t,root,environment); err != nil { t.Fatalf("App tax recovery failed: %v\n%s",err,output) }
 if calls.Load() != 3 { t.Fatalf("App recovery requests = %d, want 3",calls.Load()) }
 calls.Store(0); status.Store(http.StatusBadGateway)
 if output,err := atlasRunResourceApp(t,root,environment); err == nil { t.Fatalf("exhausted App request succeeded: %s",output) }
 if calls.Load() != 3 { t.Fatalf("App exhaustion requests = %d, want 3",calls.Load()) }
 calls.Store(0); status.Store(http.StatusUnauthorized)
 if output,err := atlasRunResourceApp(t,root,environment); err == nil { t.Fatalf("permanent App failure succeeded: %s",output) }
 if calls.Load() != 1 { t.Fatalf("App permanent-status requests = %d, want 1",calls.Load()) }
}
`

// taxRateAppRuntimeCommand obtains the configured client exclusively through App injection.
const taxRateAppRuntimeCommand = `package cmd

import (
 "context"
 "fmt"
 "example.com/invoiceeval/internal/taxrates"
)

// HelloWorldCmd consumes the submitted client through the existing command registration.
type HelloWorldCmd struct { client *taxrates.Client }

// Signature preserves the existing generated App command name.
func (*HelloWorldCmd) Signature() string { return ` + "`name:\"hello:world\" help:\"Exercise configured tax-rate client\"`" + ` }

// NewHelloWorldCmd requires the registered App provider rather than a manually constructed client.
func NewHelloWorldCmd(client *taxrates.Client) *HelloWorldCmd { return &HelloWorldCmd{client:client} }

// Run validates the configured remote operation without exposing transport internals to business callers.
func (command *HelloWorldCmd) Run() error {
 rate,err := command.client.Find(context.Background(),"United States/West")
 if err != nil { return err }
 if rate.Country != "US" || rate.Percent != 8.75 { return fmt.Errorf("App tax response changed") }
 return nil
}
`
