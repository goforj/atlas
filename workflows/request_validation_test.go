package workflows

import "testing"

// TestRequestValidationNaturalTasks covers implementation phrasing independently from the shared metadata fixtures.
func TestRequestValidationNaturalTasks(t *testing.T) {
	for _, task := range []string{"Normalize and validate incoming JSON requests before saving settings.", "Add reusable validation for customer settings.", "Distinguish omitted and null values in PATCH requests.", "Add a validated HTTP endpoint."} {
		if got := Classify(task); got != "goforj-request-validation" {
			t.Errorf("Classify(%q) = %q", task, got)
		}
	}
}
