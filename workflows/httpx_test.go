package workflows

import (
	"slices"
	"strings"
	"testing"
)

// TestHTTPClientResilienceWorkflowPreservesPins keeps outbound policy separate from route scaffolding.
func TestHTTPClientResilienceWorkflowPreservesPins(t *testing.T) {
	task := "harden the outbound tax-rate client"
	if Classify(task) != "goforj-http-client-resilience" || !slices.Contains(ClassifyAll(task), "goforj-http-client-resilience") {
		t.Fatal("wrong client workflow")
	}
	if !strings.Contains(strings.Join(workflowWarnings("goforj-http-client-resilience"), " "), "do not blindly upgrade") {
		t.Fatal("missing Project pin warning")
	}
	if slices.Contains(ClassifyAll("send outbound mail to a customer"), "goforj-http-client-resilience") {
		t.Fatal("mail was classified as an HTTP client task")
	}
	if len(workflowDocs("goforj-http-client-resilience")) < 2 {
		t.Fatal("missing API docs")
	}
}
