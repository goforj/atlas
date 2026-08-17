package eval

import (
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"testing"
)

// TestBuildProjectDiffExplainsTextAndMetadataChanges verifies the retained patch covers the reviewable candidate delta.
func TestBuildProjectDiffExplainsTextAndMetadataChanges(t *testing.T) {
	baseline := t.TempDir()
	final := t.TempDir()
	writeDiffFixture(t, baseline, "modified.txt", "before\n")
	writeDiffFixture(t, baseline, "removed.txt", "removed\n")
	writeDiffFixture(t, final, "modified.txt", "after\n")
	writeDiffFixture(t, final, "added.txt", "added\n")

	diff, err := buildFixtureProjectDiff(baseline, final)
	if err != nil {
		t.Fatalf("buildProjectDiff returned error: %v", err)
	}
	for _, want := range []string{
		`diff --git "a/added.txt" "b/added.txt"`,
		`--- /dev/null`,
		`+added`,
		`-before`,
		`+after`,
		`-removed`,
		`+++ /dev/null`,
	} {
		if !strings.Contains(diff, want) {
			t.Fatalf("diff missing %q:\n%s", want, diff)
		}
	}
}

// TestBuildProjectDiffDoesNotRetainBinaryContent verifies candidate binary payloads remain outside diagnostic artifacts.
func TestBuildProjectDiffDoesNotRetainBinaryContent(t *testing.T) {
	baseline := t.TempDir()
	final := t.TempDir()
	writeDiffFixture(t, baseline, "asset.bin", string([]byte{'s', 'e', 'c', 'r', 'e', 't', 0, 'a'}))
	writeDiffFixture(t, final, "asset.bin", string([]byte{'s', 'e', 'c', 'r', 'e', 't', 0, 'b'}))

	diff, err := buildFixtureProjectDiff(baseline, final)
	if err != nil {
		t.Fatalf("buildProjectDiff returned error: %v", err)
	}
	if !strings.Contains(diff, "Binary or oversized files") {
		t.Fatalf("diff did not classify binary content:\n%s", diff)
	}
	if strings.Contains(diff, "secret") {
		t.Fatalf("diff retained binary payload:\n%s", diff)
	}
}

// TestBuildProjectDiffReturnsEmptyForIdenticalTrees verifies unchanged attempts do not gain synthetic noise.
func TestBuildProjectDiffReturnsEmptyForIdenticalTrees(t *testing.T) {
	baseline := t.TempDir()
	final := t.TempDir()
	writeDiffFixture(t, baseline, "same.txt", "same\n")
	writeDiffFixture(t, final, "same.txt", "same\n")

	diff, err := buildFixtureProjectDiff(baseline, final)
	if err != nil {
		t.Fatalf("buildProjectDiff returned error: %v", err)
	}
	if diff != "" {
		t.Fatalf("identical diff = %q, want empty", diff)
	}
}

// TestBuildProjectDiffUsesPreMutationSnapshot proves the writable candidate cannot erase its own retained delta.
func TestBuildProjectDiffUsesPreMutationSnapshot(t *testing.T) {
	project := t.TempDir()
	writeDiffFixture(t, project, "controller.go", "package invoice\n")
	baseline, _, err := snapshotProjectForDiff(project)
	if err != nil {
		t.Fatalf("snapshotProjectForDiff returned error: %v", err)
	}
	writeDiffFixture(t, project, "controller.go", "package invoices\n")

	diff, err := buildProjectDiff(baseline, project)
	if err != nil {
		t.Fatalf("buildProjectDiff returned error: %v", err)
	}
	if !strings.Contains(diff, "-package invoice") || !strings.Contains(diff, "+package invoices") {
		t.Fatalf("diff did not retain pre-mutation state:\n%s", diff)
	}
}

// TestSnapshotProjectForDiffUsesGlobalLexicalOrder keeps Atlas tree identities compatible with the GoForj preparation contract.
func TestSnapshotProjectForDiffUsesGlobalLexicalOrder(t *testing.T) {
	project := t.TempDir()
	writeDiffFixture(t, project, "thing/item.txt", "nested\n")
	writeDiffFixture(t, project, "thing.go", "sibling\n")

	_, digest, err := snapshotProjectForDiff(project)
	if err != nil {
		t.Fatalf("snapshotProjectForDiff returned error: %v", err)
	}
	const want = "sha256:b62002c7b628f1c472ee29a35de6908fff814cefcc1ae323cc3952f7762075e4"
	if digest != want {
		t.Fatalf("tree digest = %q, want GoForj-compatible %q", digest, want)
	}
}

// TestSnapshotProjectForDiffRejectsOversizedTrees bounds candidate-controlled hashing before reading sparse content.
func TestSnapshotProjectForDiffRejectsOversizedTrees(t *testing.T) {
	project := t.TempDir()
	path := filepath.Join(project, "oversized.bin")
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
	if _, _, err := snapshotProjectForDiff(project); err == nil || !strings.Contains(err.Error(), "exceeds") {
		t.Fatalf("snapshotProjectForDiff() error = %v, want project-size rejection", err)
	}
}

// TestSnapshotProjectForDiffReservesTrustedTestContent keeps established tests available even after ordinary diff retention is exhausted.
func TestSnapshotProjectForDiffReservesTrustedTestContent(t *testing.T) {
	project := t.TempDir()
	large := strings.Repeat("x", maxDiffSourceFileSize)
	for index := 0; index < maxDiffRetainedContentSize/maxDiffSourceFileSize; index++ {
		writeDiffFixture(t, project, "ordinary-"+strconv.Itoa(index)+".txt", large)
	}
	writeDiffFixture(t, project, "z_feature_test.go", "package feature\n")
	snapshot, _, err := snapshotProjectForDiff(project)
	if err != nil {
		t.Fatalf("snapshotProjectForDiff returned error: %v", err)
	}
	tests, err := trustedTestsFromSnapshot(snapshot)
	if err != nil {
		t.Fatalf("trustedTestsFromSnapshot returned error: %v", err)
	}
	if len(tests) != 1 || tests[0].Path != "z_feature_test.go" || string(tests[0].Body) != "package feature\n" {
		t.Fatalf("trusted tests = %#v", tests)
	}
}

// buildFixtureProjectDiff captures the same pre-mutation boundary used by the runner.
func buildFixtureProjectDiff(baseline, final string) (string, error) {
	snapshot, _, err := snapshotProjectForDiff(baseline)
	if err != nil {
		return "", err
	}
	return buildProjectDiff(snapshot, final)
}

// writeDiffFixture creates one ordinary source file for deterministic diff tests.
func writeDiffFixture(t *testing.T, root, relative, body string) {
	t.Helper()
	path := filepath.Join(root, relative)
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatalf("create fixture directory: %v", err)
	}
	if err := os.WriteFile(path, []byte(body), 0o644); err != nil {
		t.Fatalf("write fixture: %v", err)
	}
}
