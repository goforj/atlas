package docs

import (
	"context"
	"os"
	"os/exec"
	"path/filepath"
	"testing"
)

func TestGitProviderLoadsDocsFromCache(t *testing.T) {
	source := t.TempDir()
	runTestGit(t, source, "init", "-b", "main")

	docsRoot := filepath.Join(source, "docs")
	if err := os.MkdirAll(docsRoot, 0o755); err != nil {
		t.Fatalf("mkdir docs: %v", err)
	}
	if err := os.WriteFile(filepath.Join(docsRoot, "index.md"), []byte("# Cached Docs\n"), 0o644); err != nil {
		t.Fatalf("write docs: %v", err)
	}
	runTestGit(t, source, "add", ".")
	runTestGit(t, source, "-c", "user.name=Atlas Test", "-c", "user.email=atlas@example.test", "commit", "-m", "Add docs")

	provider := &GitProvider{
		CacheDir: filepath.Join(t.TempDir(), "cache"),
		Repo:     source,
		Ref:      "main",
		Version:  "test-version",
		Refresh:  true,
	}

	documents, err := provider.Documents(context.Background())
	if err != nil {
		t.Fatalf("documents: %v", err)
	}
	if len(documents) != 1 || documents[0].Title != "Cached Docs" {
		t.Fatalf("unexpected documents %#v", documents)
	}

	manifest, err := provider.Manifest(context.Background())
	if err != nil {
		t.Fatalf("manifest: %v", err)
	}
	if manifest.Version != "test-version" || manifest.Revision == "" {
		t.Fatalf("unexpected manifest %#v", manifest)
	}
}

func TestGitProviderFallsBackToNativeGit(t *testing.T) {
	source := t.TempDir()
	runTestGit(t, source, "init", "-b", "main")

	docsRoot := filepath.Join(source, "docs")
	if err := os.MkdirAll(docsRoot, 0o755); err != nil {
		t.Fatalf("mkdir docs: %v", err)
	}
	if err := os.WriteFile(filepath.Join(docsRoot, "index.md"), []byte("# Native Docs\n"), 0o644); err != nil {
		t.Fatalf("write docs: %v", err)
	}
	runTestGit(t, source, "add", ".")
	runTestGit(t, source, "-c", "user.name=Atlas Test", "-c", "user.email=atlas@example.test", "commit", "-m", "Add docs")

	provider := &GitProvider{
		CacheDir: filepath.Join(t.TempDir(), "cache"),
		Repo:     source,
		Ref:      "main",
		Version:  "test-version",
		Refresh:  true,
		git: fallbackGitClient{
			shell:  unavailableGitClient{},
			native: nativeGitClient{},
		},
	}

	documents, err := provider.Documents(context.Background())
	if err != nil {
		t.Fatalf("documents: %v", err)
	}
	if len(documents) != 1 || documents[0].Title != "Native Docs" {
		t.Fatalf("unexpected documents %#v", documents)
	}
}

type unavailableGitClient struct{}

func (unavailableGitClient) Clone(context.Context, string, string) error {
	return errGitUnavailable
}

func (unavailableGitClient) Fetch(context.Context, string) error {
	return errGitUnavailable
}

func (unavailableGitClient) Checkout(context.Context, string, string) error {
	return errGitUnavailable
}

func (unavailableGitClient) Pull(context.Context, string, string) error {
	return errGitUnavailable
}

func (unavailableGitClient) Revision(context.Context, string) (string, error) {
	return "", errGitUnavailable
}

func runTestGit(t *testing.T, dir string, args ...string) {
	t.Helper()
	cmd := exec.Command("git", args...)
	cmd.Dir = dir
	output, err := cmd.CombinedOutput()
	if err != nil {
		t.Fatalf("git %v: %v\n%s", args, err, output)
	}
}
