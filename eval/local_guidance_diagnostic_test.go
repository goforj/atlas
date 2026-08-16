package eval

import (
	"path/filepath"
	"testing"
)

// TestNewLocalGuidanceDiagnosticOwnsStandardWiring keeps hosts from rebuilding Atlas policy collaborators around every command.
func TestNewLocalGuidanceDiagnosticOwnsStandardWiring(t *testing.T) {
	_, _, preparer, _, _ := newFakeRunner(t)
	diagnostic, err := NewLocalGuidanceDiagnostic(LocalGuidanceDiagnosticOptions{
		WorkRoot:       t.TempDir(),
		ArtifactRoot:   filepath.Join(t.TempDir(), "artifacts"),
		ArtifactKey:    []byte("0123456789abcdef0123456789abcdef"),
		Preparer:       preparer,
		Codex:          CodexOptions{Executable: "codex", Model: "gpt-test"},
		GoExecutable:   "go",
		ForjExecutable: "/tools/forj",
		Runtime:        fakeAttemptRequest().Runtime,
	})
	if err != nil {
		t.Fatalf("NewLocalGuidanceDiagnostic(): %v", err)
	}
	if diagnostic.runner.Registry == nil || diagnostic.runner.Preparer != preparer || diagnostic.runner.Agent == nil || diagnostic.runner.Artifacts == nil {
		t.Fatalf("diagnostic did not retain the standard Atlas wiring: %#v", diagnostic.runner)
	}
	if _, ok := diagnostic.runner.Backend.(UnconfinedLocal); !ok {
		t.Fatalf("backend = %T, want UnconfinedLocal", diagnostic.runner.Backend)
	}
}

// TestNewLocalGuidanceDiagnosticRejectsMissingHostBoundary prevents an incomplete host command from creating a partly trusted service.
func TestNewLocalGuidanceDiagnosticRejectsMissingHostBoundary(t *testing.T) {
	if _, err := NewLocalGuidanceDiagnostic(LocalGuidanceDiagnosticOptions{}); err == nil {
		t.Fatal("NewLocalGuidanceDiagnostic() accepted missing host boundaries")
	}
}
