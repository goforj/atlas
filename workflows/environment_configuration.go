package workflows

// init registers focused application planning without introducing framework mutation tools.
func init() {
	registerApplicationWorkflow(applicationWorkflowDefinition{
		commands: []string{"{build}", "go test ./..."},
		id:       "goforj-configure-environment", matches: environmentConfigurationTask, docsTopic: environmentDocsTopic,
		docs:     []DocReference{{Path: "reference/configuration.md", Heading: "Environment File Resolution"}, {Path: "applications/library-selection.md", Heading: "Match the Application Requirement"}, {Path: "developer-tools/wiring-recipes.md", Heading: "Quick Map"}},
		files:    []string{"internal/<domain>/config.go", "internal/<domain>/config_test.go", "internal/<domain>/service.go", "{wire}/inject_services_app.go", "{composition}/lifecycle.go"},
		steps:    []PlanStep{{Action: "inspect installed environment loading and process/file precedence", Purpose: "reuse the existing startup boundary without overwriting deployment settings"}, {Action: "load and validate a typed configuration provider before readiness", Purpose: "inject one snapshot, reject invalid required values, and document optional fallbacks"}},
		warnings: []string{"Worker-count configuration does not require a queued job. Keep credentials out of diagnostics."},
	})
}

// environmentConfigurationTask requires the application concern rather than a generic provider or worker mention.
func environmentConfigurationTask(task string) bool {
	return containsAny(task, "typed startup config", "typed configuration", "typed config", "required startup values", "startup config", "startup settings", "startup worker count") || (containsAny(task, "environment", "env/v2", "env.") && containsAny(task, "config", "setting", "variable", "startup", "fallback", "precedence", "worker", "timeout"))
}

// environmentDocsTopic makes documentation discovery useful without requiring an implementation request.
func environmentDocsTopic(task string) bool {
	return containsAny(task, "configuration", "environment", "env/v2", "startup settings", "startup worker count")
}
