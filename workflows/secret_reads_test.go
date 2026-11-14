package workflows

import "testing"

// TestBoundSecretReadNaturalTasks covers implementation phrasing independently from the shared metadata fixtures.
func TestBoundSecretReadNaturalTasks(t *testing.T) {
	for _, task := range []string{"Look up an API credential using a configured provider.", "Fetch API credentials through a configured provider.", "Bind a secret.Reader to the service.", "Resolve a managed secret without logging its value."} {
		if got := Classify(task); got != "goforj-read-bound-secrets" {
			t.Errorf("Classify(%q) = %q", task, got)
		}
	}
}
