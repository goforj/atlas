package workflows

import "testing"

// TestPayloadEncryptionNaturalTasks covers implementation phrasing independently from the shared metadata fixtures.
func TestPayloadEncryptionNaturalTasks(t *testing.T) {
	for _, task := range []string{"Protect stored payloads with authenticated envelopes.", "Rotate APP_KEY while retaining previous keys for old records.", "Use an injected cipher to encrypt partner data."} {
		if got := Classify(task); got != "goforj-encrypt-payloads" {
			t.Errorf("Classify(%q) = %q", task, got)
		}
	}
}
