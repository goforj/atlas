package eval

// promotedSurfaceContracts returns reviewed contracts for framework surfaces that do not need a bespoke behavioral oracle.
func promotedSurfaceContracts() []surfaceContract {
	return []surfaceContract{
		{
			id:             "add-app-command/v1",
			allowedChanges: []string{"internal/invoices/*_cmd.go", "internal/invoices/*_cmd_test.go", "app/commands.go", "app/wire/inject_cmd_app.go"},
			sources: []sourceContract{
				{id: "command-shape", paths: []string{"internal/invoices/*_cmd.go"}, identifiers: []string{"ShowCmd", "Run", "Service"}, selectorCalls: []string{"Find"}, forbiddenCalls: []string{"Background", "TODO"}},
				{id: "command-registration", paths: []string{"app/commands.go", "app/wire/inject_cmd_app.go"}, identifiers: []string{"ShowCmd", "NewShowCmd"}},
			},
			commands: standardSurfaceCommands(commandContract{id: "command-behavior", arguments: []string{"forj", "invoices:show", "inv-42"}, contains: "12500"}),
		},
		{
			id:             "add-job/v1",
			allowedChanges: []string{"internal/invoices/*_job.go", "internal/invoices/*_job_test.go", "app/wire/inject_jobs_app.go"},
			sources: []sourceContract{
				{id: "typed-job", paths: []string{"internal/invoices/*_job.go"}, identifiers: []string{"ReceiptJob", "ReceiptJobPayload", "InvoiceID", "HandleTask", "Service"}, selectorCalls: []string{"Bind", "Find", "Dispatch"}, forbiddenCalls: []string{"Background", "TODO"}, stringLiterals: []string{"invoices:receipt"}},
				{id: "job-registration", paths: []string{"app/wire/inject_jobs_app.go"}, identifiers: []string{"NewReceiptJob", "ReceiptJobTypeName", "HandleTask"}},
			},
			commands: standardSurfaceCommands(),
		},
		{
			id:             "add-migration/v1",
			allowedChanges: []string{"migrations/*_add_status_to_invoices.up.sql", "migrations/*_add_status_to_invoices.down.sql"},
			sources: []sourceContract{
				{id: "migration-up", paths: []string{"migrations/*_add_status_to_invoices.up.sql"}, text: []string{"-- Up migration (sqlite)"}},
				{id: "migration-down", paths: []string{"migrations/*_add_status_to_invoices.down.sql"}, text: []string{"-- Down migration (sqlite)"}},
			},
			commands: standardSurfaceCommands(),
		},
		{
			id:             "add-schedule/v1",
			allowedChanges: []string{"internal/invoices/*_schedule.go", "internal/invoices/*_schedule_test.go", "app/wire/inject_schedules_app.go", "app/schedules.go"},
			sources: []sourceContract{
				{id: "schedule-shape", paths: []string{"internal/invoices/*_schedule.go"}, identifiers: []string{"ReconcileSchedule", "Interval", "Handle", "Service"}, selectorCalls: []string{"Find"}, forbiddenCalls: []string{"Background", "TODO"}, stringLiterals: []string{"invoices:reconcile"}},
				{id: "schedule-registration", paths: []string{"app/wire/inject_schedules_app.go", "app/schedules.go"}, identifiers: []string{"NewReconcileSchedule"}},
			},
			commands: standardSurfaceCommands(),
		},
		{
			id:             "add-event-subscriber/v1",
			allowedChanges: []string{"internal/invoices/*_event.go", "internal/invoices/*_subscriber.go", "internal/invoices/*_subscriber_test.go", "app/wire/inject_subscribers_app.go"},
			sources: []sourceContract{
				{id: "typed-event", paths: []string{"internal/invoices/*_event.go"}, identifiers: []string{"PaidEvent", "InvoiceID", "Topic"}, stringLiterals: []string{"invoices.paid"}},
				{id: "subscriber-boundary", paths: []string{"internal/invoices/*_subscriber.go"}, identifiers: []string{"PaidSubscriber", "Handle", "Service"}, selectorCalls: []string{"Find"}, forbiddenCalls: []string{"Background", "TODO"}},
				{id: "subscriber-registration", paths: []string{"app/wire/inject_subscribers_app.go"}, identifiers: []string{"NewPaidSubscriber", "Handle"}, selectorCalls: []string{"Named", "Subscribe"}},
			},
			commands: standardSurfaceCommands(),
		},
		{
			id:             "create-model/v1",
			allowedChanges: []string{"internal/models/*.go", "app/wire/inject_repositories_app.go"},
			sources: []sourceContract{
				{id: "model-shape", paths: []string{"internal/models/*.go"}, identifiers: []string{"User", "Email", "DisplayName", "CreatedAt", "UserRepo", "ByID", "WithContext"}, stringLiterals: []string{"users"}},
				{id: "repository-registration", paths: []string{"app/wire/inject_repositories_app.go"}, identifiers: []string{"NewUserRepo"}},
			},
			commands: standardSurfaceCommands(),
		},
		{
			id:             "add-named-app-route/v1",
			allowedChanges: []string{"internal/audits/*.go", "app/admin/routes.go", "app/admin/wire/inject_http_controllers_app.go"},
			sources: []sourceContract{
				{id: "admin-route", paths: []string{"internal/audits/*.go"}, identifiers: []string{"Controller", "Routes"}, selectorCalls: []string{"JSON", "NewRoute"}},
				{id: "admin-registration", paths: []string{"app/admin/routes.go", "app/admin/wire/inject_http_controllers_app.go"}, identifiers: []string{"NewController", "Routes"}},
			},
			forbiddenText: []textExclusion{{id: "default-app-unchanged", paths: []string{"app/routes.go"}, text: "/api/v1/audits"}},
			commands:      append(standardSurfaceCommands(), commandContract{id: "admin-route-visible", arguments: []string{"forj", "admin", "route:list"}, contains: "/api/v1/audits"}),
		},
		{
			id:             "add-named-resource/v1",
			allowedChanges: []string{".env", ".env.example", "internal/queues/*_gen.go", "internal/invoices/report_dispatcher.go", "internal/invoices/report_dispatcher_test.go", "internal/reports/*.go", "app/wire/inject_services_app.go"},
			sources: []sourceContract{
				{id: "named-queue-config", paths: []string{".env"}, text: []string{"QUEUE_REPORTS_NAME=reports", "QUEUE_REPORTS_WORKERS=2"}},
				{id: "named-queue-accessor", paths: []string{"internal/queues/*_gen.go"}, identifiers: []string{"Reports"}},
				{id: "named-queue-injection", paths: []string{"internal/invoices/report_dispatcher.go", "internal/reports/*.go"}, identifiers: []string{"Manager"}, identifierChoices: [][]string{{"ReportDispatcher", "Dispatcher"}}, selectorCalls: []string{"Reports"}},
				{id: "named-queue-registration", paths: []string{"app/wire/inject_services_app.go"}, identifierChoices: [][]string{{"NewReportDispatcher", "NewDispatcher"}}},
			},
			commands: standardSurfaceCommands(),
		},
		{
			id:             "add-named-cache/v1",
			allowedChanges: []string{".env", ".env.example", "internal/caches/*_gen.go", "internal/invoices/profile_cache.go", "internal/invoices/profile_cache_test.go", "internal/profiles/*.go", "app/wire/inject_services_app.go"},
			sources: []sourceContract{
				{id: "named-cache-config", paths: []string{".env"}, text: []string{"CACHE_PROFILES_DRIVER=memory"}},
				{id: "named-cache-accessor", paths: []string{"internal/caches/*_gen.go"}, identifiers: []string{"Profiles"}},
				{id: "named-cache-injection", paths: []string{"internal/invoices/profile_cache.go", "internal/profiles/*.go"}, identifiers: []string{"Manager"}, identifierChoices: [][]string{{"ProfileCache", "Cache"}}, selectorCalls: []string{"Profiles"}},
				{id: "named-cache-registration", paths: []string{"app/wire/inject_services_app.go"}, identifierChoices: [][]string{{"NewProfileCache", "NewCache"}}},
			},
			commands: standardSurfaceCommands(),
		},
		{
			id:             "add-named-storage/v1",
			allowedChanges: []string{".env", ".env.example", "internal/storages/*_gen.go", "internal/invoices/avatar_storage.go", "internal/invoices/avatar_storage_test.go", "internal/avatars/*.go", "app/wire/inject_services_app.go"},
			sources: []sourceContract{
				{id: "named-storage-config", paths: []string{".env"}, text: []string{"STORAGE_AVATARS_DRIVER=local", "STORAGE_AVATARS_ROOT=storage/app/avatars"}},
				{id: "named-storage-accessor", paths: []string{"internal/storages/*_gen.go"}, identifiers: []string{"Avatars"}},
				{id: "named-storage-injection", paths: []string{"internal/invoices/avatar_storage.go", "internal/avatars/*.go"}, identifiers: []string{"Manager"}, identifierChoices: [][]string{{"AvatarStorage", "Storage"}}, selectorCalls: []string{"Avatars"}},
				{id: "named-storage-registration", paths: []string{"app/wire/inject_services_app.go"}, identifierChoices: [][]string{{"NewAvatarStorage", "NewStorage"}}},
			},
			commands: standardSurfaceCommands(),
		},
		{
			id:             "repair-wire-provider/v1",
			allowedChanges: []string{"app/wire/inject_services_app.go"},
			sources: []sourceContract{
				{id: "provider-registration", paths: []string{"app/wire/inject_services_app.go"}, identifiers: []string{"NewService"}},
			},
			commands: append(standardSurfaceCommands(), commandContract{id: "report-route-visible", arguments: []string{"forj", "route:list"}, contains: "/api/v1/reports"}),
		},
		{
			id:             "build-json-api-feature/v1",
			allowedChanges: []string{"internal/users/*.go", "app/routes.go", "app/wire/inject_http_controllers_app.go", "app/wire/inject_services_app.go"},
			sources: []sourceContract{
				{id: "users-application-boundary", paths: []string{"internal/users/*.go"}, identifiers: []string{"User", "Controller", "Routes"}, identifierChoices: [][]string{{"Service", "UseCase", "Query"}, {"Find", "Get", "Lookup"}, {"Show", "Get"}}, selectorCalls: []string{"Param", "JSON", "NewRoute"}, forbiddenCalls: []string{"Background", "TODO"}, stringLiterals: []string{"/users/:id"}},
				{id: "users-registration", paths: []string{"app/routes.go", "app/wire/inject_http_controllers_app.go", "app/wire/inject_services_app.go"}, identifiers: []string{"NewController"}, identifierChoices: [][]string{{"NewService", "NewUseCase", "NewQuery"}}},
			},
			commands: append(standardSurfaceCommands(), commandContract{id: "users-route-visible", arguments: []string{"forj", "route:list"}, contains: "/api/v1/users/:id"}),
		},
		{
			id:             "create-additional-app/v1",
			allowedChanges: []string{".env", ".env.example", ".env.local", ".goforj.yml", "Dockerfile", "Makefile", "app/statuspage/**", "cmd/statuspage/**", "internal/runtime/apps.go", "internal/runtime/apps_test.go"},
			sources: []sourceContract{
				{id: "statuspage-project-config", paths: []string{".goforj.yml"}, text: []string{"statuspage:", "./bin/statuspage"}},
				{id: "statuspage-entrypoint", paths: []string{"cmd/statuspage/main.go"}, identifiers: []string{"main"}, selectorCalls: []string{"LaunchApplication"}},
				{id: "statuspage-app-boundary", paths: []string{"app/statuspage/routes.go", "app/statuspage/wire/*.go"}, identifiers: []string{"ProvideRoutes", "InitializeApplication"}},
			},
			commands: append(standardSurfaceCommands(), commandContract{id: "statuspage-build", arguments: []string{"forj", "statuspage", "build"}}),
		},
		{
			id:             "add-app-lifecycle-hook/v1",
			allowedChanges: []string{"app/lifecycle.go"},
			sources: []sourceContract{
				{id: "application-readiness-hook", paths: []string{"app/lifecycle.go"}, identifiers: []string{"LifecycleRegistry", "NewLifecycleRegistry", "Register", "BeforeStartup", "Service"}, selectorCalls: []string{"On", "Find"}, forbiddenCalls: []string{"Background", "TODO"}, text: []string{"runtime.BeforeStartup"}},
			},
			commands: standardSurfaceCommands(),
		},
		{
			id:             "add-outbound-http-integration/v1",
			allowedChanges: []string{"go.mod", "go.sum", "internal/taxrates/*.go", "app/wire/inject_services_app.go"},
			sources: []sourceContract{
				{id: "typed-http-client", paths: []string{"internal/taxrates/*.go"}, identifiers: []string{"Rate", "Client", "NewClient", "Find"}, forbiddenCalls: []string{"Background", "TODO"}, declarations: []declarationContract{{name: "Find", identifiers: []string{"ctx"}}}},
				{id: "http-client-provider", paths: []string{"app/wire/inject_services_app.go"}, identifiers: []string{"provideTaxRateClient", "NewClient"}, selectorCalls: []string{"Get"}},
			},
			commands: standardSurfaceCommands(),
		},
		{
			id:             "add-validated-write-endpoint/v1",
			allowedChanges: []string{"internal/invoices/*.go"},
			sources: []sourceContract{
				{id: "validated-request-boundary", paths: []string{"internal/invoices/controller.go"}, identifiers: []string{"createInvoiceRequest", "validationErrorResponse", "Store", "CreateInput", "MethodPost"}, selectorCalls: []string{"Bind", "Create", "JSON"}, forbiddenCalls: []string{"Background", "TODO"}, stringLiterals: []string{"/invoices", "invalid_payload", "validation_failed", "customer_id", "total_cents"}},
				{id: "invoice-create-behavior", paths: []string{"internal/invoices/repository.go", "internal/invoices/service.go"}, identifiers: []string{"Repository", "Create", "CreateInput"}, forbiddenCalls: []string{"Background", "TODO"}},
			},
			commands: append(standardSurfaceCommands(), commandContract{id: "invoice-write-route", arguments: []string{"forj", "route:list"}, contains: "/api/v1/invoices"}),
		},
		{
			id:             "add-route-middleware/v1",
			allowedChanges: []string{"internal/invoices/controller.go", "internal/invoices/middleware.go", "internal/invoices/middleware_test.go", "app/wire/inject_http_controllers_app.go"},
			sources: []sourceContract{
				{id: "invoice-token-policy", paths: []string{"internal/invoices/middleware.go", "internal/invoices/controller.go"}, identifiers: []string{"RequireToken"}, selectorCalls: []string{"NewRoute"}, forbiddenCalls: []string{"Background", "TODO"}, text: []string{"RequireToken(controller.token)"}, declarations: []declarationContract{{name: "RequireToken", selectorCalls: []string{"Request", "Get", "JSON"}, stringLiterals: []string{"X-Invoice-Token", "unauthorized"}}}},
				{id: "resolved-token-provider", paths: []string{"app/wire/inject_http_controllers_app.go"}, identifiers: []string{"provideInvoiceController"}, declarations: []declarationContract{{name: "provideInvoiceController", selectorCalls: []string{"Get", "NewController"}, stringLiterals: []string{"INVOICE_HTTP_TOKEN", ""}, forbiddenLiterals: []string{"invoice-secret"}}}},
			},
			commands: standardSurfaceCommands(commandContract{
				id:        "token-policy-behavior",
				arguments: []string{"go", "test", "./internal/invoices", "-run", "^TestAtlasRequireTokenBehavior$", "-count=1"},
				supervisorFiles: []supervisorFile{{
					path: "internal/invoices/atlas_eval_token_policy_test.go",
					body: tokenPolicyBehaviorProbe,
				}},
			}),
		},
		{
			id:             "add-database-transaction/v1",
			allowedChanges: []string{"go.mod", "go.sum", "internal/accounts/*.go", "app/wire/inject_services_app.go"},
			sources: []sourceContract{
				{id: "transaction-bound-repository", paths: []string{"internal/accounts/repository.go"}, identifiers: []string{"Repository", "WithTransaction", "AdjustBalance"}, selectorCalls: []string{"WithContext", "Transaction", "UpdateColumn"}, forbiddenCalls: []string{"Background", "TODO"}},
				{id: "atomic-transfer-service", paths: []string{"internal/accounts/service.go"}, identifiers: []string{"Service", "Transfer"}, forbiddenCalls: []string{"Background", "TODO"}, declarations: []declarationContract{{name: "Transfer", selectorCalls: []string{"WithTransaction", "AdjustBalance"}, nestedCalls: []nestedCallContract{{outer: "WithTransaction", inner: "AdjustBalance"}}}}},
				{id: "account-service-registration", paths: []string{"app/wire/inject_services_app.go"}, identifiers: []string{"NewRepository", "NewService"}},
			},
			commands: standardSurfaceCommands(),
		},
		{
			id:             "add-mail-workflow/v1",
			allowedChanges: []string{"internal/invoices/receipt_mailer.go", "internal/invoices/receipt_mailer_test.go", "app/wire/inject_services_app.go"},
			sources: []sourceContract{
				{id: "receipt-mail-service", paths: []string{"internal/invoices/receipt_mailer.go"}, identifiers: []string{"ReceiptMailer", "NewReceiptMailer", "Send", "Manager"}, forbiddenCalls: []string{"Background", "TODO"}, declarations: []declarationContract{{name: "Send", selectorCalls: []string{"Find", "Default", "Message", "To", "Subject", "Text", "Send"}}}},
				{id: "receipt-mail-registration", paths: []string{"app/wire/inject_services_app.go"}, identifiers: []string{"NewReceiptMailer"}},
			},
			forbiddenText: []textExclusion{{id: "no-provider-sdk", paths: []string{"internal/invoices/receipt_mailer.go"}, text: "smtp"}},
			commands:      standardSurfaceCommands(),
		},
		{
			id:             "protect-route-with-auth/v1",
			allowedChanges: []string{".env", ".env.example", ".env.local", "app/routes.go"},
			sources: []sourceContract{
				{id: "generated-auth-composition", paths: []string{"app/routes.go"}, identifiers: []string{"publicRoutes", "protectedRoutes", "invoicesController", "authService", "RequireAuth"}, selectorCalls: []string{"NewRouteGroup"}, assignments: []assignmentContract{{name: "publicRoutes", forbiddenIdentifiers: []string{"invoicesController"}}, {name: "protectedRoutes", identifiers: []string{"invoicesController"}, selectorCalls: []string{"Routes"}}}},
			},
			commands: append(standardSurfaceCommands(), commandContract{id: "protected-route-visible", arguments: []string{"forj", "route:list"}, contains: "/api/v1/invoices/:id"}),
		},
		{
			id:             "add-cached-repository/v1",
			allowedChanges: []string{".env", ".env.example", "internal/caches/*_gen.go", "internal/users/*.go", "app/wire/inject_services_app.go"},
			sources: []sourceContract{
				{id: "cache-aside-repository", paths: []string{"internal/users/repository.go", "internal/users/service.go"}, identifiers: []string{"UserRepository", "MemoryUserRepository", "CachedUserRepository", "Get", "Set"}, selectorCalls: []string{"WithContext", "Find"}, forbiddenCalls: []string{"Background", "TODO"}},
				{id: "profiles-cache-registration", paths: []string{"app/wire/inject_services_app.go"}, identifiers: []string{"NewCachedUserRepository"}, selectorCalls: []string{"Profiles"}},
			},
			commands: standardSurfaceCommands(),
		},
		{
			id:             "add-upload-workflow/v1",
			allowedChanges: []string{".env", ".env.example", "go.mod", "internal/storages/*_gen.go", "internal/uploads/*.go", "app/routes.go", "app/wire/inject_http_controllers_app.go", "app/wire/inject_services_app.go"},
			sources: []sourceContract{
				{id: "upload-boundary", paths: []string{"internal/uploads/service.go", "internal/uploads/controller.go"}, identifiers: []string{"StoreInput", "StoredUpload", "Service", "Controller", "Store", "Routes"}, selectorCalls: []string{"Bind", "Store", "NewRoute"}, forbiddenCalls: []string{"Background", "TODO"}, stringLiterals: []string{"/uploads"}},
				{id: "uploads-storage-registration", paths: []string{"app/wire/inject_services_app.go"}, identifiers: []string{"NewService"}, selectorCalls: []string{"Uploads"}},
			},
			commands: append(standardSurfaceCommands(), commandContract{id: "uploads-route-visible", arguments: []string{"forj", "route:list"}, contains: "/api/v1/uploads"}),
		},
		{
			id:             "publish-domain-event/v1",
			allowedChanges: []string{"internal/events/*.go", "internal/users/*.go", "internal/notifications/*.go", "app/routes.go", "app/lifecycle.go", "app/wire/inject_http_controllers_app.go", "app/wire/inject_services_app.go"},
			sources: []sourceContract{
				{id: "typed-user-event", paths: []string{"internal/events/*.go"}, identifiers: []string{"UserCreated", "UserID", "Topic"}, stringLiterals: []string{"users.created"}},
				{id: "domain-event-publication", paths: []string{"internal/users/events.go", "internal/users/service.go"}, identifiers: []string{"UserEvents", "UserEventPublisher", "PublishCreated"}, selectorCalls: []string{"Publish", "WithContext"}, forbiddenCalls: []string{"Background", "TODO"}},
				{id: "event-reaction", paths: []string{"internal/notifications/subscribers.go", "app/lifecycle.go"}, identifiers: []string{"Subscribers", "Register", "Startup"}, selectorCalls: []string{"Subscribe"}, forbiddenCalls: []string{"Background", "TODO"}},
			},
			commands: standardSurfaceCommands(),
		},
		{
			id:             "dispatch-event-followup-job/v1",
			allowedChanges: []string{".env", ".env.example", "internal/jobs/*.go", "internal/reports/*.go", "internal/notifications/*.go", "internal/storages/*_gen.go", "app/lifecycle.go", "app/wire/inject_jobs_app.go", "app/wire/inject_services_app.go"},
			sources: []sourceContract{
				{id: "typed-report-job", paths: []string{"internal/reports/service.go", "internal/reports/generate_job.go"}, identifiers: []string{"GeneratePayload", "GenerateJob", "GenerateJobTypeName", "HandleTask", "ReportQueue"}, selectorCalls: []string{"Dispatch", "Bind", "GenerateForUser"}, forbiddenCalls: []string{"Background", "TODO"}, stringLiterals: []string{"reports:generate"}, declarations: []declarationContract{{name: "GeneratePayload", identifiers: []string{"UserID"}, forbiddenIdentifiers: []string{"Email"}}, {name: "GenerateForUser", selectorCalls: []string{"Find"}}, {name: "HandleTask", selectorCalls: []string{"Bind", "GenerateForUser"}}}},
				{id: "event-job-boundary", paths: []string{"internal/notifications/service.go"}, identifiers: []string{"HandleUserCreated", "ReportQueue"}, selectorCalls: []string{"Queue"}, forbiddenCalls: []string{"Background", "TODO"}},
				{id: "report-job-registration", paths: []string{"app/wire/inject_jobs_app.go", "app/wire/inject_services_app.go"}, identifiers: []string{"NewGenerateJob", "NewService"}},
			},
			commands: standardSurfaceCommands(),
		},
		{
			id:             "schedule-existing-job/v1",
			allowedChanges: []string{"internal/reports/*.go", "internal/users/*.go", "app/wire/inject_schedules_app.go", "app/wire/inject_services_app.go"},
			sources: []sourceContract{
				{id: "scheduled-job-dispatch", paths: []string{"internal/reports/daily.go", "internal/reports/daily_schedule.go"}, identifiers: []string{"DailyTargetRepository", "DailyRunner", "DailySchedule", "Interval", "Handle"}, selectorCalls: []string{"ListDailyReportTargets", "Queue", "Run"}, forbiddenCalls: []string{"Background", "TODO"}, stringLiterals: []string{"reports:daily"}},
				{id: "daily-target-repository", paths: []string{"internal/users/*.go"}, identifiers: []string{"ListDailyReportTargets"}},
				{id: "daily-schedule-registration", paths: []string{"app/wire/inject_schedules_app.go", "app/wire/inject_services_app.go"}, identifiers: []string{"NewDailySchedule", "NewDailyRunner"}},
			},
			commands: standardSurfaceCommands(),
		},
	}
}

