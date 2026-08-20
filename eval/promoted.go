package eval

// PromotedWorkflows returns the versioned workflow contracts available to live evaluations.
func PromotedWorkflows() []WorkflowExpectation {
	return []WorkflowExpectation{
		{
			ID: "goforj-add-http-route/v1",
			Generators: []GeneratorRequirement{
				{ID: "generate-invoice-controller", Arguments: []string{"make:controller", "invoices"}},
			},
			Requirements: []WorkflowRequirement{
				qualityInspection("inspect-project", "Inspect the owning App and existing invoice boundary before editing.", "app/**", "internal/invoices/**"),
			},
		},
		promotedGeneratorWorkflow("goforj-add-app-command/v1", "generate-invoice-command", "make:command", "invoices:show"),
		promotedGeneratorWorkflow("goforj-add-job/v1", "generate-receipt-job", "make:job", "invoices:receipt"),
		promotedGeneratorWorkflow("goforj-add-migration/v1", "generate-invoice-status-migration", "make:migration", "add_status_to_invoices"),
		promotedGeneratorWorkflow("goforj-add-schedule/v1", "generate-reconcile-schedule", "make:schedule", "invoices:reconcile", "--every", "1h"),
		{
			ID: "goforj-add-event-subscriber/v1",
			Generators: []GeneratorRequirement{
				{ID: "generate-invoice-paid-event", Arguments: []string{"make:event", "invoices:paid"}},
				{ID: "generate-invoice-paid-subscriber", Arguments: []string{"make:subscriber", "invoices:paid"}},
			},
		},
		promotedGeneratorWorkflow("goforj-create-model/v1", "generate-user-model", "make:model", "users"),
		{
			ID: "goforj-create-data-resource/v1",
			Generators: []GeneratorRequirement{
				{ID: "generate-photo-migration", Arguments: []string{"make:migration", "create_photos", "--no-open"}},
				{ID: "generate-photo-model", Arguments: []string{"make:model", "photos", "--package", "photos", "--no-open"}},
			},
			Requirements: []WorkflowRequirement{
				qualityInspection("inspect-data-workflow", "Inspect the database connection and schema between migration application and model generation.", ".goforj.yml", ".env*", "migrations/**", "app/wire/inject_repositories_app.go"),
			},
		},
		{
			ID: "goforj-model-relationships/v1",
			Generators: []GeneratorRequirement{
				{ID: "generate-post-model", Arguments: []string{"make:model", "posts"}},
				{ID: "generate-user-model", Arguments: []string{"make:model", "users"}},
			},
			Requirements: []WorkflowRequirement{qualityInspection("inspect-related-schema", "Inspect the related tables and explicit relationship contract before generating models.", ".db-relationships.yaml", "migrations/**", "internal/content/**", "internal/models/**")},
		},
		promotedGeneratorWorkflow("goforj-create-additional-app/v1", "generate-statuspage-app", "make:app", "statuspage", "--components", "web-api", "--dev-run", "run"),
		{
			ID:           "goforj-add-app-lifecycle-hook/v1",
			Requirements: []WorkflowRequirement{qualityInspection("inspect-lifecycle-registry", "Inspect the App lifecycle registry and existing application boundary before registering readiness work.", "app/lifecycle.go", "internal/invoices/**")},
		},
		{
			ID:           "goforj-add-outbound-http-integration/v1",
			Requirements: []WorkflowRequirement{qualityInspection("inspect-integration-boundary", "Inspect the existing application boundary and keep the remote contract in a focused context-aware client.", "app/wire/**", "internal/invoices/**", "internal/taxrates/**")},
		},
		{
			ID:           "goforj-add-validated-write-endpoint/v1",
			Requirements: []WorkflowRequirement{qualityInspection("inspect-request-boundary", "Inspect the existing controller, service, and repository before extending the write contract.", "internal/invoices/**")},
		},
		{
			ID:           "goforj-add-route-middleware/v1",
			Requirements: []WorkflowRequirement{qualityInspection("inspect-route-composition", "Inspect route composition and resolve middleware configuration outside the request path.", "app/routes.go", "app/wire/**", "internal/invoices/**")},
		},
		{
			ID:           "goforj-add-database-transaction/v1",
			Requirements: []WorkflowRequirement{qualityInspection("inspect-database-boundary", "Inspect generated database access and keep the transaction boundary in the application service.", "internal/accounts/**", "internal/database/**")},
		},
		{
			ID:           "goforj-add-mail-workflow/v1",
			Requirements: []WorkflowRequirement{qualityInspection("inspect-mail-boundary", "Inspect the generated mail manager and existing service boundary before adding delivery.", "internal/invoices/**", "internal/mail/**")},
		},
		{
			ID:           "goforj-protect-route-with-auth/v1",
			Requirements: []WorkflowRequirement{qualityInspection("inspect-auth-route-groups", "Inspect generated Auth route composition and reuse its existing middleware boundary.", "app/routes.go", "internal/auth/**")},
		},
		promotedGeneratorWorkflow("goforj-add-named-app-route/v1", "generate-admin-audit-controller", "admin", "make:controller", "audits"),
		promotedGeneratorWorkflow("goforj-add-named-resource/v1", "generate-reports-queue", "make:queue", "reports", "--workers", "2"),
		promotedGeneratorWorkflow("goforj-add-named-cache/v1", "generate-profiles-cache", "generate", "--cache"),
		promotedGeneratorWorkflow("goforj-add-named-storage/v1", "generate-avatar-storage", "generate", "--storage"),
		{
			ID:           "goforj-choose-storage-for-files/v1",
			Generators:   []GeneratorRequirement{{ID: "generate-attachment-storage", Arguments: []string{"generate", "--storage"}}},
			Requirements: []WorkflowRequirement{qualityInspection("inspect-file-resource-boundary", "Inspect existing storage resources and feature ownership before choosing where durable files belong.", ".env", "internal/storages/**", "internal/invoices/**")},
		},
		promotedGeneratorWorkflow("goforj-serve-cacheable-image/v1", "generate-avatar-controller", "make:controller", "avatars"),
		promotedGeneratorWorkflow("goforj-build-json-api-feature/v1", "generate-users-controller", "make:controller", "users"),
		{
			ID:           "goforj-add-cached-repository/v1",
			Requirements: []WorkflowRequirement{qualityInspection("inspect-repository-boundary", "Inspect the existing service and keep cache-aside behavior behind its repository contract.", "internal/users/**", "internal/caches/**")},
		},
		promotedGeneratorWorkflow("goforj-add-upload-workflow/v1", "generate-uploads-controller", "make:controller", "uploads"),
		{
			ID:           "goforj-publish-domain-event/v1",
			Requirements: []WorkflowRequirement{qualityInspection("inspect-domain-boundary", "Inspect the existing write workflow before publishing a typed domain fact through the App event bus.", "app/lifecycle.go", "internal/users/**")},
		},
		promotedGeneratorWorkflow("goforj-dispatch-event-followup-job/v1", "generate-report-job", "make:job", "reports:generate"),
		promotedGeneratorWorkflow("goforj-add-resilient-job/v1", "generate-resilient-report-job", "make:job", "reports:generate"),
		promotedGeneratorWorkflow("goforj-schedule-existing-job/v1", "generate-report-schedule", "make:schedule", "reports:daily", "--every", "24h", "--no-open"),
		{
			ID:           "goforj-repair-wire-provider/v1",
			Requirements: []WorkflowRequirement{qualityInspection("inspect-wire-diagnostics", "Inspect the Wire failure and owning provider set before editing.", "app/wire/**", "internal/reports/**")},
		},
		{
			ID:           "goforj-runtime-observability/v1",
			Requirements: []WorkflowRequirement{qualityInspection("inspect-observability-surfaces", "Inspect generated metrics, inspect capture, and Lighthouse operator surfaces before reporting runtime readiness.", ".env.local", "internal/metrics/**", "internal/inspects/**")},
		},
		{
			ID:           "goforj-clarify-unknown-shape/v1",
			Requirements: []WorkflowRequirement{qualityInspection("inspect-project", "Inspect the existing application boundaries before requesting a decision.", ".goforj.yml", "app/**", "internal/**")},
		},
	}
}

// promotedGeneratorWorkflow keeps one-generator contracts concise without weakening exact argument matching.
func promotedGeneratorWorkflow(id, requirement string, arguments ...string) WorkflowExpectation {
	return WorkflowExpectation{
		ID:           id,
		Generators:   []GeneratorRequirement{{ID: requirement, Arguments: append([]string(nil), arguments...)}},
		Requirements: []WorkflowRequirement{qualityInspection("inspect-project", "Inspect the owning App and existing application boundary before editing.", ".goforj.yml", "app/**", "internal/**")},
	}
}

// qualityInspection records scoped read evidence without promoting an implementation preference into workflow conformance.
func qualityInspection(id, description string, paths ...string) WorkflowRequirement {
	return WorkflowRequirement{ID: id, Kind: RequirementQuality, Capability: CapabilityFileReads, Description: description, Paths: append([]string(nil), paths...)}
}
