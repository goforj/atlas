package eval

const incidentNotificationBehaviorProbe = `package incidents

import (
    "context"
    "errors"
    "encoding/json"
    "net/http"
    "net/http/httptest"
    "testing"

    "example.com/invoiceeval/internal/notifications"
    "github.com/goforj/notification"
)

// atlasIncidentContextKey distinguishes caller propagation from a replacement context.
type atlasIncidentContextKey struct{}

// TestAtlasIncidentNotificationBehavior records calls without sending real notifications.
func TestAtlasIncidentNotificationBehavior(t *testing.T) {
    tests := []struct {
        name string
        primary error
        fallback error
        canceled bool
        wantPrimary int
        wantFallback int
        wantError error
    }{
        {name: "accepted", wantPrimary: 1},
        {name: "permanent fallback", primary: notification.ErrPermanent, wantPrimary: 1, wantFallback: 1},
        {name: "unauthorized fallback", primary: notification.ErrUnauthorized, wantPrimary: 1, wantFallback: 1},
        {name: "render fallback", primary: notification.ErrRender, wantPrimary: 1, wantFallback: 1},
        {name: "both reject", primary: notification.ErrPermanent, fallback: notification.ErrUnauthorized, wantPrimary: 1, wantFallback: 1, wantError: notification.ErrUnauthorized},
        {name: "unknown acceptance", primary: notification.ErrOutcomeUnknown, wantPrimary: 1, wantError: notification.ErrOutcomeUnknown},
        {name: "unclassified acceptance", primary: errors.New("untrusted provider response"), wantPrimary: 1, wantError: notification.ErrOutcomeUnknown},
        {name: "retryable", primary: notification.ErrRetryable, wantPrimary: 1, wantError: notification.ErrRetryable},
        {name: "throttled", primary: notification.ErrThrottled, wantPrimary: 1, wantError: notification.ErrThrottled},
        {name: "canceled during send", primary: context.Canceled, wantPrimary: 1, wantError: context.Canceled},
        {name: "deadline", primary: context.DeadlineExceeded, wantPrimary: 1, wantError: context.DeadlineExceeded},
        {name: "already canceled", canceled: true, wantError: context.Canceled},
    }
    for _, test := range tests {
        t.Run(test.name, func(t *testing.T) {
            ctx, cancel := context.WithCancel(context.WithValue(context.Background(), atlasIncidentContextKey{}, "caller"))
            defer cancel()
            if test.canceled { cancel() }
            primaryCalls, fallbackCalls := 0, 0
            incident := Incident{ID: "inc-73", Severity: "critical", Summary: "Payments unavailable"}
            want := notification.Message{Title: "Incident inc-73", Body: "critical: Payments unavailable"}
            record := func(calls *int, failure error) notification.Sender {
                return notification.SenderFunc(func(got context.Context, message notification.Message) error {
                    *calls++
                    if got != ctx || got.Value(atlasIncidentContextKey{}) != "caller" { t.Error("caller context lost") }
                    if message != want { t.Errorf("message = %#v, want %#v", message, want) }
                    return failure
                })
            }
            err := NewAlertService(record(&primaryCalls, test.primary), record(&fallbackCalls, test.fallback)).Notify(ctx, incident)
            if test.wantError == nil && err != nil || test.wantError != nil && !errors.Is(err, test.wantError) {
                t.Fatalf("Notify() = %v, want classification %v", err, test.wantError)
            }
            if primaryCalls != test.wantPrimary || fallbackCalls != test.wantFallback {
                t.Fatalf("calls = %d/%d, want %d/%d", primaryCalls, fallbackCalls, test.wantPrimary, test.wantFallback)
            }
        })
    }
}

// TestAtlasConfiguredIncidentNotificationBehavior exercises generated sender selection through local webhook transports.
func TestAtlasConfiguredIncidentNotificationBehavior(t *testing.T) {
    for _, test := range []struct { name string; status int; uncertain bool; wantFallback int; wantError error }{
        {name: "primary accepted", status: http.StatusNoContent},
        {name: "primary rejected", status: http.StatusBadRequest, wantFallback: 1},
        {name: "primary transient", status: http.StatusServiceUnavailable, wantError: notification.ErrRetryable},
        {name: "accepted then connection lost", uncertain: true, wantError: notification.ErrOutcomeUnknown},
    } {
        t.Run(test.name, func(t *testing.T) {
            primaryCalls, fallbackCalls := 0, 0
            record := func(calls *int, primary bool) http.HandlerFunc {
                return func(response http.ResponseWriter, request *http.Request) {
                    *calls++
                    var payload notification.TextPayload
                    if err := json.NewDecoder(request.Body).Decode(&payload); err != nil { t.Error(err) }
                    if payload.Title != "Incident inc-104" || payload.Body != "warning: slow checkout" { t.Errorf("configured message = %#v", payload) }
                    if primary && test.uncertain {
                        connection, _, err := response.(http.Hijacker).Hijack()
                        if err != nil { t.Error(err); return }
                        _ = connection.Close()
                        return
                    }
                    status := http.StatusNoContent
                    if primary { status = test.status }
                    response.WriteHeader(status)
                }
            }
            primary := httptest.NewServer(record(&primaryCalls, true))
            defer primary.Close()
            fallback := httptest.NewServer(record(&fallbackCalls, false))
            defer fallback.Close()
            t.Setenv("NOTIFICATION_DRIVER", "log")
            t.Setenv("NOTIFICATION_PRIMARY_DRIVER", "webhook")
            t.Setenv("NOTIFICATION_PRIMARY_WEBHOOK_URL", primary.URL)
            t.Setenv("NOTIFICATION_PRIMARY_WEBHOOK_ALLOW_PRIVATE", "true")
            t.Setenv("NOTIFICATION_SECONDARY_DRIVER", "webhook")
            t.Setenv("NOTIFICATION_SECONDARY_WEBHOOK_URL", fallback.URL)
            t.Setenv("NOTIFICATION_SECONDARY_WEBHOOK_ALLOW_PRIVATE", "true")
            manager, err := notifications.NewManager()
            if err != nil { t.Fatal(err) }
            defer manager.Close()
            err = NewConfiguredAlertService(manager).Notify(context.Background(), Incident{ID: "inc-104", Severity: "warning", Summary: "slow checkout"})
            if test.wantError == nil && err != nil || test.wantError != nil && !errors.Is(err, test.wantError) { t.Fatalf("configured Notify = %v, want %v", err, test.wantError) }
            if primaryCalls != 1 || fallbackCalls != test.wantFallback { t.Fatalf("configured calls = %d/%d, want 1/%d", primaryCalls, fallbackCalls, test.wantFallback) }
        })
    }
}
`