const tokenPolicyBehaviorProbe = `package invoices

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/goforj/web"
	"github.com/goforj/web/webtest"
)

// TestAtlasRequireTokenBehavior verifies rejected and accepted policy paths through a supervisor-owned oracle.
func TestAtlasRequireTokenBehavior(t *testing.T) {
	tests := []struct {
		name string
		expected string
		provided string
		want int
	}{
		{name: "unconfigured", want: http.StatusUnauthorized},
		{name: "missing", expected: "secret", want: http.StatusUnauthorized},
		{name: "accepted", expected: "secret", provided: "secret", want: http.StatusNoContent},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			request := httptest.NewRequest(http.MethodGet, "/invoices/inv-42", nil)
			request.Header.Set("X-Invoice-Token", test.provided)
			response := httptest.NewRecorder()
			context := webtest.NewContext(request, response, "/invoices/:id", nil)
			next := func(context web.Context) error { return context.NoContent(http.StatusNoContent) }
			if err := RequireToken(test.expected)(next)(context); err != nil {
				t.Fatalf("middleware: %v", err)
			}
			if response.Code != test.want {
				t.Fatalf("status = %d, want %d", response.Code, test.want)
			}
		})
	}
}
`

// standardSurfaceCommands proves the complete Project after source-level checks pass.
func standardSurfaceCommands(additional ...commandContract) []commandContract {
	commands := []commandContract{
		{id: "app-build", arguments: []string{"forj", "build"}},
		{id: "project-compile", arguments: []string{"go", "test", "./..."}},
	}
	return append(commands, additional...)
}
