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
				{id: "admin-route", paths: []string{"internal/audits/*.go"}, identifiers: []string{"Controller", "Routes", "Index"}, selectorCalls: []string{"JSON"}, stringLiterals: []string{"/api/v1/audits"}},
				{id: "admin-registration", paths: []string{"app/admin/routes.go", "app/admin/wire/inject_http_controllers_app.go"}, identifiers: []string{"NewController", "Routes"}},
			},
			forbiddenText: []textExclusion{{id: "default-app-unchanged", paths: []string{"app/routes.go"}, text: "/api/v1/audits"}},
			commands:      append(standardSurfaceCommands(), commandContract{id: "admin-route-visible", arguments: []string{"forj", "admin", "route:list"}, contains: "/api/v1/audits"}),
		},
		{
			id:             "add-named-resource/v1",
			allowedChanges: []string{".env", "internal/queues/*_gen.go", "internal/invoices/report_dispatcher.go", "internal/invoices/report_dispatcher_test.go", "app/wire/inject_services_app.go"},
			sources: []sourceContract{
				{id: "named-queue-config", paths: []string{".env"}, text: []string{"QUEUE_REPORTS_NAME=reports", "QUEUE_REPORTS_WORKERS=2"}},
				{id: "named-queue-accessor", paths: []string{"internal/queues/*_gen.go"}, identifiers: []string{"Reports"}},
				{id: "named-queue-injection", paths: []string{"internal/invoices/report_dispatcher.go"}, identifiers: []string{"ReportDispatcher", "Manager"}, selectorCalls: []string{"Reports"}},
				{id: "named-queue-registration", paths: []string{"app/wire/inject_services_app.go"}, identifiers: []string{"NewReportDispatcher"}},
			},
			commands: standardSurfaceCommands(),
		},
		{
			id:             "add-named-cache/v1",
			allowedChanges: []string{".env", "internal/caches/*_gen.go", "internal/invoices/profile_cache.go", "internal/invoices/profile_cache_test.go", "app/wire/inject_services_app.go"},
			sources: []sourceContract{
				{id: "named-cache-config", paths: []string{".env"}, text: []string{"CACHE_PROFILES_DRIVER=memory"}},
				{id: "named-cache-accessor", paths: []string{"internal/caches/*_gen.go"}, identifiers: []string{"Profiles"}},
				{id: "named-cache-injection", paths: []string{"internal/invoices/profile_cache.go"}, identifiers: []string{"ProfileCache", "Manager"}, selectorCalls: []string{"Profiles"}},
				{id: "named-cache-registration", paths: []string{"app/wire/inject_services_app.go"}, identifiers: []string{"NewProfileCache"}},
			},
			commands: standardSurfaceCommands(),
		},
		{
			id:             "add-named-storage/v1",
			allowedChanges: []string{".env", "internal/storages/*_gen.go", "internal/invoices/avatar_storage.go", "internal/invoices/avatar_storage_test.go", "app/wire/inject_services_app.go"},
			sources: []sourceContract{
				{id: "named-storage-config", paths: []string{".env"}, text: []string{"STORAGE_AVATARS_DRIVER=local", "STORAGE_AVATARS_ROOT=storage/app/avatars"}},
				{id: "named-storage-accessor", paths: []string{"internal/storages/*_gen.go"}, identifiers: []string{"Avatars"}},
				{id: "named-storage-injection", paths: []string{"internal/invoices/avatar_storage.go"}, identifiers: []string{"AvatarStorage", "Manager"}, selectorCalls: []string{"Avatars"}},
				{id: "named-storage-registration", paths: []string{"app/wire/inject_services_app.go"}, identifiers: []string{"NewAvatarStorage"}},
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
	}
}

// standardSurfaceCommands proves the complete Project after source-level checks pass.
func standardSurfaceCommands(additional ...commandContract) []commandContract {
	commands := []commandContract{
		{id: "app-build", arguments: []string{"forj", "build"}},
		{id: "project-tests", arguments: []string{"go", "test", "./..."}},
	}
	return append(commands, additional...)
}
