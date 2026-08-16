package eval

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

// TestVerifierCommandsUsesPrivateCloneAndAllowlistedTools proves candidate evidence remains unchanged by executable checks.
func TestVerifierCommandsUsesPrivateCloneAndAllowlistedTools(t *testing.T) {
	source := t.TempDir()
	if err := os.WriteFile(filepath.Join(source, "go.mod"), []byte("module example.test/fixture\n\ngo 1.25\n"), 0o644); err != nil {
		t.Fatalf("write go.mod: %v", err)
	}
	if err := os.WriteFile(filepath.Join(source, "fixture_test.go"), []byte("package fixture\nimport \"testing\"\nfunc TestFixture(t *testing.T) {}\n"), 0o644); err != nil {
		t.Fatalf("write test: %v", err)
	}
	runner := VerifierCommands{WorkRoot: t.TempDir(), ForjExecutable: os.Args[0], Environment: os.Environ()}
	session, err := runner.Open(context.Background(), source)
	if err != nil {
		t.Fatalf("Open(): %v", err)
	}
	if err := session.WriteFile("oracle_test.go", []byte("package fixture\n")); err != nil {
		t.Fatalf("WriteFile(): %v", err)
	}
	if _, err := session.Run(context.Background(), []string{"go", "test", "./..."}); err != nil {
		t.Fatalf("Run(): %v", err)
	}
	if _, err := session.Run(context.Background(), []string{"sh", "-c", "touch escaped"}); err == nil || !strings.Contains(err.Error(), "not allowlisted") {
		t.Fatalf("unallowlisted command error = %v", err)
	}
	if err := session.Close(context.Background()); err != nil {
		t.Fatalf("Close(): %v", err)
	}
	if _, err := os.Stat(filepath.Join(source, "escaped")); !os.IsNotExist(err) {
		t.Fatalf("verifier mutated source evidence: %v", err)
	}
	if _, err := os.Stat(filepath.Join(source, "oracle_test.go")); !os.IsNotExist(err) {
		t.Fatalf("supervisor oracle leaked into source evidence: %v", err)
	}
}

// TestVerifierCommandsBoundsCandidateProcesses proves a noisy or stalled candidate cannot retain verifier resources.
func TestVerifierCommandsBoundsCandidateProcesses(t *testing.T) {
	if os.Getenv("ATLAS_VERIFIER_HELPER") == "1" {
		if os.Getenv("ATLAS_VERIFIER_HELPER_MODE") == "noisy" {
			_, _ = fmt.Fprint(os.Stdout, strings.Repeat("x", maxVerifierOutput+1))
			return
		}
		time.Sleep(5 * time.Second)
		return
	}
	source := t.TempDir()
	for _, test := range []struct {
		name string
		mode string
		want string
	}{
		{name: "output", mode: "noisy", want: "output exceeds"},
		{name: "deadline", mode: "stalled", want: "deadline exceeded"},
	} {
		t.Run(test.name, func(t *testing.T) {
			runner := VerifierCommands{WorkRoot: t.TempDir(), GoExecutable: os.Args[0], ForjExecutable: os.Args[0], Environment: append(os.Environ(), "ATLAS_VERIFIER_HELPER=1", "ATLAS_VERIFIER_HELPER_MODE="+test.mode)}
			session, err := runner.Open(context.Background(), source)
			if err != nil {
				t.Fatalf("Open(): %v", err)
			}
			defer session.Close(context.Background())
			ctx, cancel := context.WithTimeout(context.Background(), 100*time.Millisecond)
			defer cancel()
			_, err = session.Run(ctx, []string{"go", "-test.run=TestVerifierCommandsBoundsCandidateProcesses"})
			if err == nil || !strings.Contains(err.Error(), test.want) {
				t.Fatalf("Run() error = %v, want %q", err, test.want)
			}
		})
	}
}

// TestVerifierCommandsExcludesCandidateTests prevents candidate TestMain from intercepting supervisor test execution.
func TestVerifierCommandsExcludesCandidateTests(t *testing.T) {
	source := t.TempDir()
	if err := os.WriteFile(filepath.Join(source, "candidate_test.go"), []byte("package fixture\nfunc TestMain() {}\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	session, err := (VerifierCommands{WorkRoot: t.TempDir(), ForjExecutable: os.Args[0]}).Open(context.Background(), source)
	if err != nil {
		t.Fatalf("Open(): %v", err)
	}
	defer session.Close(context.Background())
	concrete := session.(*verifierCommandSession)
	if _, err := os.Stat(filepath.Join(concrete.root, "candidate_test.go")); !os.IsNotExist(err) {
		t.Fatalf("candidate test copied into verifier clone: %v", err)
	}
}

// TestVerifierCommandsRestrictsSupervisorFiles rejects traversal, symlink aliases, and candidate collisions.
func TestVerifierCommandsRestrictsSupervisorFiles(t *testing.T) {
	source := t.TempDir()
	if err := os.Mkdir(filepath.Join(source, "safe"), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(source, "safe", "existing.go"), []byte("package safe\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.Symlink("safe", filepath.Join(source, "alias")); err != nil {
		t.Fatal(err)
	}
	session, err := (VerifierCommands{WorkRoot: t.TempDir(), ForjExecutable: os.Args[0]}).Open(context.Background(), source)
	if err != nil {
		t.Fatalf("Open(): %v", err)
	}
	defer func() {
		if err := session.Close(context.Background()); err != nil {
			t.Errorf("Close(): %v", err)
		}
	}()
	for _, path := range []string{"../escape.go", "alias/oracle.go", "safe/existing.go", "missing/oracle.go"} {
		if err := session.WriteFile(path, []byte("package safe\n")); err == nil {
			t.Fatalf("WriteFile(%q) unexpectedly succeeded", path)
		}
	}
	if err := session.WriteFile("safe/oracle.go", []byte("package safe\n")); err != nil {
		t.Fatalf("WriteFile(safe/oracle.go): %v", err)
	}
}

// TestVerifierCommandsRejectsEscapingSymlink keeps candidate-controlled links out of executable verifier clones.
func TestVerifierCommandsRejectsEscapingSymlink(t *testing.T) {
	source := t.TempDir()
	if err := os.Symlink(filepath.Join(t.TempDir(), "outside"), filepath.Join(source, "escape")); err != nil {
		t.Fatalf("create symlink: %v", err)
	}
	_, err := (VerifierCommands{WorkRoot: t.TempDir(), ForjExecutable: os.Args[0]}).Open(context.Background(), source)
	if err == nil || !strings.Contains(err.Error(), "escapes") {
		t.Fatalf("Open() error = %v, want escaping symlink rejection", err)
	}
}
