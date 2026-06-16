package docs

import (
	"context"
	"testing"
)

func TestCachingProviderLoadsDocumentsOnce(t *testing.T) {
	source := &countingProvider{
		manifest: Manifest{Version: "test", Revision: "rev1"},
		documents: []Document{
			{Path: "index.md", Title: "Home", Content: "# Home\n"},
		},
	}
	provider := &CachingProvider{Provider: source}

	for i := 0; i < 3; i++ {
		documents, err := provider.Documents(context.Background())
		if err != nil {
			t.Fatalf("documents: %v", err)
		}
		if len(documents) != 1 {
			t.Fatalf("expected one document, got %d", len(documents))
		}
	}
	if source.documentCalls != 1 {
		t.Fatalf("expected one source document load, got %d", source.documentCalls)
	}
}

type countingProvider struct {
	manifest      Manifest
	documents     []Document
	manifestCalls int
	documentCalls int
}

func (p *countingProvider) Manifest(context.Context) (Manifest, error) {
	p.manifestCalls++
	return p.manifest, nil
}

func (p *countingProvider) Documents(context.Context) ([]Document, error) {
	p.documentCalls++
	return p.documents, nil
}
