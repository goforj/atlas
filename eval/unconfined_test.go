package eval

import (
	"context"
	"os"
	"path/filepath"
	"testing"
)

// preparedProjectFixture supplies the data-only Project boundary required by the backend.
type preparedProjectFixture struct {
	result PreparationResult
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
