package eval

const incidentNotificationBehaviorProbe = `package incidents

import (
    "context"
    "errors"
    "testing"

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
`
