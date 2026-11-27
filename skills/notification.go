package skills

const notificationDelivery = `# Notification delivery boundaries

Choose github.com/goforj/notification for portable multi-channel plain-text messages and sender composition. Choose mail for email composition and delivery, events for reactions to domain facts, and queue for durable deferred work. Keep adequate existing standard-library or explicit provider code when portability or composition is unnecessary. Do not replace mail merely because its messages are notifications.

Inspect the existing application boundary and the selected release API before editing. notification v0.1.0 exposes Sender.Send(context.Context, Message) error and SenderFunc. Message has Title and Body. For the opt-in notification framework component, the generated inject_managers.go constructs *notifications.Manager and owns its cleanup. Register an application provider in the owning App's inject_services_app.go. Wire injects the existing manager into that provider; select Default() or typed named accessors there and pass the selected notification.Sender values to the feature service. With declared Primary and Secondary senders, for example:

    // NewConfiguredAlertService borrows the App-managed senders.
    func NewConfiguredAlertService(notifications *notifications.Manager) *AlertService {
        return NewAlertService(notifications.Primary(), notifications.Secondary())
    }

Use app/wire/inject_services_app.go for the default App or app/<name>/wire/inject_services_app.go for a named App. Do not edit generated inject_managers.go, construct another manager, or inject the App as a service locator. App.Notifications() is for callers that already own the App, not the service injection path. Check the owning App in .goforj.yml. If adding notification, enable it there and run forj render from the Project root to install the App Wire providers. Configure NOTIFICATION_DRIVER and NOTIFICATION_SUPPORTED_DRIVERS; named senders use NOTIFICATION_<NAME>_DRIVER and existing App environment overlays. Run forj generate --notification and rebuild when changing the compiled manifest. Initial framework drivers are log and webhook, not every driver published by the standalone library. The manager owns constructed transports and closes them during initialization cleanup or shutdown. Do not reconstruct drivers inside feature services or put credentials in source. There is no make:notification generator. Keep business message content in the feature package and preserve caller context.

A synchronous nil error means local handling or provider acceptance, never proof of recipient receipt. New validates and sanitizes one sender call. Fallback(primary, secondary) tries the secondary only after permanent rejection, unauthorized credentials, or unsupported rendering. It stops on ErrOutcomeUnknown, unclassified failures, ErrRetryable, ErrThrottled, and cancellation. Do not retry or fall back automatically when the primary may already have accepted a message. Fanout intentionally sends to every target and is unsuitable as a primary/fallback substitute.

Neither Send nor Fallback starts background delivery or adds retries. Durable scheduling, retry policy, and idempotency belong to an explicit queue or application workflow. Use errors.Is for classified failures. Test primary success, definitive rejection, secondary failure, uncertain acceptance, cancellation, exact message data, and caller context with recording SenderFunc implementations. Do not send real notifications during tests.

Read internal/notifications/README.md for generated configuration and libraries/notification.md for the released sender API. Validate the owning package and App composition with go test and forj build. For a local App integration test, use the log driver or a webhook test server under your control.
`
