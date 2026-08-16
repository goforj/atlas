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
				{
					ID:          "inspect-project",
					Kind:        RequirementQuality,
					Capability:  CapabilityFileReads,
					Description: "Inspect the owning App and existing invoice boundary before editing.",
				},
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
		promotedGeneratorWorkflow("goforj-create-additional-app/v1", "generate-statuspage-app", "make:app", "statuspage", "--components", "web-api", "--dev-run", "run"),
		{
			ID:           "goforj-add-app-lifecycle-hook/v1",
			Requirements: []WorkflowRequirement{{ID: "inspect-lifecycle-registry", Kind: RequirementQuality, Capability: CapabilityFileReads, Description: "Inspect the App lifecycle registry and existing application boundary before registering readiness work."}},
		},
		{
			ID:           "goforj-add-outbound-http-integration/v1",
			Requirements: []WorkflowRequirement{{ID: "inspect-integration-boundary", Kind: RequirementQuality, Capability: CapabilityFileReads, Description: "Inspect the existing application boundary and keep the remote contract in a focused context-aware client."}},
		},
		{
			ID:           "goforj-add-validated-write-endpoint/v1",
			Requirements: []WorkflowRequirement{{ID: "inspect-request-boundary", Kind: RequirementQuality, Capability: CapabilityFileReads, Description: "Inspect the existing controller, service, and repository before extending the write contract."}},
		},
		{
			ID:           "goforj-add-route-middleware/v1",
			Requirements: []WorkflowRequirement{{ID: "inspect-route-composition", Kind: RequirementQuality, Capability: CapabilityFileReads, Description: "Inspect route composition and resolve middleware configuration outside the request path."}},
		},
		{
			ID:           "goforj-add-database-transaction/v1",
			Requirements: []WorkflowRequirement{{ID: "inspect-database-boundary", Kind: RequirementQuality, Capability: CapabilityFileReads, Description: "Inspect generated database access and keep the transaction boundary in the application service."}},
		},
		{
			ID:           "goforj-add-mail-workflow/v1",
			Requirements: []WorkflowRequirement{{ID: "inspect-mail-boundary", Kind: RequirementQuality, Capability: CapabilityFileReads, Description: "Inspect the generated mail manager and existing service boundary before adding delivery."}},
		},
		{
			ID:           "goforj-protect-route-with-auth/v1",
			Requirements: []WorkflowRequirement{{ID: "inspect-auth-route-groups", Kind: RequirementQuality, Capability: CapabilityFileReads, Description: "Inspect generated Auth route composition and reuse its existing middleware boundary."}},
		},
		promotedGeneratorWorkflow("goforj-add-named-app-route/v1", "generate-admin-audit-controller", "admin", "make:controller", "audits"),
		promotedGeneratorWorkflow("goforj-add-named-resource/v1", "generate-reports-queue", "make:queue", "reports", "--workers", "2"),
		promotedGeneratorWorkflow("goforj-add-named-cache/v1", "generate-profiles-cache", "generate", "--cache"),
		promotedGeneratorWorkflow("goforj-add-named-storage/v1", "generate-avatar-storage", "generate", "--storage"),
		promotedGeneratorWorkflow("goforj-build-json-api-feature/v1", "generate-users-controller", "make:controller", "users"),
		{
			ID:           "goforj-add-cached-repository/v1",
			Requirements: []WorkflowRequirement{{ID: "inspect-repository-boundary", Kind: RequirementQuality, Capability: CapabilityFileReads, Description: "Inspect the existing service and keep cache-aside behavior behind its repository contract."}},
		},
		promotedGeneratorWorkflow("goforj-add-upload-workflow/v1", "generate-uploads-controller", "make:controller", "uploads"),
		{
			ID:           "goforj-publish-domain-event/v1",
			Requirements: []WorkflowRequirement{{ID: "inspect-domain-boundary", Kind: RequirementQuality, Capability: CapabilityFileReads, Description: "Inspect the existing write workflow before publishing a typed domain fact through the App event bus."}},
		},
		promotedGeneratorWorkflow("goforj-dispatch-event-followup-job/v1", "generate-report-job", "make:job", "reports:generate", "--output-dir", "./internal/jobs"),
		promotedGeneratorWorkflow("goforj-schedule-existing-job/v1", "generate-report-schedule", "make:schedule", "reports:daily", "--every", "24h", "--no-open"),
		{
			ID:           "goforj-repair-wire-provider/v1",
			Requirements: []WorkflowRequirement{{ID: "inspect-wire-diagnostics", Kind: RequirementQuality, Capability: CapabilityFileReads, Description: "Inspect the Wire failure and owning provider set before editing."}},
		},
		{
			ID:           "goforj-clarify-unknown-shape/v1",
			Requirements: []WorkflowRequirement{{ID: "inspect-project", Kind: RequirementQuality, Capability: CapabilityFileReads, Description: "Inspect the existing application boundaries before requesting a decision."}},
		},
	}
}

// promotedGeneratorWorkflow keeps one-generator contracts concise without weakening exact argument matching.
func promotedGeneratorWorkflow(id, requirement string, arguments ...string) WorkflowExpectation {
	return WorkflowExpectation{
		ID:           id,
		Generators:   []GeneratorRequirement{{ID: requirement, Arguments: append([]string(nil), arguments...)}},
		Requirements: []WorkflowRequirement{{ID: "inspect-project", Kind: RequirementQuality, Capability: CapabilityFileReads, Description: "Inspect the owning App and existing application boundary before editing."}},
	}
}
