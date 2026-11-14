package workflows

// init registers focused application planning without introducing framework mutation tools.
func init() {
	registerApplicationWorkflow(applicationWorkflowDefinition{
		commands: []string{"{build}", "go test ./..."},
		id:       "goforj-encrypt-payloads", matches: payloadEncryptionTask, docsTopic: encryptionDocsTopic,
		docs:     []DocReference{{Path: "libraries/crypt.md", Heading: "Key format & rotation {#key-format-&-rotation}"}, {Path: "applications/library-selection.md", Heading: "Match the Application Requirement"}, {Path: "developer-tools/wiring-recipes.md", Heading: "Quick Map"}},
		files:    []string{"internal/<domain>/cipher.go", "internal/<domain>/service.go", "internal/<domain>/service_test.go", "{wire}/inject_services_app.go", "{composition}/lifecycle.go"},
		steps:    []PlanStep{{Action: "inspect the persisted payload format and configured current/previous key contract", Purpose: "preserve compatibility without inventing a secret manager"}, {Action: "compose one cipher provider and protect the existing service storage boundary", Purpose: "authenticate before returning plaintext, retain previous-key reads, and use only the current key for new writes"}},
		warnings: []string{"Use fake keys to test tampering and rotation; retaining old keys does not rewrite existing records."},
	})
}

// payloadEncryptionTask requires the application concern rather than a generic provider or worker mention.
func payloadEncryptionTask(task string) bool {
	return containsAny(task, "authenticated envelope", "payload authentication", "encrypt", "decrypt", "payload cipher", "crypt.new", "key rotation", "previous keys", "app_previous_keys", "app_key")
}

// encryptionDocsTopic makes documentation discovery useful without requiring an implementation request.
func encryptionDocsTopic(task string) bool {
	return containsAny(task, "crypt", "encrypt", "decrypt", "cipher", "key rotation", "previous keys")
}
