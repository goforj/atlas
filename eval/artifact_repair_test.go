package eval

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// TestRepairFinalizationArtifactsRejectsReplacedAttemptDirectory prevents recovery writes from crossing the original artifact identity boundary.
func TestRepairFinalizationArtifactsRejectsReplacedAttemptDirectory(t *testing.T) {
	root := filepath.Join(t.TempDir(), "artifacts")
	store, err := NewArtifactStore(root, []byte("0123456789abcdef0123456789abcdef"), NewRedactor(nil))
	if err != nil {
		t.Fatal(err)
	}
	artifacts, err := store.Begin("attempt-repair")
	if err != nil {
		t.Fatal(err)
	}
	if err := artifacts.WriteJSON("run.json", AttemptResult{EvaluationStatus: EvaluationValid}); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(artifacts.directory, artifactManifestName), []byte("occupied"), 0o600); err != nil {
		t.Fatal(err)
	}
	if _, err := artifacts.Finalize("sha256:plan", "sha256:baseline", "sha256:final"); err == nil {
		t.Fatal("Finalize() accepted a preexisting manifest")
	}
	original := artifacts.directory + "-original"
	if err := os.Rename(artifacts.directory, original); err != nil {
		t.Fatal(err)
	}
	if err := os.Mkdir(artifacts.directory, 0o700); err != nil {
		t.Fatal(err)
	}
	sentinel := []byte("replacement attempt must remain unchanged")
	if err := os.WriteFile(filepath.Join(artifacts.directory, "run.json"), sentinel, 0o600); err != nil {
		t.Fatal(err)
	}
	result := AttemptResult{EvaluationStatus: EvaluationEvaluatorError}
	repairFinalizationArtifacts(artifacts, fakeAttemptRequest(), &result)
	if !hasSecondaryFailurePhase(result.SecondaryFailures, "artifact_repair") {
		t.Fatalf("secondary failures = %#v, want artifact_repair", result.SecondaryFailures)
	}
	body, err := os.ReadFile(filepath.Join(artifacts.directory, "run.json"))
	if err != nil {
		t.Fatal(err)
	}
	if string(body) != string(sentinel) {
		t.Fatalf("replacement directory was modified: %q", body)
	}
	if strings.Contains(string(body), string(EvaluationEvaluatorError)) {
		t.Fatalf("terminal result crossed the attempt identity boundary: %q", body)
	}
}
