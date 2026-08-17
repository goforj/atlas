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
	}
}
