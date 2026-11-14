package workflows

// init registers focused application planning without introducing framework mutation tools.
func init() {
	registerApplicationWorkflow(applicationWorkflowDefinition{
		commands: []string{"{build}", "go test ./..."},
		id:       "goforj-request-validation", matches: requestValidationTask, docsTopic: validationDocsTopic,
		docs:     []DocReference{{Path: "applications/requests-validation.md", Heading: "Bind, Normalize, and Validate"}, {Path: "applications/library-selection.md", Heading: "Preserve Input Meaning"}, {Path: "developer-tools/wiring-recipes.md", Heading: "Quick Map"}},
		files:    []string{"internal/<domain>/request.go", "internal/<domain>/validation.go", "internal/<domain>/service.go", "internal/<domain>/controller.go", "{wire}/inject_services_app.go"},
		steps:    []PlanStep{{Action: "inspect existing binding and service rules", Purpose: "preserve transport behavior while sharing application validation"}, {Action: "normalize and validate presence-aware inputs before changing state", Purpose: "keep omitted, null, zero, and false distinct and operational errors separate from client issues"}},
		warnings: []string{"Reuse existing request and service boundaries; add a controller only when a new transport surface is explicitly requested."},
	})
}

// requestValidationTask requires the application concern rather than a generic provider or worker mention.
func requestValidationTask(task string) bool {
	return containsAny(task, "validation rules", "validated http endpoint", "validated write endpoint", "request validation", "payload validation", "validate input", "validate request", "validate payload") || (containsAny(task, "request", "payload", "json", "patch", "settings", "field") && containsAny(task, "validat", "omitted", "explicit null", "zero and false", "field errors"))
}

// validationDocsTopic makes documentation discovery useful without requiring an implementation request.
func validationDocsTopic(task string) bool {
	return containsAny(task, "validation", "validate request", "patch presence", "field errors")
}
