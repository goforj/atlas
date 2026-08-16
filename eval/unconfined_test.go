package eval

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// preparedProjectFixture supplies the data-only Project boundary required by the backend.
type preparedProjectFixture struct {
	result PreparationResult
}

// TestUnconfinedLocalRejectsOversizedSealedTree prevents a diagnostic candidate from exhausting verifier storage.
func TestUnconfinedLocalRejectsOversizedSealedTree(t *testing.T) {
	projectRoot := t.TempDir()
	path := filepath.Join(projectRoot, "oversized.bin")
	file, err := os.Create(path)
	if err != nil {
		t.Fatal(err)
	}
	if err := file.Truncate(maxProjectTreeBytes + 1); err != nil {
		_ = file.Close()
		t.Fatal(err)
	}
	if err := file.Close(); err != nil {
		t.Fatal(err)
	}
	environment, err := (UnconfinedLocal{WorkRoot: t.TempDir()}).Open(context.Background(), BackendRequest{Project: preparedProjectFixture{result: PreparationResult{ProjectRoot: projectRoot}}})
	if err != nil {
		t.Fatal(err)
	}
	defer environment.Close(context.Background())
	if _, err := environment.Seal(context.Background()); err == nil || !strings.Contains(err.Error(), "exceeds") {
		t.Fatalf("Seal() error = %v, want project-size rejection", err)
	}
}

// Result returns the fixture Project identity.
func (project preparedProjectFixture) Result() PreparationResult {
	return project.result
}

// Close leaves fixture ownership with the test.
func (preparedProjectFixture) Close(context.Context) error {
	return nil
}

// TestUnconfinedLocalOwnsOnlyPrivateAgentState documents the diagnostic backend's deliberately narrow lifecycle.
func TestUnconfinedLocalOwnsOnlyPrivateAgentState(t *testing.T) {
	projectRoot := t.TempDir()
	backend := UnconfinedLocal{WorkRoot: t.TempDir()}
	capabilities, err := backend.Capabilities(context.Background())
	if err != nil || len(capabilities) != 0 {
		t.Fatalf("Capabilities() = %q, %v", capabilities, err)
	}
	environment, err := backend.Open(context.Background(), BackendRequest{
		Project: preparedProjectFixture{result: PreparationResult{ResolutionID: "fixture", ProjectRoot: projectRoot}},
	})
	if err != nil {
		t.Fatalf("Open(): %v", err)
	}
	paths := environment.Environment()
	if paths.ProjectRoot != projectRoot || filepath.Dir(paths.HomeRoot) != backend.WorkRoot {
		t.Fatalf("environment = %#v", paths)
	}
	if err := os.WriteFile(filepath.Join(projectRoot, "candidate.txt"), []byte("before seal"), 0o644); err != nil {
		t.Fatalf("write candidate: %v", err)
	}
	sealed, err := environment.Seal(context.Background())
	if err != nil {
		t.Fatalf("Seal(): %v", err)
	}
	if sealed.Root == projectRoot || sealed.TreeDigest == "" {
		t.Fatalf("sealed Project = %#v", sealed)
	}
	if err := os.WriteFile(filepath.Join(projectRoot, "candidate.txt"), []byte("after seal"), 0o644); err != nil {
		t.Fatalf("mutate candidate: %v", err)
	}
	sealedBody, err := os.ReadFile(filepath.Join(sealed.Root, "candidate.txt"))
	if err != nil || string(sealedBody) != "before seal" {
		t.Fatalf("sealed content = %q, %v", sealedBody, err)
	}
	if err := environment.Close(context.Background()); err != nil {
		t.Fatalf("Close(): %v", err)
	}
	if _, err := os.Stat(paths.HomeRoot); !os.IsNotExist(err) {
		t.Fatalf("private home remains after cleanup: %v", err)
	}
	if _, err := os.Stat(sealed.Root); !os.IsNotExist(err) {
		t.Fatalf("sealed Project remains after cleanup: %v", err)
	}
	if _, err := os.Stat(projectRoot); err != nil {
		t.Fatalf("backend removed preparer-owned Project: %v", err)
	}
}

// TestUnconfinedLocalSealPreservesCandidateTests keeps the sealed final identity independent of verifier-only filtering.
func TestUnconfinedLocalSealPreservesCandidateTests(t *testing.T) {
	projectRoot := t.TempDir()
	files := map[string]string{
		"internal/invoices/service_test.go":    "package invoices\n",
		"internal/invoices/controller_test.go": "package invoices\n",
	}
	for relative, body := range files {
		path := filepath.Join(projectRoot, relative)
		if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
			t.Fatalf("create test directory: %v", err)
		}
		if err := os.WriteFile(path, []byte(body), 0o644); err != nil {
			t.Fatalf("write candidate test: %v", err)
		}
	}
	environment, err := (UnconfinedLocal{WorkRoot: t.TempDir()}).Open(context.Background(), BackendRequest{Project: preparedProjectFixture{result: PreparationResult{ProjectRoot: projectRoot}}})
	if err != nil {
		t.Fatal(err)
	}
	defer environment.Close(context.Background())
	sealed, err := environment.Seal(context.Background())
	if err != nil {
		t.Fatalf("Seal(): %v", err)
	}
	for relative, want := range files {
		body, err := os.ReadFile(filepath.Join(sealed.Root, relative))
		if err != nil || string(body) != want {
			t.Fatalf("sealed %s = %q, %v", relative, body, err)
		}
	}
	wantDigest, err := digestProjectTree(projectRoot)
	if err != nil {
		t.Fatalf("digest candidate: %v", err)
	}
	if sealed.TreeDigest != wantDigest {
		t.Fatalf("sealed digest = %s, want %s", sealed.TreeDigest, wantDigest)
	}
}
