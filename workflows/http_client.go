package workflows

// init registers outbound-client intent while preserving its established files and pin warnings.
func init() {
	registerApplicationWorkflow(applicationWorkflowDefinition{
		id: "goforj-http-client-resilience", matches: httpClientResilienceTask, docsTopic: httpClientDocsTopic,
		commands: []string{"{build}", "go test ./..."},
		docs:     []DocReference{{Path: "libraries/httpx.md", Heading: "Installation {#installation}"}, {Path: "libraries/httpx.md", Heading: "Retry {#retry-2}"}, {Path: "applications/library-selection.md", Heading: "Match the Application Requirement"}},
		steps:    []PlanStep{{Action: "inspect the installed HTTP client pin and owning service", Purpose: "preserve the Project's compatible dependency and caller context"}, {Action: "implement bounded retry and response policy in the existing outbound client", Purpose: "preserve non-idempotent behavior and verify actual request counts with a local server"}},
	})
}

// httpClientResilienceTask requires an HTTP concern rather than generic outbound mail or provider wording.
func httpClientResilienceTask(task string) bool {
	return containsAny(task, "outbound http", "http client", "httpx", "tax-rate", "tax rate")
}

// httpClientDocsTopic routes documentation lookup without turning client keywords into a controller request.
func httpClientDocsTopic(task string) bool {
	return httpClientResilienceTask(task) || containsAny(task, "http retries", "http retry", "remote http")
}
