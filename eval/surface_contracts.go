package eval

// promotedSurfaceContracts returns reviewed contracts for framework surfaces that do not need a bespoke behavioral oracle.
func promotedSurfaceContracts() []surfaceContract {
	return []surfaceContract{
		{
			id:             "add-app-command/v1",
			allowedChanges: []string{"internal/invoices/*_cmd.go", "internal/invoices/*_cmd_test.go", "app/commands.go", "app/wire/inject_cmd_app.go"},
			sources: []sourceContract{
				{id: "command-shape", paths: []string{"internal/invoices/*_cmd.go"}, identifiers: []string{"ShowCmd", "Service"}, forbiddenCalls: []string{"TODO"}, declarations: []declarationContract{{name: "Run", receiver: "ShowCmd", identifiers: []string{"ctx"}, selectorCalls: []string{"Find"}, forbiddenCalls: []string{"Background"}}}},
				{id: "command-registration", paths: []string{"app/commands.go", "app/wire/inject_cmd_app.go"}, identifiers: []string{"ShowCmd", "NewShowCmd"}},
			},
			commands: standardSurfaceCommands(
				commandContract{id: "command-behavior-primary", arguments: []string{"forj", "invoices:show", "inv-42"}, contains: []string{"inv-42", "12500"}},
				commandContract{id: "command-behavior-variable", arguments: []string{"forj", "invoices:show", "inv-99"}, contains: []string{"inv-99", "9900"}},
			),
		},
		{
			id:             "add-job/v1",
			allowedChanges: []string{"internal/invoices/*_job.go", "internal/invoices/*_job_test.go", "app/wire/inject_jobs_app.go"},
			sources: []sourceContract{
				{id: "typed-job", paths: []string{"internal/invoices/*_job.go"}, identifiers: []string{"ReceiptJob", "ReceiptJobPayload", "Service"}, forbiddenCalls: []string{"TODO"}, stringLiterals: []string{"invoices:receipt"}, declarations: []declarationContract{{name: "ReceiptJobPayload", identifiers: []string{"InvoiceID"}}, {name: "Queue", receiver: "ReceiptJob", identifiers: []string{"ctx"}, selectorCalls: []string{"Dispatch"}, forbiddenCalls: []string{"Background"}}, {name: "HandleTask", receiver: "ReceiptJob", identifiers: []string{"ctx"}, selectorCalls: []string{"Bind"}, forbiddenCalls: []string{"Background"}}}},
				{id: "job-registration", paths: []string{"app/wire/inject_jobs_app.go"}, identifiers: []string{"NewReceiptJob", "ReceiptJobTypeName", "HandleTask"}},
			},
			commands: standardSurfaceCommands(commandContract{
				id:        "receipt-job-behavior",
				arguments: []string{"go", "test", "./internal/invoices", "-run", "^TestAtlasReceiptJobBehavior$", "-count=1"},
				supervisorFiles: []supervisorFile{{
					path: "internal/invoices/atlas_eval_receipt_job_test.go",
					body: receiptJobBehaviorProbe,
				}},
			}),
		},
		{
			id:             "add-migration/v1",
			allowedChanges: []string{"migrations/*_add_status_to_invoices.up.sql", "migrations/*_add_status_to_invoices.down.sql"},
			sources: []sourceContract{
				{id: "migration-up", paths: []string{"migrations/*_add_status_to_invoices.up.sql"}, sqlColumnChanges: []sqlColumnChangeContract{{table: "invoices", column: "status", add: true}}},
				{id: "migration-down", paths: []string{"migrations/*_add_status_to_invoices.down.sql"}, sqlColumnChanges: []sqlColumnChangeContract{{table: "invoices", column: "status"}}},
			},
			commands: standardSurfaceCommands(),
		},
		{
			id:             "add-schedule/v1",
			allowedChanges: []string{"internal/*/*.go", "app/*_schedule.go", "app/wire/*schedule*.go", "app/schedules.go"},
			sources: []sourceContract{
				{id: "schedule-shape", paths: []string{"internal/*/*.go", "app/*_schedule.go"}, identifiers: []string{"Interval", "Service"}, forbiddenCalls: []string{"TODO"}, stringLiterals: []string{"invoices:reconcile"}, declarations: []declarationContract{{name: "Handle", identifiers: []string{"ctx"}, selectorCalls: []string{"Find"}, forbiddenCalls: []string{"Background"}}}},
				{id: "schedule-registration", paths: []string{"internal/*/*.go", "app/*.go", "app/wire/*.go"}, scheduleRegistration: true},
			},
			commands: standardSurfaceCommands(commandContract{
				id:    "reconcile-schedule-behavior",
				probe: runReconcileScheduleBehaviorProbe,
			}),
		},
		{
			id:                  "add-event-subscriber/v1",
			allowedChanges:      []string{"internal/invoices/*_event.go", "internal/invoices/*_subscriber.go", "app/wire/inject_subscribers_app.go"},
			qualityTestPatterns: []string{"internal/invoices/*_event_test.go", "internal/invoices/*_subscriber_test.go"},
			sources: []sourceContract{
				{id: "typed-event", paths: []string{"internal/invoices/*_event.go"}, identifiers: []string{"PaidEvent", "InvoiceID", "Topic"}, stringLiterals: []string{"invoices.paid"}},
				{id: "subscriber-boundary", paths: []string{"internal/invoices/*_subscriber.go"}, identifiers: []string{"PaidSubscriber", "Service"}, forbiddenCalls: []string{"TODO"}, declarations: []declarationContract{{name: "Handle", receiver: "PaidSubscriber", identifiers: []string{"ctx", "InvoiceID"}, selectorCalls: []string{"Find"}, forbiddenCalls: []string{"Background"}}}},
				{id: "subscriber-registration", paths: []string{"app/wire/inject_subscribers_app.go"}, identifiers: []string{"NewPaidSubscriber", "Handle"}, selectorCalls: []string{"Subscribe"}},
			},
			commands: standardSurfaceCommands(commandContract{
				id:        "paid-subscriber-behavior",
				arguments: []string{"go", "test", "./internal/invoices", "-run", "^TestAtlasPaidSubscriberBehavior$", "-count=1"},
				supervisorFiles: []supervisorFile{{
					path: "internal/invoices/atlas_eval_paid_subscriber_test.go",
					body: paidSubscriberBehaviorProbe,
				}},
			}),
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
			id:                  "model-relationships/v1",
			allowedChanges:      []string{".db-relationships.yaml", "internal/content/*.go", "internal/models/*.go", "app/wire/inject_repositories_app.go"},
			qualityTestPatterns: []string{"internal/content/*_test.go", "internal/models/*_test.go"},
			sources: []sourceContract{
				{id: "relationship-contract", paths: []string{".db-relationships.yaml"}, normalizedText: []string{"users:", "1-many id -> posts:user_id"}},
				{id: "related-model-shape", paths: []string{"internal/content/*.go", "internal/models/*.go"}, identifiers: []string{"User", "Post", "Posts", "UserRepo", "PostRepo", "Relationships", "WithContext"}, stringLiterals: []string{"Posts"}},
				{id: "related-repository-registration", paths: []string{"app/wire/inject_repositories_app.go"}, identifiers: []string{"NewUserRepo", "NewPostRepo"}},
			},
			commands: standardSurfaceCommands(),
		},
		{
			id:             "add-named-app-route/v1",
			allowedChanges: []string{"internal/audit/*.go", "internal/audits/*.go", "app/admin/routes.go", "app/admin/wire/inject_http_controllers_app.go"},
			sources: []sourceContract{
				{id: "admin-route", paths: []string{"internal/audit/*.go", "internal/audits/*.go"}, identifiers: []string{"Controller"}, declarations: []declarationContract{{name: "Routes", receiver: "Controller", selectorCalls: []string{"NewRoute"}}, {nameChoices: []string{"Get", "Show", "Index"}, receiver: "Controller", selectorCalls: []string{"JSON"}}}},
				{id: "admin-registration", paths: []string{"app/admin/routes.go", "app/admin/wire/inject_http_controllers_app.go"}, identifiers: []string{"NewController", "Routes"}},
			},
			forbiddenText: []textExclusion{{id: "default-app-unchanged", paths: []string{"app/routes.go"}, text: "/api/v1/audits"}},
			commands:      append(standardSurfaceCommands(), commandContract{id: "admin-route-visible", arguments: []string{"forj", "admin", "route:list"}, contains: []string{"/api/v1/audits"}}),
		},
		{
			id:              "add-named-resource/v1",
			allowedChanges:  generatedEnvironmentChanges("internal/queues/*_gen.go", "app/*.go", "app/wire/inject_services_app.go"),
			requiredChanges: []string{"app/wire/inject_services_app.go"},
			sources: []sourceContract{
				{id: "named-queue-config", paths: []string{".env"}, text: []string{"QUEUE_REPORTS_NAME=reports", "QUEUE_REPORTS_WORKERS=2"}},
				{id: "named-queue-accessor", paths: []string{"internal/queues/*_gen.go"}, identifiers: []string{"Reports"}},
				{id: "named-queue-injection", paths: []string{"internal/*/*.go", "app/*.go"}, providerConnection: &providerConnectionContract{accessor: "Reports", managerImportSuffix: "/internal/queues", wirePaths: []string{"app/wire/inject_services_app.go"}}},
			},
			commands: standardSurfaceCommands(commandContract{id: "named-queue-behavior", arguments: []string{"go", "test", "./internal/invoices", "-run", "^TestAtlasNamedQueueBehavior$", "-count=1"}, supervisorFiles: []supervisorFile{{path: "internal/invoices/atlas_eval_named_queue_test.go", body: namedQueueBehaviorProbe}}, namedResourceProbe: &providerConnectionContract{accessor: "Reports", managerImportSuffix: "/internal/queues", wirePaths: []string{"app/wire/inject_services_app.go"}}}),
		},
		{
			id:              "add-named-cache/v1",
			allowedChanges:  generatedEnvironmentChanges("internal/caches/*_gen.go", "app/*.go", "app/wire/inject_services_app.go"),
			requiredChanges: []string{"app/wire/inject_services_app.go"},
			sources: []sourceContract{
				{id: "named-cache-config", paths: []string{".env"}, text: []string{"CACHE_PROFILES_DRIVER=memory"}},
				{id: "named-cache-accessor", paths: []string{"internal/caches/*_gen.go"}, identifiers: []string{"Profiles"}},
				{id: "named-cache-injection", paths: []string{"internal/*/*.go", "app/*.go"}, providerConnection: &providerConnectionContract{accessor: "Profiles", managerImportSuffix: "/internal/caches", wirePaths: []string{"app/wire/inject_services_app.go"}}},
			},
			commands: standardSurfaceCommands(commandContract{id: "named-cache-behavior", arguments: []string{"go", "test", "./internal/invoices", "-run", "^TestAtlasNamedCacheBehavior$", "-count=1"}, supervisorFiles: []supervisorFile{{path: "internal/invoices/atlas_eval_named_cache_test.go", body: namedCacheBehaviorProbe}}, namedResourceProbe: &providerConnectionContract{accessor: "Profiles", managerImportSuffix: "/internal/caches", wirePaths: []string{"app/wire/inject_services_app.go"}}}),
		},
		{
			id:              "add-named-storage/v1",
			allowedChanges:  generatedEnvironmentChanges("internal/storages/*_gen.go", "app/*.go", "app/wire/inject_services_app.go"),
			requiredChanges: []string{"app/wire/inject_services_app.go"},
			sources: []sourceContract{
				{id: "named-storage-config", paths: []string{".env"}, text: []string{"STORAGE_AVATARS_DRIVER=local", "STORAGE_AVATARS_ROOT=storage/app/avatars"}},
				{id: "named-storage-accessor", paths: []string{"internal/storages/*_gen.go"}, identifiers: []string{"Avatars"}},
				{id: "named-storage-injection", paths: []string{"internal/*/*.go", "app/*.go"}, providerConnection: &providerConnectionContract{accessor: "Avatars", managerImportSuffix: "/internal/storages", wirePaths: []string{"app/wire/inject_services_app.go"}}},
			},
			commands: standardSurfaceCommands(commandContract{id: "named-storage-behavior", arguments: []string{"go", "test", "./internal/invoices", "-run", "^TestAtlasNamedStorageBehavior$", "-count=1"}, supervisorFiles: []supervisorFile{{path: "internal/invoices/atlas_eval_named_storage_test.go", body: namedStorageBehaviorProbe}}, namedResourceProbe: &providerConnectionContract{accessor: "Avatars", managerImportSuffix: "/internal/storages", wirePaths: []string{"app/wire/inject_services_app.go"}}}),
		},
		{
			id:                  "choose-storage-for-files/v1",
			allowedChanges:      generatedEnvironmentChanges("go.mod", "internal/storages/*_gen.go", "internal/invoices/*attachment*.go", "app/wire/inject_services_app.go", "app/wire/app.go"),
			qualityTestPatterns: []string{"internal/invoices/*attachment*_test.go"},
			sources: []sourceContract{
				{id: "attachment-storage-config", paths: []string{".env"}, text: []string{"STORAGE_ATTACHMENTS_DRIVER=", "STORAGE_ATTACHMENTS_ROOT="}},
				{id: "attachment-storage-accessor", paths: []string{"internal/storages/*_gen.go"}, identifiers: []string{"Attachments"}},
				{id: "attachment-service-boundary", paths: []string{"internal/invoices/*attachment*.go"}, identifiers: []string{"Attachment", "AttachmentService"}, selectorCalls: []string{"Attachments"}, forbiddenCalls: []string{"Background", "WriteFile", "ReadFile"}, declarations: []declarationContract{
					{name: "NewAttachmentService", identifiers: []string{"Manager"}},
					{name: "Store", receiver: "AttachmentService", identifiers: []string{"ctx"}, selectorCalls: []string{"WithContext", "Put"}, forbiddenCalls: []string{"Background", "WriteFile"}},
					{name: "Read", receiver: "AttachmentService", identifiers: []string{"ctx"}, selectorCalls: []string{"WithContext", "Get"}, forbiddenCalls: []string{"Background", "ReadFile"}},
				}},
				{id: "attachment-service-registration", paths: []string{"app/wire/inject_services_app.go", "app/wire/app.go"}, identifiers: []string{"NewAttachmentService"}},
			},
			commands: standardSurfaceCommands(commandContract{
				id:        "attachment-storage-behavior",
				arguments: []string{"go", "test", "./internal/invoices", "-run", "^TestAtlasAttachmentStorageBehavior$", "-count=1"},
				supervisorFiles: []supervisorFile{{
					path: "internal/invoices/atlas_eval_attachment_storage_test.go",
					body: attachmentStorageBehaviorProbe,
				}},
			}),
		},
		{
			id:                  "serve-cacheable-image/v1",
			allowedChanges:      []string{"internal/avatars/*.go", "app/routes.go", "app/wire/inject_http_controllers_app.go", "app/wire/inject_services_app.go"},
			qualityTestPatterns: []string{"internal/avatars/*_test.go"},
			sources: []sourceContract{
				{id: "avatar-storage-boundary", paths: []string{"internal/avatars/*.go"}, identifiers: []string{"Image", "Service", "Digest"}, forbiddenCalls: []string{"Background", "ReadFile"}, declarations: []declarationContract{{name: "Find", receiver: "Service", identifiers: []string{"ctx"}, selectorCalls: []string{"WithContext", "Get"}, forbiddenCalls: []string{"Background", "ReadFile"}}}},
				{id: "avatar-revalidation", paths: []string{"internal/avatars/*.go"}, identifiers: []string{"Controller"}, declarations: []declarationContract{{name: "Show", receiver: "Controller", selectorCalls: []string{"Find", "SetHeader", "NoContent", "Blob"}, stringLiterals: []string{"Cache-Control", "ETag", "If-None-Match"}}}},
				{id: "avatar-route-registration", paths: []string{"internal/avatars/controller.go", "app/routes.go", "app/wire/inject_http_controllers_app.go"}, identifiers: []string{"NewController", "Routes"}, stringLiterals: []string{"/avatars/:id"}},
				{id: "avatar-service-registration", paths: []string{"app/wire/inject_http_controllers_app.go", "app/wire/inject_services_app.go"}, identifiers: []string{"NewService"}, selectorCalls: []string{"Avatars"}},
			},
			commands: append(standardSurfaceCommands(commandContract{
				id:        "avatar-revalidation-behavior",
				arguments: []string{"go", "test", "./internal/avatars", "-run", "^TestAtlasAvatarRevalidationBehavior$", "-count=1"},
				supervisorFiles: []supervisorFile{{
					path: "internal/avatars/atlas_eval_avatar_revalidation_test.go",
					body: avatarRevalidationBehaviorProbe,
				}},
			}), commandContract{id: "avatar-route-visible", arguments: []string{"forj", "route:list"}, contains: []string{"/api/v1/avatars/:id"}}),
		},
		{
			id:             "repair-wire-provider/v1",
			allowedChanges: []string{"app/wire/inject_services_app.go"},
			sources: []sourceContract{
				{id: "provider-registration", paths: []string{"app/wire/inject_services_app.go"}, identifiers: []string{"NewService"}},
			},
			commands: append(standardSurfaceCommands(), commandContract{id: "report-route-visible", arguments: []string{"forj", "route:list"}, contains: []string{"/api/v1/reports"}}),
		},
		{
			id:              "runtime-observability/v1",
			allowedChanges:  []string{".env.local"},
			requiredChanges: []string{".env.local"},
			sources: []sourceContract{
				{id: "local-inspect-capture-config", paths: []string{".env.local"}, text: []string{"LIGHTHOUSE_INSPECT_ENABLED="}},
				{id: "metrics-endpoint", paths: []string{"internal/metrics/endpoint.go"}, declarations: []declarationContract{{name: "StartPrometheusEndpoint", identifiers: []string{"ctx", "registry"}, selectorCalls: []string{"Handler", "Listen", "Shutdown"}}}},
				{id: "metrics-surface", paths: []string{"internal/metrics/README.md"}, text: []string{"/metrics", "METRICS_HTTP_ENABLED", "METRICS_EVENTS_ENABLED", "METRICS_QUEUE_ENABLED", "METRICS_SCHEDULER_ENABLED"}},
				{id: "inspect-lighthouse-surface", paths: []string{"internal/inspects/README.md"}, text: []string{"LIGHTHOUSE_INSPECT_ENABLED", "Lighthouse", "HTTP request", "job execution", "scheduler run"}},
			},
			commands: append(standardSurfaceCommands(commandContract{
				id:        "local-inspect-capture-behavior",
				arguments: []string{"go", "test", "./internal/inspects", "-run", "^TestAtlasLocalInspectCaptureBehavior$", "-count=1"},
				supervisorFiles: []supervisorFile{{
					path: "internal/inspects/atlas_eval_local_capture_test.go",
					body: runtimeObservabilityBehaviorProbe,
				}},
			}), commandContract{id: "metrics-route-visible", arguments: []string{"forj", "route:list"}, contains: []string{"/metrics"}}),
		},
		{
			id:             "build-json-api-feature/v1",
			allowedChanges: []string{"internal/users/*.go", "app/routes.go", "app/wire/inject_http_controllers_app.go", "app/wire/inject_services_app.go"},
			sources: []sourceContract{
				{id: "users-application-boundary", paths: []string{"internal/users/*.go"}, identifiers: []string{"User", "Controller"}, identifierChoices: [][]string{{"Service", "UseCase", "Query"}, {"Find", "Get", "Lookup"}}, forbiddenCalls: []string{"Background", "TODO"}, stringLiterals: []string{"/users/:id"}, declarations: []declarationContract{{name: "Routes", receiver: "Controller", selectorCalls: []string{"NewRoute"}}}},
				{id: "users-registration", paths: []string{"app/routes.go", "app/wire/inject_http_controllers_app.go", "app/wire/inject_services_app.go"}, identifiers: []string{"NewController"}, identifierChoices: [][]string{{"NewService", "NewUseCase", "NewQuery"}}},
			},
			commands: append(standardSurfaceCommands(commandContract{
				id:        "json-api-behavior",
				arguments: []string{"go", "test", "./internal/users", "-run", "^TestAtlasJSONAPIFeatureBehavior$", "-count=1"},
				supervisorFiles: []supervisorFile{{
					path: "internal/users/atlas_eval_json_api_test.go",
					body: jsonAPIFeatureBehaviorProbe,
				}},
			}), commandContract{id: "users-route-visible", arguments: []string{"forj", "route:list"}, contains: []string{"/api/v1/users/:id"}}),
		},
		{
			id:             "create-additional-app/v1",
			allowedChanges: generatedEnvironmentChanges(".env.local", ".goforj.yml", "Dockerfile", "Makefile", "app/statuspage/**", "cmd/statuspage/**", "internal/runtime/apps.go", "internal/runtime/apps_test.go"),
			sources: []sourceContract{
				{id: "statuspage-project-config", paths: []string{".goforj.yml"}, appConfiguration: &appConfigurationContract{name: "statuspage", requiredComponents: []string{"web_api"}}},
				{id: "statuspage-entrypoint", paths: []string{"cmd/statuspage/main.go"}, declarations: []declarationContract{{name: "main", selectorCalls: []string{"LaunchApplication"}}}},
				{id: "statuspage-app-boundary", paths: []string{"app/statuspage/routes.go", "app/statuspage/wire/*.go"}, identifiers: []string{"ProvideRoutes", "InitializeApplication"}},
			},
			commands: standardSurfaceCommandsWithBuilds([][]string{{"forj", "build"}, {"forj", "statuspage", "build"}}),
		},
		{
			id:                  "add-app-lifecycle-hook/v1",
			allowedChanges:      []string{"app/lifecycle.go"},
			qualityTestPatterns: []string{"app/*_test.go"},
			sources: []sourceContract{
				{id: "application-readiness-hook", paths: []string{"app/lifecycle.go"}, identifiers: []string{"LifecycleRegistry", "NewLifecycleRegistry", "Service"}, forbiddenCalls: []string{"TODO"}, text: []string{"runtime.BeforeStartup"}, declarations: []declarationContract{{name: "Register", receiver: "LifecycleRegistry", selectorCalls: []string{"On"}}, {name: "BeforeStartup", receiver: "LifecycleRegistry", identifiers: []string{"ctx"}, selectorCalls: []string{"Find"}, forbiddenCalls: []string{"Background"}}}},
			},
			commands: standardSurfaceCommands(commandContract{
				id:              "application-readiness-behavior",
				arguments:       []string{"go", "test", "./app", "-run", "^TestAtlasLifecycleReadinessBehavior$", "-count=1"},
				supervisorFiles: []supervisorFile{{path: "app/atlas_eval_lifecycle_readiness_test.go", body: lifecycleReadinessBehaviorProbe}},
			}),
		},
		{
			id:                  "add-outbound-http-integration/v1",
			allowedChanges:      []string{".env.example", "go.mod", "go.sum", "internal/taxrates/*.go", "app/wire/inject_services_app.go", "app/wire/*_test.go"},
			qualityTestPatterns: []string{"internal/taxrates/*_test.go", "app/wire/*_test.go"},
			sources: []sourceContract{
				{id: "typed-http-client", paths: []string{"internal/taxrates/*.go"}, identifiers: []string{"Rate", "Client", "NewClient"}, forbiddenCalls: []string{"TODO"}, declarations: []declarationContract{{name: "Find", receiver: "Client", identifiers: []string{"ctx"}, forbiddenCalls: []string{"Background"}}}},
				{id: "http-client-provider", paths: []string{"app/wire/inject_services_app.go"}, identifiers: []string{"provideTaxRateClient", "NewClient"}, selectorCalls: []string{"Get"}},
			},
			commands: standardSurfaceCommands(commandContract{
				id:              "tax-rate-http-behavior",
				arguments:       []string{"go", "test", "./internal/taxrates", "-run", "^TestAtlasTaxRateHTTPBehavior$", "-count=1"},
				supervisorFiles: []supervisorFile{{path: "internal/taxrates/atlas_eval_http_behavior_test.go", body: taxRateHTTPBehaviorProbe}},
			}),
		},
		{
			id:                  "add-validated-write-endpoint/v1",
			allowedChanges:      []string{"internal/invoices/*.go"},
			qualityTestPatterns: []string{"internal/invoices/*_test.go"},
			sources: []sourceContract{
				{id: "validated-request-boundary", paths: []string{"internal/invoices/controller.go"}, identifiers: []string{"createInvoiceRequest", "validationErrorResponse", "CreateInput", "MethodPost"}, forbiddenCalls: []string{"TODO"}, stringLiterals: []string{"/invoices", "invalid_payload", "validation_failed", "customer_id", "total_cents"}, declarations: []declarationContract{{name: "Store", receiver: "Controller", selectorCalls: []string{"Bind", "Create", "JSON"}, forbiddenCalls: []string{"Background"}}}},
				{id: "invoice-create-behavior", paths: []string{"internal/invoices/repository.go", "internal/invoices/service.go"}, identifiers: []string{"Repository", "Create", "CreateInput"}, forbiddenCalls: []string{"Background", "TODO"}, declarations: []declarationContract{{name: "Repository", identifiers: []string{"Create"}}, {name: "Create", receiver: "MemoryRepository", identifiers: []string{"Invoice"}}, {name: "Create", receiver: "Service", identifiers: []string{"ctx"}, selectorCalls: []string{"Create"}}}},
			},
			commands: append(standardSurfaceCommands(commandContract{
				id:        "invoice-validation-behavior",
				arguments: []string{"go", "test", "./internal/invoices", "-run", "^TestAtlasInvoiceValidationBehavior$", "-count=1"},
				supervisorFiles: []supervisorFile{{
					path: "internal/invoices/atlas_eval_invoice_validation_test.go",
					body: invoiceValidationBehaviorProbe,
				}},
			}), commandContract{id: "invoice-write-route", arguments: []string{"forj", "route:list"}, contains: []string{"/api/v1/invoices"}}),
		},
		{
			id:                  "add-route-middleware/v1",
			allowedChanges:      []string{"internal/invoices/controller.go", "internal/invoices/middleware.go", "internal/invoices/middleware_test.go", "app/wire/inject_http_controllers_app.go"},
			qualityTestPatterns: []string{"internal/invoices/*_test.go"},
			sources: []sourceContract{
				{id: "invoice-token-policy", paths: []string{"internal/invoices/middleware.go", "internal/invoices/controller.go"}, identifiers: []string{"RequireToken"}, selectorCalls: []string{"NewRoute"}, forbiddenCalls: []string{"Background", "TODO"}, text: []string{"RequireToken(controller.token)"}},
				{id: "resolved-token-provider", paths: []string{"app/wire/inject_http_controllers_app.go"}, identifiers: []string{"provideInvoiceController"}, declarations: []declarationContract{{name: "provideInvoiceController", identifiers: []string{"NewController"}, forbiddenLiterals: []string{"invoice-secret"}, argumentFlows: []callArgumentFlowContract{{call: "NewController", literal: "INVOICE_HTTP_TOKEN"}}}}},
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
			allowedChanges: []string{"go.mod", "go.sum", "internal/accounts/*.go", "app/wire/app.go", "app/wire/inject_repositories_app.go", "app/wire/inject_services_app.go"},
			sources: []sourceContract{
				{id: "transaction-bound-repository", paths: []string{"internal/accounts/repository.go"}, identifiers: []string{"Repository", "WithTransaction", "AdjustBalance"}, forbiddenCalls: []string{"TODO"}, declarations: []declarationContract{{name: "WithTransaction", receiver: "Repository", identifiers: []string{"ctx"}, selectorCalls: []string{"Transaction"}}, {name: "AdjustBalance", receiver: "Repository", identifiers: []string{"ctx"}, selectorCalls: []string{"UpdateColumn"}}}},
				{id: "atomic-transfer-service", paths: []string{"internal/accounts/service.go"}, identifiers: []string{"Service", "Transfer"}, forbiddenCalls: []string{"TODO"}, declarations: []declarationContract{{name: "Transfer", receiver: "Service", selectorCalls: []string{"WithTransaction", "AdjustBalance"}, forbiddenCalls: []string{"Background"}, nestedCalls: []nestedCallContract{{outer: "WithTransaction", inner: "AdjustBalance"}}}}},
				{id: "account-repository-registration", paths: []string{"app/wire/app.go", "app/wire/inject_repositories_app.go"}, identifierChoices: [][]string{{"NewRepository", "ProvideRepository"}}},
				{id: "account-service-registration", paths: []string{"app/wire/inject_services_app.go"}, identifiers: []string{"NewService"}},
			},
			commands: standardSurfaceCommands(commandContract{
				id:        "transaction-behavior",
				arguments: []string{"go", "test", "./internal/accounts", "-run", "^TestAtlasTransferTransactionBehavior$", "-count=1"},
				supervisorFiles: []supervisorFile{{
					path: "internal/accounts/atlas_eval_transfer_transaction_test.go",
					body: transferTransactionBehaviorProbe,
				}},
			}),
		},
		{
			id:                  "add-mail-workflow/v1",
			allowedChanges:      []string{"internal/invoices/receipt_mailer.go", "internal/invoices/receipt_mailer_test.go", "app/wire/app.go", "app/wire/inject_services_app.go"},
			qualityTestPatterns: []string{"internal/invoices/*_test.go"},
			sources: []sourceContract{
				{id: "receipt-mail-service", paths: []string{"internal/invoices/receipt_mailer.go"}, identifiers: []string{"ReceiptMailer", "NewReceiptMailer", "Send", "Manager"}, forbiddenCalls: []string{"Background", "TODO"}, declarations: []declarationContract{{name: "Send", receiver: "ReceiptMailer", selectorCalls: []string{"Find", "Default", "Message", "To", "Subject", "Text", "Send"}}}},
				{id: "receipt-mail-registration", paths: []string{"app/wire/inject_services_app.go"}, identifiers: []string{"NewReceiptMailer"}},
			},
			forbiddenText: []textExclusion{{id: "no-provider-sdk", paths: []string{"internal/invoices/receipt_mailer.go"}, text: "smtp"}},
			commands: standardSurfaceCommands(commandContract{
				id:              "receipt-mail-behavior",
				arguments:       []string{"go", "test", "./internal/invoices", "-run", "^TestAtlasReceiptMailBehavior$", "-count=1"},
				supervisorFiles: []supervisorFile{{path: "internal/invoices/atlas_eval_receipt_mail_test.go", body: receiptMailBehaviorProbe}},
			}),
		},
		{
			id:             "protect-route-with-auth/v1",
			allowedChanges: generatedEnvironmentChanges(".env.local", "app/routes.go"),
			sources: []sourceContract{
				{id: "generated-auth-composition", paths: []string{"app/routes.go"}, identifiers: []string{"publicRoutes", "protectedRoutes", "invoicesController", "authService", "RequireAuth"}, selectorCalls: []string{"NewRouteGroup"}, assignments: []assignmentContract{{name: "publicRoutes", forbiddenIdentifiers: []string{"invoicesController"}}, {name: "protectedRoutes", identifiers: []string{"invoicesController"}, selectorCalls: []string{"Routes"}}}, routeGroups: []routeGroupContract{{routesIdentifier: "protectedRoutes", middlewareSelector: "RequireAuth"}}},
			},
			commands: append(standardSurfaceCommands(),
				commandContract{id: "protected-route-visible", arguments: []string{"forj", "route:list"}, contains: []string{"/api/v1/invoices/:id"}},
				commandContract{id: "protected-middleware-visible", arguments: []string{"forj", "route:list"}, contains: []string{"RequireAuth"}},
			),
		},
		{
			id:                     "add-cached-repository/v1",
			allowedChanges:         generatedEnvironmentChanges("internal/caches/*_gen.go", "internal/users/*.go", "app/wire/inject_services_app.go"),
			baselineTestExclusions: []string{"internal/users/service_test.go"},
			sources: []sourceContract{
				{id: "cache-aside-repository", paths: []string{"internal/users/*.go"}, identifiers: []string{"User"}, forbiddenCalls: []string{"Background", "TODO"}, declarations: []declarationContract{{name: "Find", receiver: "Service", identifiers: []string{"ctx"}, selectorCalls: []string{"Find"}}}},
				{id: "profiles-cache-access", paths: []string{"internal/users/*.go", "app/wire/inject_services_app.go"}, selectorCalls: []string{"Profiles"}},
			},
			commands: standardSurfaceCommands(commandContract{
				id:    "cache-aside-behavior",
				probe: runCacheBehaviorProbe,
			}),
		},
		{
			id:                  "add-upload-workflow/v1",
			allowedChanges:      generatedEnvironmentChanges("go.mod", "internal/storages/*_gen.go", "internal/uploads/*.go", "app/routes.go", "app/wire/inject_http_controllers_app.go", "app/wire/inject_services_app.go"),
			qualityTestPatterns: []string{"internal/uploads/*_test.go"},
			sources: []sourceContract{
				{id: "upload-boundary", paths: []string{"internal/uploads/service.go", "internal/uploads/controller.go"}, identifiers: []string{"StoreInput", "StoredUpload", "Service", "Controller", "Store", "Routes"}, forbiddenCalls: []string{"Background", "TODO"}, stringLiterals: []string{"/uploads"}, declarations: []declarationContract{{name: "Store", receiver: "Service", identifiers: []string{"ctx"}}, {name: "Store", receiver: "Controller", selectorCalls: []string{"Bind", "Store"}}, {name: "Routes", receiver: "Controller", selectorCalls: []string{"NewRoute"}}}},
				{id: "uploads-storage-registration", paths: []string{"app/wire/inject_services_app.go", "internal/uploads/service.go"}, identifiers: []string{"NewService"}, selectorCalls: []string{"Uploads"}},
			},
			commands: append(standardSurfaceCommands(commandContract{
				id:              "upload-workflow-behavior",
				arguments:       []string{"go", "test", "./internal/uploads", "-run", "^TestAtlasUploadWorkflowBehavior$", "-count=1"},
				supervisorFiles: []supervisorFile{{path: "internal/uploads/atlas_eval_upload_workflow_test.go", body: uploadWorkflowBehaviorProbe}},
			}), commandContract{id: "uploads-route-visible", arguments: []string{"forj", "route:list"}, contains: []string{"/api/v1/uploads"}}),
		},
		{
			id:                  "publish-domain-event/v1",
			allowedChanges:      []string{"internal/events/*.go", "internal/users/*.go", "internal/notifications/*.go", "app/routes.go", "app/lifecycle.go", "app/wire/inject_http_controllers_app.go", "app/wire/inject_services_app.go", "app/wire/inject_subscribers_app.go"},
			qualityTestPatterns: []string{"internal/users/*_test.go", "internal/notifications/*_test.go"},
			sources: []sourceContract{
				{id: "typed-user-event", paths: []string{"internal/events/*.go"}, identifiers: []string{"UserID", "Topic"}, identifierChoices: [][]string{{"UserCreated", "UserCreatedEvent"}}, stringLiterals: []string{"users.created"}, declarations: []declarationContract{{nameChoices: []string{"UserCreated", "UserCreatedEvent"}, identifiers: []string{"UserID"}, forbiddenIdentifiers: []string{"Email"}}}},
				{id: "domain-event-publication", paths: []string{"internal/users/*.go"}, identifiers: []string{"UserEvents"}, forbiddenCalls: []string{"Background", "TODO"}, declarations: []declarationContract{{name: "Create", receiver: "Service", identifiers: []string{"ctx"}, selectorCalls: []string{"Save"}}}},
				{id: "event-reaction", paths: []string{"internal/notifications/*.go", "app/lifecycle.go"}, identifiers: []string{"Subscribers", "UserCreatedHandler", "Register", "Startup", "Shutdown"}, forbiddenCalls: []string{"Background", "TODO"}, declarations: []declarationContract{{name: "Register", receiver: "Subscribers", identifiers: []string{"UserID"}, selectorCalls: []string{"Subscribe"}}, {name: "Startup", receiver: "LifecycleRegistry", selectorCalls: []string{"Register"}}, {name: "Shutdown", receiver: "LifecycleRegistry", selectorCalls: []string{"Close"}}}},
				{id: "user-event-publisher", paths: []string{"internal/users/*.go", "internal/events/*.go"}, identifiers: []string{"UserEventPublisher", "NewUserEventPublisher"}, forbiddenCalls: []string{"Background", "TODO"}, declarations: []declarationContract{{anyName: true, receiver: "UserEventPublisher", identifiers: []string{"ctx", "UserID"}, selectorCalls: []string{"Publish", "WithContext"}}}},
			},
			commands: standardSurfaceCommands(commandContract{id: "domain-event-behavior", probe: runDomainEventBehaviorProbe}),
		},
		{
			id:                  "dispatch-event-followup-job/v1",
			allowedChanges:      generatedEnvironmentChanges("internal/jobs/*.go", "internal/reports/*.go", "internal/notifications/*.go", "internal/storages/*_gen.go", "app/lifecycle.go", "app/wire/inject_jobs_app.go", "app/wire/inject_services_app.go", "app/wire/inject_subscribers_app.go"),
			qualityTestPatterns: []string{"internal/reports/*_test.go", "internal/notifications/*_test.go"},
			sources: []sourceContract{
				{id: "typed-report-job", paths: []string{"internal/reports/*.go"}, identifiers: []string{"GeneratePayload", "GenerateJob", "GenerateJobTypeName", "HandleTask"}, forbiddenCalls: []string{"Background", "TODO"}, stringLiterals: []string{"reports:generate"}, declarations: []declarationContract{{name: "GeneratePayload", identifiers: []string{"UserID"}, forbiddenIdentifiers: []string{"Email"}}, {name: "GenerateForUser", receiver: "Service", selectorCalls: []string{"Find"}}, {name: "HandleTask", receiver: "GenerateJob", selectorCalls: []string{"Bind", "GenerateForUser"}}, {name: "Queue", receiver: "GenerateJob", selectorCalls: []string{"Dispatch"}}}},
				{id: "event-job-boundary", paths: []string{"internal/notifications/service.go"}, identifiers: []string{"HandleUserCreated", "ReportQueue"}, forbiddenCalls: []string{"Background", "TODO"}, declarations: []declarationContract{{name: "HandleUserCreated", receiver: "Service", selectorCalls: []string{"Queue"}}}},
				{id: "report-job-registration", paths: []string{"app/wire/inject_jobs_app.go", "app/wire/inject_services_app.go"}, identifiers: []string{"NewGenerateJob", "NewService"}},
			},
			commands: standardSurfaceCommands(commandContract{
				id:              "event-followup-job-behavior",
				arguments:       []string{"go", "test", "./internal/reports", "-run", "^TestAtlasEventFollowupJobBehavior$", "-count=1"},
				supervisorFiles: []supervisorFile{{path: "internal/reports/atlas_eval_event_followup_job_test.go", body: eventFollowupJobBehaviorProbe}},
			}),
		},
		{
			id:                  "add-resilient-job/v1",
			allowedChanges:      generatedEnvironmentChanges("internal/jobs/*.go", "internal/reports/*.go", "internal/notifications/*.go", "internal/storages/*_gen.go", "app/lifecycle.go", "app/wire/inject_jobs_app.go", "app/wire/inject_services_app.go", "app/wire/inject_subscribers_app.go"),
			qualityTestPatterns: []string{"internal/reports/*_test.go", "internal/notifications/*_test.go"},
			sources: []sourceContract{
				{id: "retry-safe-report-job", paths: []string{"internal/reports/*.go"}, identifiers: []string{"GeneratePayload", "GenerateJob", "GenerateJobTypeName", "HandleTask"}, forbiddenCalls: []string{"Background", "TODO"}, stringLiterals: []string{"reports:generate", "profile.json"}, declarations: []declarationContract{{name: "GeneratePayload", identifiers: []string{"UserID"}, forbiddenIdentifiers: []string{"Email"}}, {name: "GenerateForUser", receiver: "Service", selectorCalls: []string{"Find", "Put"}}, {name: "HandleTask", receiver: "GenerateJob", selectorCalls: []string{"Bind", "GenerateForUser"}}, {name: "Queue", receiver: "GenerateJob", selectorCalls: []string{"Dispatch", "Retry", "Timeout"}}}},
				{id: "resilient-job-boundary", paths: []string{"internal/notifications/service.go"}, identifiers: []string{"HandleUserCreated", "ReportQueue"}, forbiddenCalls: []string{"Background", "TODO"}, declarations: []declarationContract{{name: "HandleUserCreated", receiver: "Service", selectorCalls: []string{"Queue"}}}},
				{id: "resilient-job-registration", paths: []string{"app/wire/inject_jobs_app.go", "app/wire/inject_services_app.go"}, identifiers: []string{"NewGenerateJob", "NewService"}},
			},
			commands: standardSurfaceCommands(commandContract{
				id:              "resilient-job-behavior",
				arguments:       []string{"go", "test", "./internal/reports", "-run", "^TestAtlasResilientJobBehavior$", "-count=1"},
				supervisorFiles: []supervisorFile{{path: "internal/reports/atlas_eval_resilient_job_test.go", body: resilientJobBehaviorProbe}},
			}),
		},
		{
			id:             "schedule-existing-job/v1",
			allowedChanges: []string{"internal/reports/*.go", "internal/users/*.go", "app/wire/inject_schedules_app.go", "app/wire/inject_services_app.go"},
			sources: []sourceContract{
				{id: "scheduled-job-dispatch", paths: []string{"internal/reports/*.go"}, identifiers: []string{"DailyTargetRepository", "DailyRunner", "DailySchedule", "Interval", "Handle"}, forbiddenCalls: []string{"Background", "TODO"}, stringLiterals: []string{"reports:daily"}, declarations: []declarationContract{{name: "Run", receiver: "DailyRunner", selectorCalls: []string{"ListDailyReportTargets", "Queue"}}, {name: "Handle", receiver: "DailySchedule", selectorCalls: []string{"Run"}}}},
				{id: "daily-target-repository", paths: []string{"internal/users/*.go"}, identifiers: []string{"ListDailyReportTargets"}},
				{id: "daily-schedule-registration", paths: []string{"app/wire/inject_schedules_app.go", "app/wire/inject_services_app.go"}, identifiers: []string{"NewDailySchedule", "NewDailyRunner"}},
			},
			commands: standardSurfaceCommands(commandContract{
				id:              "daily-schedule-behavior",
				arguments:       []string{"go", "test", "./internal/reports", "-run", "^TestAtlasDailyScheduleBehavior$", "-count=1"},
				supervisorFiles: []supervisorFile{{path: "internal/reports/atlas_eval_daily_schedule_test.go", body: dailyScheduleBehaviorProbe}},
			}),
		},
	}
}

// generatedEnvironmentChanges keeps synchronized environment mirrors inside ownership whenever a task changes project configuration.
func generatedEnvironmentChanges(paths ...string) []string {
	changes := []string{".env", ".env.example", ".env.testing"}
	return append(changes, paths...)
}

const tokenPolicyBehaviorProbe = `package invoices

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/goforj/web"
	"github.com/goforj/web/webtest"
)

// TestAtlasRequireTokenBehavior verifies rejected and accepted policy paths through a supervisor-owned oracle.
func TestAtlasRequireTokenBehavior(t *testing.T) {
	tests := []struct {
		name     string
		expected string
		provided string
		want     int
		next     bool
	}{
		{name: "unconfigured", want: http.StatusUnauthorized},
		{name: "missing", expected: "secret", want: http.StatusUnauthorized},
		{name: "accepted", expected: "secret", provided: "secret", want: http.StatusNoContent, next: true},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			request := httptest.NewRequest(http.MethodGet, "/invoices/inv-42", nil)
			request.Header.Set("X-Invoice-Token", test.provided)
			response := httptest.NewRecorder()
			context := webtest.NewContext(request, response, "/invoices/:id", nil)
			calledNext := false
			next := func(context web.Context) error {
				calledNext = true
				return context.NoContent(http.StatusNoContent)
			}
			if err := RequireToken(test.expected)(next)(context); err != nil {
				t.Fatalf("middleware: %v", err)
			}
			if response.Code != test.want {
				t.Fatalf("status = %d, want %d", response.Code, test.want)
			}
			if calledNext != test.next {
				t.Fatalf("next called = %v, want %v", calledNext, test.next)
			}
			if !test.next {
				var payload map[string]string
				if err := json.Unmarshal(response.Body.Bytes(), &payload); err != nil {
					t.Fatalf("decode unauthorized response: %v", err)
				}
				found := false
				for _, value := range payload {
					found = found || value == "unauthorized"
				}
				if !found {
					t.Fatalf("unauthorized response = %#v", payload)
				}
			}
		})
	}
}
`

// standardSurfaceCommands proves the complete Project after source-level checks pass.
func standardSurfaceCommands(additional ...commandContract) []commandContract {
	return standardSurfaceCommandsWithBuilds(defaultWireBuildCommands(), additional...)
}

// standardSurfaceCommandsWithBuilds regenerates every App owned by a surface before sharing that private phase with compilation.
func standardSurfaceCommandsWithBuilds(builds [][]string, additional ...commandContract) []commandContract {
	commands := []commandContract{
		{standard: true, standardBuilds: builds},
	}
	return append(commands, additional...)
}

// defaultWireBuildCommands returns the supported regeneration path for the default App.
func defaultWireBuildCommands() [][]string {
	return [][]string{{"forj", "build"}}
}
