package workflows

// isBusinessMetricsTask recognizes instrumentation intent without redirecting runtime diagnostics.
func isBusinessMetricsTask(lower string) bool {
	return containsAny(lower, "business metrics", "business counter", "checkout metric", "checkout counter", "duration histogram", "instrument checkout") || containsAny(lower, "instrument", "increment", "record", "add") && containsAny(lower, "counter", "histogram")
}

// init registers focused instrumentation without replacing established workflow metadata.
func init() {
	registerApplicationWorkflow(applicationWorkflowDefinition{id: "goforj-business-metrics", matches: isBusinessMetricsTask, docsTopic: businessMetricsDocsTopic})
}

// businessMetricsDocsTopic keeps metrics documentation discovery independent from implementation verbs.
func businessMetricsDocsTopic(task string) bool {
	return containsAny(task, "metric", "counter", "histogram", "instrumentation", "prometheus")
}
