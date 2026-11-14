package eval

const taxRateResilienceBehaviorProbe = `package taxrates

import (
    "context"
    "encoding/json"
    "errors"
    "fmt"
    "net/http"
    "net/http/httptest"
    "sync/atomic"
    "testing"
    "time"
    "strings"

    "github.com/goforj/httpx/v2"
)

// TestAtlasTaxRateResilienceCounts observes policy through actual HTTP requests rather than retry configuration tokens.
func TestAtlasTaxRateResilienceCounts(t *testing.T) {
    for _, test := range []struct {
        name string
        status int
        failures int64
        malformed bool
        disconnect bool
        nilContext bool
        wantCalls int64
        wantError bool
    }{
        {name: "success", status: 200, wantCalls: 1},
        {name: "recover 502", status: 502, failures: 1, wantCalls: 2},
        {name: "nil caller context", status: 502, failures: 1, nilContext: true, wantCalls: 2},
        {name: "recover 503", status: 503, failures: 2, wantCalls: 3},
        {name: "exhaust 502", status: 502, failures: 99, wantCalls: 3, wantError: true},
        {name: "exhaust 503", status: 503, failures: 99, wantCalls: 3, wantError: true},
        {name: "400", status: 400, failures: 99, wantCalls: 1, wantError: true},
        {name: "401", status: 401, failures: 99, wantCalls: 1, wantError: true},
        {name: "404", status: 404, failures: 99, wantCalls: 1, wantError: true},
        {name: "409", status: 409, failures: 99, wantCalls: 1, wantError: true},
        {name: "429", status: 429, failures: 99, wantCalls: 1, wantError: true},
        {name: "500", status: 500, failures: 99, wantCalls: 1, wantError: true},
        {name: "504", status: 504, failures: 99, wantCalls: 1, wantError: true},
        {name: "malformed success", status: 200, malformed: true, wantCalls: 1, wantError: true},
        {name: "connection lost", disconnect: true, wantCalls: 1, wantError: true},
    } {
        t.Run(test.name, func(t *testing.T) {
            var calls atomic.Int64
            country := "United States/West"
            server := httptest.NewServer(http.HandlerFunc(func(response http.ResponseWriter, request *http.Request) {
                count := calls.Add(1)
                if request.Method != http.MethodGet || request.URL.EscapedPath() != "/rates/United%20States%2FWest" { t.Errorf("request = %s %s", request.Method, request.URL.EscapedPath()) }
                if test.disconnect {
                    connection, _, err := response.(http.Hijacker).Hijack()
                    if err != nil { t.Error(err); return }
                    _ = connection.Close()
                    return
                }
                response.Header().Set("Content-Type", "application/json")
                if count <= test.failures { response.WriteHeader(test.status); _, _ = fmt.Fprint(response, "upstream rejected"); return }
                if test.malformed { _, _ = fmt.Fprint(response, "{\"country\":"); return }
                _, _ = fmt.Fprint(response, "{\"country\":\"US\",\"percent\":8.75}")
            }))
            defer server.Close()
            ctx := context.Background()
            if test.nilContext { ctx = nil }
            rate, err := NewClient(server.URL).Find(ctx, country)
            if (err != nil) != test.wantError { t.Fatalf("Find() = %#v, %v", rate, err) }
            if !test.wantError && (rate.Country != "US" || rate.Percent != 8.75) { t.Fatalf("typed result = %#v", rate) }
            if test.wantError && !strings.Contains(err.Error(), country) { t.Fatalf("operation context lost: %v", err) }
            if test.wantError && test.status != 200 && !test.disconnect {
                var status *httpx.HTTPError
                if !errors.As(err, &status) || status.StatusCode != test.status { t.Fatalf("typed status error lost: %v", err) }
            }
            if test.malformed {
                var syntax *json.SyntaxError
                if !errors.As(err, &syntax) { t.Fatalf("typed decoding error lost: %v", err) }
            }
            if got := calls.Load(); got != test.wantCalls { t.Fatalf("requests = %d, want %d", got, test.wantCalls) }
        })
    }
}

// TestAtlasTaxRateResiliencePreCanceled prevents an expired caller from contacting the service.
func TestAtlasTaxRateResiliencePreCanceled(t *testing.T) {
    var calls atomic.Int64
    server := httptest.NewServer(http.HandlerFunc(func(response http.ResponseWriter, request *http.Request) { calls.Add(1) }))
    defer server.Close()
    ctx, cancel := context.WithCancel(context.Background())
    cancel()
    if _, err := NewClient(server.URL).Find(ctx, "US"); !errors.Is(err, context.Canceled) { t.Fatalf("cancellation = %v", err) }
    deadline, stop := context.WithDeadline(context.Background(), time.Now().Add(-time.Second))
    defer stop()
    if _, err := NewClient(server.URL).Find(deadline, "US"); !errors.Is(err, context.DeadlineExceeded) { t.Fatalf("deadline = %v", err) }
    if calls.Load() != 0 { t.Fatal("expired caller sent a request") }
}

// TestAtlasTaxRateResilienceInFlightCancel proves cancellation reaches a request already accepted by the server.
func TestAtlasTaxRateResilienceInFlightCancel(t *testing.T) { atlasAssertTaxClientCancellation(t, false) }

// TestAtlasTaxRateResilienceInFlightDeadline proves an active request respects its caller's deadline.
func TestAtlasTaxRateResilienceInFlightDeadline(t *testing.T) { atlasAssertTaxClientCancellation(t, true) }

// atlasAssertTaxClientCancellation bounds a broken client so calibration can report context loss without hanging cleanup.
func atlasAssertTaxClientCancellation(t *testing.T, deadline bool) {
    t.Helper()
    entered, release := make(chan struct{}, 1), make(chan struct{})
    var calls atomic.Int64
    server := httptest.NewServer(http.HandlerFunc(func(response http.ResponseWriter, request *http.Request) {
        calls.Add(1)
        select { case entered <- struct{}{}: default: }
        select { case <-request.Context().Done(): case <-release: }
    }))
    defer server.Close()
    defer close(release)
    ctx, cancel := context.WithCancel(context.Background())
    expected := context.Canceled
    if deadline { cancel(); ctx, cancel = context.WithTimeout(context.Background(), time.Second); expected = context.DeadlineExceeded }
    defer cancel()
    result := make(chan error, 1)
    go func() { _, err := NewClient(server.URL).Find(ctx, "US"); result <- err }()
    select { case <-entered: case <-time.After(3*time.Second): t.Fatal("request never reached server") }
    if !deadline { cancel() }
    select {
    case err := <-result:
        if !errors.Is(err, expected) { t.Fatalf("context cause = %v, want %v", err, expected) }
    case <-time.After(3*time.Second): t.Fatal("request ignored caller context")
    }
    if calls.Load() != 1 { t.Fatalf("canceled request retried %d times", calls.Load()) }
}
`
