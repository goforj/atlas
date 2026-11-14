package workflows

// init keeps catalog resource work composable with independently registered application policies.
func init() {
	registerApplicationWorkflow(applicationWorkflowDefinition{
		id:        "goforj-catalog-search",
		matches:   catalogSearchTask,
		docsTopic: catalogSearchDocsTopic,
		docs:      WorkflowDocsMap["goforj-catalog-search"],
	})
}

// catalogSearchDocsTopic recognizes library concepts without interpreting documentation lookup as index creation.
func catalogSearchDocsTopic(task string) bool {
	return containsAny(task, "lexical search", "full-text search", "full text search", "catalog search", "product search", "search index", "search resource", "searchable catalog", "searchable product")
}
