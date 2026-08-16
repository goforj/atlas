package eval

import (
	"path/filepath"
	"testing"
)

// TestNewLocalGuidanceDiagnosticOwnsStandardWiring keeps hosts from rebuilding Atlas policy collaborators around every command.
func TestNewLocalGuidanceDiagnosticOwnsStandardWiring(t *testing.T) {
	_, _, preparer, _, _ := newFakeRunner(t)
	credential, err := NewCodexCredential([]byte(`{"access_token":"local-diagnostic-secret"}`))
	if err != nil {
		t.Fatal(err)
	}
	diagnostic, err := NewLocalGuidanceDiagnostic(LocalGuidanceDiagnosticOptions{
		WorkRoot:       t.TempDir(),
		ArtifactRoot:   filepath.Join(t.TempDir(), "artifacts"),
		ArtifactKey:    []byte("0123456789abcdef0123456789abcdef"),
		Preparer:       preparer,
		Codex:          CodexOptions{Executable: "codex", Model: "gpt-test", Credential: credential},
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
	if redacted := diagnostic.runner.Artifacts.redactor.Text("local-diagnostic-secret"); redacted != redactedValue {
		t.Fatalf("artifact redactor did not cover frozen authority: %q", redacted)
	}
}

// TestNewLocalGuidanceDiagnosticRejectsMissingHostBoundary prevents an incomplete host command from creating a partly trusted service.
func TestNewLocalGuidanceDiagnosticRejectsMissingHostBoundary(t *testing.T) {
	if _, err := NewLocalGuidanceDiagnostic(LocalGuidanceDiagnosticOptions{}); err == nil {
		t.Fatal("NewLocalGuidanceDiagnostic() accepted missing host boundaries")
	}
}
