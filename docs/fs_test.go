package docs

import (
	"context"
	"os"
	"path/filepath"
	"testing"
)

func TestFSProviderLoadsMarkdown(t *testing.T) {
	root := t.TempDir()
	if err := os.WriteFile(filepath.Join(root, "index.md"), []byte("# Home\n\nWelcome."), 0o644); err != nil {
		t.Fatalf("write docs: %v", err)
	}
	if err := os.MkdirAll(filepath.Join(root, "nested"), 0o755); err != nil {
		t.Fatalf("mkdir docs: %v", err)
	}
	if err := os.WriteFile(filepath.Join(root, "nested", "apps.md"), []byte("# Apps\n\nUse app/."), 0o644); err != nil {
		t.Fatalf("write docs: %v", err)
	}

	provider := FSProvider{Root: root, Version: "0.18.0"}
	docs, err := provider.Documents(context.Background())
	if err != nil {
		t.Fatalf("load docs: %v", err)
	}
	if len(docs) != 2 {
		t.Fatalf("expected 2 docs, got %d", len(docs))
	}

	manifest, err := provider.Manifest(context.Background())
	if err != nil {
		t.Fatalf("manifest: %v", err)
	}
	if manifest.Version != "0.18.0" || manifest.Revision == "" {
		t.Fatalf("unexpected manifest %#v", manifest)
	}
}
