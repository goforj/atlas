package workflows

import (
	"strings"

	"github.com/goforj/atlas/project"
)

// projectHasNotificationComponent treats declared component metadata as capability evidence without guessing a release threshold.
func projectHasNotificationComponent(p project.Project) bool {
	for _, component := range p.Components {
		if strings.EqualFold(strings.TrimSpace(component), "notification") {
			return true
		}
	}
	return false
}

// isNotificationTask selects sender composition when the request calls for a notification resource or a non-email channel.
func isNotificationTask(lower string) bool {
	if containsAny(lower, "notification resource", "notification component", "notification library", "notification_", "notifications.manager", "github.com/goforj/notification") {
		return true
	}
	if !mentionsAlertDelivery(lower) {
		return false
	}
	if containsAny(lower, "multi-channel", "multichannel", "multi channel") || !hasTaskWord(lower, "email", "mailer", "mail") {
		return true
	}
	if containsAny(lower, "email-only", "email only", "only email", "only by email") {
		return false
	}
	return hasTaskWord(lower, "webhook", "slack", "sms", "push", "discord", "pagerduty") || strings.Contains(lower, "microsoft teams")
}

// isEmailNotificationTask keeps email-only delivery on the existing App mailer.
func isEmailNotificationTask(lower string) bool {
	return mentionsAlertDelivery(lower) && hasTaskWord(lower, "email", "mailer", "mail") && !isNotificationTask(lower)
}

// mentionsAlertDelivery recognizes incident and sender terminology shared by mail and notification.
func mentionsAlertDelivery(lower string) bool {
	return hasTaskWord(lower, "notification", "notifications") || containsAny(lower, "incident alert", "fallback sender")
}

// notificationHTTPConcern requires an explicit inbound transport request rather than outbound webhook terminology.
func notificationHTTPConcern(lower string) bool {
	return containsAny(lower, "http endpoint", "api endpoint", "http route", "api route", "add a route", "add route", "controller", "http handler")
}

// init registers sender resource work independently from broad transport and persistence keywords.
func init() {
	registerApplicationWorkflow(applicationWorkflowDefinition{
		id: "goforj-email-notifications", matches: isEmailNotificationTask, docsTopic: isEmailNotificationTask,
		commands: []string{"{build}", "go test ./..."},
		docs: []DocReference{
			{Path: "applications/mail.md", Heading: "Send a Message"},
			{Path: "applications/mail.md", Heading: "Testing"},
		},
		steps: []PlanStep{
			{Action: "find the existing mailer and delivery service in the owning App", Purpose: "preserve the App's configured sender and delivery policy"},
			{Action: "send the email from the existing application boundary and verify it with a local mailer", Purpose: "exercise recipient, subject and body without contacting real recipients"},
		},
	})
	registerApplicationWorkflow(applicationWorkflowDefinition{
		id: "goforj-notification-delivery", matches: isNotificationTask, docsTopic: isNotificationTask,
		commands: []string{"{forj} generate --notification", "{build}", "go test ./..."},
		docs:     WorkflowDocsMap["goforj-notification-delivery"],
		steps: []PlanStep{
			{Action: "if notification is disabled for the selected App, enable it in .goforj.yml and run forj render from the Project root; then declare NOTIFICATION_* resources in .env and its App overlay", Purpose: "keep configured sender membership scoped to its owner while inheriting root resources"},
			{Action: "{forj} generate --notification", Purpose: "generate the manager and typed sender accessors from the Project snapshot"},
			{Action: "register the application provider in {wire}/inject_services_app.go, inject *notifications.Manager, and pass its selected Sender values to the service; leave generated manager construction and cleanup to the framework", Purpose: "borrow framework-owned senders without creating hidden queues, retries, fallback, or fanout"},
			{Action: "test message data, caller cancellation, sender errors, and explicit primary/fallback acceptance policy using injected senders", Purpose: "avoid duplicate notifications when primary acceptance is uncertain"},
		},
		files:    []string{".goforj.yml", ".env", "internal/notifications/manager_gen.go", "internal/notifications/accessors_gen.go", "internal/<domain>/alert_service.go", "internal/<domain>/alert_service_test.go", "{wire}/inject_services_app.go", "{composition}/app.go"},
		warnings: []string{"Read goforj-notification-delivery before choosing notification, mail, events, or queue.", "Synchronous acceptance does not prove receipt or add retries; unknown acceptance must not automatically trigger fallback.", "Initial framework drivers are log and webhook. Keep fallback, fanout, and transport exposure explicit; App-prefixed generation uses the Project resource snapshot."},
	})
}
