package workflows

import (
	"context"
	"testing"

	"github.com/goforj/atlas/docs"
)

// TestWorkflowReferencesAgainstDocsCheckout verifies real section extraction when a documentation checkout is supplied.
func TestWorkflowReferencesAgainstDocsCheckout(t *testing.T) {
	provider, configured := docs.ProviderFromEnv("")
	if !configured {
		t.Skip("set GOFORJ_DOCS_PATH to validate against a documentation checkout")
	}
	documents, err := provider.Documents(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	corpus := docs.StaticProvider{Docs: documents}
	check := func(t *testing.T, ref DocReference) {
		t.Helper()
		section, found, err := docs.ReadSection(context.Background(), corpus, ref.Path, ref.Heading, 0)
		if err != nil || !found || section.Body == "" {
			t.Errorf("unavailable section %s: %s (found=%v, err=%v)", ref.Path, ref.Heading, found, err)
		}
	}
	for id, refs := range WorkflowDocsMap {
		t.Run(id, func(t *testing.T) {
			for _, ref := range refs {
				check(t, ref)
			}
		})
	}
	for _, ref := range workflowDocs("unknown-workflow") {
		check(t, ref)
	}
	ids := make([]string, 0, len(WorkflowDocsMap))
	for id := range WorkflowDocsMap {
		ids = append(ids, id)
	}
	for _, ref := range workflowScenarioRefs(ids) {
		check(t, ref)
	}
}
