package workflows

import "strings"

// init registers focused application planning without introducing framework mutation tools.
func init() {
	registerApplicationWorkflow(applicationWorkflowDefinition{
		commands: []string{"{build}", "go test ./..."},
		id:       "goforj-read-bound-secrets", matches: boundSecretReadTask, docsTopic: secretDocsTopic,
		docs:     []DocReference{{Path: "libraries/secret.md", Heading: "Quick Start {#quick-start}"}, {Path: "applications/library-selection.md", Heading: "Match the Application Requirement"}, {Path: "core/wiring-recipes.md", Heading: "Quick Map"}},
		files:    []string{"internal/<domain>/service.go", "internal/<domain>/service_test.go", "{wire}/inject_services_app.go"},
		steps:    []PlanStep{{Action: "inspect the configured provider binding and available root/driver releases", Purpose: "do not invent a dependency version or replace existing secret lookup with environment parsing or encryption"}, {Action: "inject the bound Reader into the application service", Purpose: "preserve exact values, caller context, classified failures, and safe diagnostics"}},
		warnings: []string{"Confirm root and selected driver publication before adding dependencies. A bound Reader is not authorization to rotate, replace, or delete secrets."},
	})
}

// boundSecretReadTask requires the application concern rather than a generic provider or worker mention.
func boundSecretReadTask(task string) bool {
	return containsAny(task, "secret.reader", "bound secret", "named secret", "managed secret", "secret provider", "secret store") || (strings.Contains(task, "credential") && containsAny(task, "provider", "secret", "binding", "bound reader")) || (strings.Contains(task, "secret") && containsAny(task, "reader", "lookup", "fetch", "resolve", "bind"))
}

// secretDocsTopic makes documentation discovery useful without requiring an implementation request.
func secretDocsTopic(task string) bool {
	return containsAny(task, "secret", "managed credential", "credential provider")
}
