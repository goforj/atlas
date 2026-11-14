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

// constrainNotificationPlan requests CLI capability evidence before proposing an unreleased component to an older Project.
func constrainNotificationPlan(result *PlanResult, p project.Project, scope appScope) {
	if !stringSliceContains(result.WorkflowIDs, "goforj-notification-delivery") || projectHasNotificationComponent(p) {
		return
	}
	generate := scope.command("generate --notification")
	inspect := scope.command("generate --help")
	for i, command := range result.Commands {
		if command == generate {
			result.Commands[i] = inspect
		}
	}
	result.Warnings = append(result.Warnings, "Project metadata does not declare the notification component. Inspect generate --help for --notification support before enabling it; use application-owned native composition if the CLI lacks that capability.")
	conditional := []PlanStep{
		{WorkflowID: "goforj-notification-delivery", Action: inspect, Purpose: "verify optional notification generation is supported by the selected CLI"},
		{WorkflowID: "goforj-notification-delivery", Action: "after confirming support, enable notification for the owning App and request a new workflow plan", Purpose: "keep unsupported component keys and commands out of older Projects"},
	}
	steps := make([]PlanStep, 0, len(result.Steps))
	inserted := false
	for _, step := range result.Steps {
		if step.WorkflowID == "goforj-notification-delivery" {
			if !inserted {
				steps = append(steps, conditional...)
				inserted = true
			}
			continue
		}
		steps = append(steps, step)
	}
	for i := range steps {
		steps[i].Order = i + 1
	}
	result.Steps = steps
	for i := range result.Segments {
		segment := &result.Segments[i]
		if segment.WorkflowID != "goforj-notification-delivery" {
			continue
		}
		for index, command := range segment.Commands {
			if command == generate {
				segment.Commands[index] = inspect
			}
		}
		segment.Steps = nil
		for _, step := range steps {
			if step.WorkflowID == segment.WorkflowID {
				segment.Steps = append(segment.Steps, step)
			}
		}
		segment.Warnings = append(segment.Warnings, "Inspect CLI support before enabling the optional notification component.")
	}
}

// isNotificationTask recognizes sender configuration without using generic message vocabulary.
func isNotificationTask(lower string) bool {
	return containsAny(lower, "notification", "incident alert", "fallback sender")
}

// notificationHTTPConcern requires an explicit inbound transport request rather than outbound webhook terminology.
func notificationHTTPConcern(lower string) bool {
	return containsAny(lower, "http endpoint", "api endpoint", "http route", "api route", "add a route", "add route", "controller", "http handler")
}

// init registers sender resource work independently from broad transport and persistence keywords.
func init() {
	registerApplicationWorkflow(applicationWorkflowDefinition{
		id: "goforj-notification-delivery", matches: isNotificationTask, docsTopic: isNotificationTask,
		commands: []string{"{forj} generate --notification", "{build}", "go test ./..."},
		docs:     WorkflowDocsMap["goforj-notification-delivery"],
		steps: []PlanStep{
			{Action: "enable notification for the owning App in .goforj.yml and declare NOTIFICATION_* resources in .env and its App overlay", Purpose: "keep configured sender membership scoped to its owner while inheriting root resources"},
			{Action: "{forj} generate --notification", Purpose: "generate the manager and typed sender accessors from the Project snapshot"},
			{Action: "inject notifications.Manager or its selected Sender into the App service; use App.Notifications() for App access", Purpose: "borrow framework-owned senders without creating hidden queues, retries, fallback, or fanout"},
			{Action: "test message data, caller cancellation, sender errors, and explicit primary/fallback acceptance policy using injected senders", Purpose: "avoid duplicate notifications when primary acceptance is uncertain"},
		},
		files:    []string{".goforj.yml", ".env", "internal/notifications/manager_gen.go", "internal/notifications/accessors_gen.go", "internal/<domain>/alert_service.go", "internal/<domain>/alert_service_test.go", "{wire}/inject_services_app.go", "{composition}/app.go"},
		warnings: []string{"Read goforj-notification-delivery before choosing notification, mail, events, or queue.", "Synchronous acceptance does not prove receipt or add retries; unknown acceptance must not automatically trigger fallback.", "Initial framework drivers are log and webhook. Keep fallback, fanout, and transport exposure explicit; App-prefixed generation uses the Project resource snapshot."},
	})
}
