package eval

// notificationAppRuntimeProbe exercises configured transports through the generated App's actual Wire graph.
const notificationAppRuntimeProbe = `package incidents

import (
 "context"
 "time"
    "encoding/json"
    "net/http"
    "net/http/httptest"
    "os"
    "os/exec"
    "path/filepath"
    "strings"
    "sync/atomic"
    "testing"
)

// TestAtlasResourceApp proves the generated App resolves the configured service and routes real webhook sends.
func TestAtlasResourceApp(t *testing.T) {
    root := atlasAppRoot(t)
    var primaryCalls, secondaryCalls atomic.Int32
    var primaryStatus atomic.Int32
    primaryStatus.Store(http.StatusNoContent)
    record := func(calls *atomic.Int32, primary bool) http.HandlerFunc {
        return func(response http.ResponseWriter, request *http.Request) {
            calls.Add(1)
            var payload struct { Title string; Body string }
            if err := json.NewDecoder(request.Body).Decode(&payload); err != nil { t.Error(err) }
            if payload.Title != "Incident inc-app-29" || payload.Body != "critical: App checkout unavailable" {
                t.Errorf("App webhook payload = %#v", payload)
            }
            status := http.StatusNoContent
            if primary { status = int(primaryStatus.Load()) }
            response.WriteHeader(status)
        }
    }
    primary := httptest.NewServer(record(&primaryCalls, true))
    defer primary.Close()
    secondary := httptest.NewServer(record(&secondaryCalls, false))
    defer secondary.Close()
    environment := []string{
        "NOTIFICATION_DRIVER=log",
        "NOTIFICATION_PRIMARY_DRIVER=webhook",
        "NOTIFICATION_PRIMARY_WEBHOOK_URL=" + primary.URL,
        "NOTIFICATION_PRIMARY_WEBHOOK_ALLOW_PRIVATE=true",
        "NOTIFICATION_SECONDARY_DRIVER=webhook",
        "NOTIFICATION_SECONDARY_WEBHOOK_URL=" + secondary.URL,
        "NOTIFICATION_SECONDARY_WEBHOOK_ALLOW_PRIVATE=true",
    }
    atlasBuildResourceApp(t, root, environment)
    if output, err := atlasRunResourceApp(t, root, environment); err != nil {
        t.Fatalf("run accepted notification App: %v\n%s", err, output)
    }
    if primaryCalls.Load() != 1 || secondaryCalls.Load() != 0 {
        t.Fatalf("accepted App calls = %d/%d, want 1/0", primaryCalls.Load(), secondaryCalls.Load())
    }
    primaryStatus.Store(http.StatusUnauthorized)
    if output, err := atlasRunResourceApp(t, root, environment); err != nil {
        t.Fatalf("run fallback notification App: %v\n%s", err, output)
    }
    if primaryCalls.Load() != 2 || secondaryCalls.Load() != 1 {
        t.Fatalf("fallback App calls = %d/%d, want 2/1", primaryCalls.Load(), secondaryCalls.Load())
    }
    primaryStatus.Store(http.StatusInternalServerError)
    if output, err := atlasRunResourceApp(t, root, environment); err == nil {
        t.Fatalf("retryable primary failure must remain an App error: %s", output)
    }
    if primaryCalls.Load() != 3 || secondaryCalls.Load() != 1 {
        t.Fatalf("retryable App calls = %d/%d, want 3/1", primaryCalls.Load(), secondaryCalls.Load())
    }
}
`

// notificationAppRuntimeCommand consumes the candidate's service solely through App constructor injection.
const notificationAppRuntimeCommand = `package cmd

import (
    "context"

    "example.com/invoiceeval/internal/incidents"
)

// HelloWorldCmd exposes the supervisor's synthetic incident through the existing registered command.
type HelloWorldCmd struct {
    alerts *incidents.AlertService
}

// Signature preserves the generated command registration contract.
func (*HelloWorldCmd) Signature() string {
    return ` + "`name:\"hello:world\" help:\"Exercise configured incident notifications\"`" + `
}

// NewHelloWorldCmd requires Wire to resolve the candidate's configured AlertService provider.
func NewHelloWorldCmd(alerts *incidents.AlertService) *HelloWorldCmd {
    return &HelloWorldCmd{alerts: alerts}
}

// Run sends one synthetic incident through the App-owned service.
func (command *HelloWorldCmd) Run() error {
    return command.alerts.Notify(context.Background(), incidents.Incident{ID: "inc-app-29", Severity: "critical", Summary: "App checkout unavailable"})
}
`
