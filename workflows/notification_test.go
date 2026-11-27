package workflows

import (
	"context"
	"slices"
	"strings"
	"testing"

	"github.com/goforj/atlas/docs"
)

// TestNotificationPlanPreservesAcceptanceBoundaries keeps senders separate from generated jobs.
func TestNotificationPlanPreservesAcceptanceBoundaries(t *testing.T) {
	for _, task := range []string{"add incident alerts with primary and fallback senders", "compose notification senders"} {
		if Classify(task) != "goforj-notification-delivery" || !slices.Contains(ClassifyAll(task), "goforj-notification-delivery") {
			t.Fatalf("wrong workflow for %q", task)
		}
	}
	if len(workflowDocs("goforj-notification-delivery")) < 2 {
		t.Fatal("missing notification docs")
	}
	for _, command := range workflowCommands("goforj-notification-delivery", appScope{}) {
		if strings.Contains(command, "make:") {
			t.Fatalf("invented notification generator: %s", command)
		}
	}
	if !strings.Contains(strings.Join(workflowWarnings("goforj-notification-delivery"), " "), "unknown acceptance") {
		t.Fatal("missing acceptance warning")
	}
}

// TestNotificationResourcePlansSelectAppAccessors keeps framework wiring and sender acceptance policy explicit.
func TestNotificationResourcePlansSelectAppAccessors(t *testing.T) {
	ctx := fixtureContext()
	ctx.Project.Components = append(ctx.Project.Components, "notification")
	for _, app := range []string{"app", "marketplace"} {
		result, ok := Plan(ctx, PlanRequest{Task: "configure notification webhook senders", App: app})
		if !ok {
			t.Fatal("missing plan")
		}
		command := "forj generate --notification"
		wire := "app/wire/inject_services_app.go"
		if app != "app" {
			command = "forj " + app + " generate --notification"
			wire = "app/" + app + "/wire/inject_services_app.go"
		}
		if !slices.Contains(result.Commands, command) {
			t.Fatalf("commands = %v", result.Commands)
		}
		for _, path := range []string{".goforj.yml", ".env", "internal/notifications/manager_gen.go", "internal/notifications/accessors_gen.go", wire} {
			if !slices.Contains(result.Files, path) {
				t.Fatalf("missing %s", path)
			}
		}
		if slices.Contains(result.WorkflowIDs, "goforj-add-http-route") {
			t.Fatalf("outbound webhook invented an inbound route: %v", result.WorkflowIDs)
		}
		advice, ok := CommandAdvice(ctx, CommandAdviceRequest{Task: "configure notification senders", App: app})
		if !ok || advice.Command != command {
			t.Fatalf("advice = %#v", advice)
		}
	}
	for _, task := range []string{"discuss the notification API", "configure HTTP webhook notification sender"} {
		if slices.Contains(ClassifyAll(task), "goforj-add-http-route") {
			t.Fatalf("invented inbound HTTP for %q", task)
		}
	}
	ids := ClassifyAll("add HTTP endpoint to send incident alerts")
	if !slices.Contains(ids, "goforj-add-http-route") || !slices.Contains(ids, "goforj-notification-delivery") {
		t.Fatalf("explicit composition = %v", ids)
	}
}

// TestNotificationPlansRequireCLICapability avoids inventing a release threshold for optional generation.
func TestNotificationPlansRequireCLICapability(t *testing.T) {
	ctx := fixtureContext()
	result, ok := Plan(ctx, PlanRequest{Task: "configure notification sender", App: "marketplace"})
	if !ok || !slices.Contains(result.Commands, "forj marketplace generate --help") || slices.Contains(result.Commands, "forj marketplace generate --notification") {
		t.Fatalf("plan = %#v", result)
	}
	for _, step := range result.Steps {
		if step.Action == "forj marketplace generate --notification" {
			t.Fatal("unsupported generation step")
		}
	}
	advice, ok := CommandAdvice(ctx, CommandAdviceRequest{Task: "configure notification sender", App: "marketplace"})
	if !ok || advice.Command != "forj marketplace generate --help" {
		t.Fatalf("advice = %#v", advice)
	}
}

