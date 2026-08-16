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
		promotedGeneratorWorkflow("goforj-add-migration/v1", "generate-invoice-status-migration", "make:migration", "add_status_to_invoices", "--no-open"),
		promotedGeneratorWorkflow("goforj-add-schedule/v1", "generate-reconcile-schedule", "make:schedule", "invoices:reconcile", "--every", "1h"),
		{
			ID: "goforj-add-event-subscriber/v1",
			Generators: []GeneratorRequirement{
				{ID: "generate-invoice-paid-event", Arguments: []string{"make:event", "invoices:paid"}},
				{ID: "generate-invoice-paid-subscriber", Arguments: []string{"make:subscriber", "invoices:paid"}},
			},
		},
		promotedGeneratorWorkflow("goforj-create-model/v1", "generate-user-model", "make:model", "users"),
		promotedGeneratorWorkflow("goforj-add-named-app-route/v1", "generate-admin-audit-controller", "admin", "make:controller", "audits"),
		promotedGeneratorWorkflow("goforj-add-named-resource/v1", "generate-reports-queue", "make:queue", "reports", "--workers", "2"),
		promotedGeneratorWorkflow("goforj-add-named-cache/v1", "generate-profiles-cache", "generate", "--cache"),
		promotedGeneratorWorkflow("goforj-add-named-storage/v1", "generate-avatar-storage", "generate", "--storage"),
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