// TestNotificationDocsAndOwnershipUseResourceSources protects generated sender outputs and bounded documentation retrieval.
func TestNotificationDocsAndOwnershipUseResourceSources(t *testing.T) {
	provider := docs.StaticProvider{Docs: []docs.Document{{Path: "developer-tools/wiring-recipes.md", Content: "# Library Resources\n\n## Named Resource\n\nDeclare resources and generate sender accessors.\n"}}}
	pack, err := DocsSectionPack(context.Background(), provider, "", "configure notification sender", 100)
	if err != nil || len(pack.Sections) == 0 || !pack.Sections[0].Found {
		t.Fatalf("docs pack = %#v, %v", pack, err)
	}
	for _, path := range []string{"internal/notifications/manager_gen.go", "internal/notifications/accessors_gen.go"} {
		policy := FilePolicy(FilePolicyRequest{Path: path})
		if policy.Editable || !strings.Contains(policy.ChangeThrough, "generate --notification") {
			t.Fatalf("policy = %#v", policy)
		}
	}
}

// TestNotificationDocsRequestsRemainReadOnly keeps API explanations out of generated resource changes.
func TestNotificationDocsRequestsRemainReadOnly(t *testing.T) {
	ctx := fixtureContext()
	for _, task := range []string{"explain notification webhook API", "read docs for notification senders"} {
		result, ok := Plan(ctx, PlanRequest{Task: task, App: "marketplace"})
		if !ok || result.WorkflowID != "goforj-read-docs" || len(result.Commands) != 0 || len(result.Files) != 0 {
			t.Fatalf("docs plan = %#v", result)
		}
		if !docsContain(result.Docs, "developer-tools/wiring-recipes.md") {
			t.Fatalf("missing resource docs: %#v", result.Docs)
		}
		if steps := ValidationPlan(ctx, PlanRequest{Task: task, App: "marketplace"}).Steps; len(steps) != 0 {
			t.Fatalf("read-only validation = %v", steps)
		}
		advice, ok := CommandAdvice(ctx, CommandAdviceRequest{Task: task, App: "marketplace"})
		if !ok || advice.Command != "Atlas search-docs" {
			t.Fatalf("docs advice = %#v", advice)
		}
	}
}

// TestNotificationPlanningKeepsEmailOnExistingMailer covers the distinction between mail and sender composition.
func TestNotificationPlanningKeepsEmailOnExistingMailer(t *testing.T) {
	ctx := fixtureContext()
	for _, task := range []string{
		"add order email notifications using the existing mailer",
		"send email notification after checkout",
		"send email notifications for incident alerts using the existing mailer",
		"configure a fallback sender for email notifications",
		"send email-only incident alerts without a webhook",
	} {
		plan, ok := Plan(ctx, PlanRequest{Task: task})
		if !ok || plan.WorkflowID != "goforj-email-notifications" || !docsContain(plan.Docs, "applications/mail.md") {
			t.Fatalf("email-only plan for %q = %#v", task, plan)
		}
		if slices.Contains(plan.WorkflowIDs, "goforj-notification-delivery") || slices.Contains(plan.WorkflowIDs, "goforj-add-http-route") {
			t.Fatalf("email-only request selected unrelated workflows for %q: %v", task, plan.WorkflowIDs)
		}
	}
	for _, task := range []string{
		"configure a fallback sender for email and webhook notifications",
		"add notifications through email and webhook",
		"send Slack and email notifications",
		"add notifications through email and push",
		"configure notifications for email and mobile push",
		"add multi-channel email and Teams notifications",
		"enable the notification component for email notifications",
		"configure NOTIFICATION_DRIVER for email notifications",
		"use the Notification library for email incident alerts",
		"use notifications.Manager for incident email alerts",
	} {
		if !slices.Contains(ClassifyAll(task), "goforj-notification-delivery") {
			t.Fatalf("multichannel task %q lost notification guidance", task)
		}
	}
}
