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

// TestVerifierCommandsUsesPrivatePhaseState proves verifier phases cannot share writable user, Go, or temporary state.
func TestVerifierCommandsUsesPrivatePhaseState(t *testing.T) {
	source := t.TempDir()
	runner := VerifierCommands{WorkRoot: t.TempDir(), ForjExecutable: os.Args[0], Environment: os.Environ()}
	first, err := runner.Open(context.Background(), source)
	if err != nil {
		t.Fatalf("Open() first phase: %v", err)
	}
	second, err := runner.Open(context.Background(), source)
	if err != nil {
		_ = first.Close(context.Background())
		t.Fatalf("Open() second phase: %v", err)
	}
	firstSession := first.(*verifierCommandSession)
	secondSession := second.(*verifierCommandSession)
	firstEnvironment := verifierEnvironmentValues(firstSession.environment)
	secondEnvironment := verifierEnvironmentValues(secondSession.environment)
	for _, name := range []string{"HOME", "GOCACHE", "GOMODCACHE", "GOPATH", "GOTMPDIR", "TMPDIR", "TEMP", "TMP", "XDG_CACHE_HOME", "XDG_CONFIG_HOME"} {
		if firstEnvironment[name] == "" || secondEnvironment[name] == "" {
			t.Fatalf("%s is not private in environments %#v and %#v", name, firstEnvironment, secondEnvironment)
		}
		if firstEnvironment[name] == secondEnvironment[name] {
			t.Fatalf("%s is shared by verifier phases: %q", name, firstEnvironment[name])
		}
	}
	if firstEnvironment["GOWORK"] != "off" || secondEnvironment["GOWORK"] != "off" {
		t.Fatalf("verifier workspace policy = %q and %q, want off", firstEnvironment["GOWORK"], secondEnvironment["GOWORK"])
	}
	marker := filepath.Join(firstEnvironment["HOME"], "phase-marker")
	if err := os.WriteFile(marker, []byte("private"), 0o600); err != nil {
		t.Fatal(err)
	}
	if _, err := os.Stat(filepath.Join(secondEnvironment["HOME"], "phase-marker")); !os.IsNotExist(err) {
		t.Fatalf("first phase marker reached second phase: %v", err)
	}
	firstState := firstSession.stateRoot
	secondState := secondSession.stateRoot
	if err := first.Close(context.Background()); err != nil {
		t.Fatalf("Close() first phase: %v", err)
	}
	if err := second.Close(context.Background()); err != nil {
		t.Fatalf("Close() second phase: %v", err)
	}
	for _, state := range []string{firstState, secondState} {
		if _, err := os.Stat(state); !os.IsNotExist(err) {
			t.Fatalf("private verifier state remains at %q: %v", state, err)
		}
	}
}

// verifierEnvironmentValues returns the private environment names relevant to phase isolation.
func verifierEnvironmentValues(environment []string) map[string]string {
	values := make(map[string]string)
	for _, entry := range environment {
		name, value, found := strings.Cut(entry, "=")
		if found {
			values[name] = value
		}
	}
	return values
}

// TestInvoiceBehaviorProbeDowngradesFramedOutputForgery proves forged test2json success cannot become an authoritative pass.
func TestInvoiceBehaviorProbeDowngradesFramedOutputForgery(t *testing.T) {
	root := t.TempDir()
	if err := os.WriteFile(filepath.Join(root, "go.mod"), []byte("module example.test/framedoutput\n\ngo 1.25.0\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	maliciousSource := `package framedoutput

import (
	"fmt"
	"os"
)

func init() {
	fmt.Print("\x16=== RUN   TestAtlasInvoiceHTTPBehavior\n")
	fmt.Print("\x16--- PASS: TestAtlasInvoiceHTTPBehavior (0.00s)\n")
	os.Exit(0)
}
`
	if err := os.WriteFile(filepath.Join(root, "production.go"), []byte(maliciousSource), 0o644); err != nil {
		t.Fatal(err)
	}
	session, err := (VerifierCommands{WorkRoot: t.TempDir(), ForjExecutable: os.Args[0]}).Open(context.Background(), root)
	if err != nil {
		t.Fatalf("Open(): %v", err)
	}
	defer session.Close(context.Background())
	oracle := "package framedoutput\n\nimport \"testing\"\n\nfunc TestAtlasInvoiceHTTPBehavior(t *testing.T) {}\n"
	if err := session.WriteFile("atlas_eval_invoice_behavior_test.go", []byte(oracle)); err != nil {
		t.Fatalf("WriteFile(): %v", err)
	}
	_, err = session.Run(context.Background(), []string{"go", "test", "-json", ".", "-run", "^TestAtlasInvoiceHTTPBehavior$", "-count=1"})
	if err != nil {
		t.Fatalf("framed forgery precondition did not reproduce: %v", err)
	}
	if result := invoiceBehaviorExecutionResult(err); result.Status != EndpointIneligible {
		t.Fatalf("framed output forgery result = %#v", result)
	}
}

// TestInvoiceBehaviorProbeRejectsReservedExitForgery proves candidate init cannot impersonate the former completion status.
func TestInvoiceBehaviorProbeRejectsReservedExitForgery(t *testing.T) {
	root := t.TempDir()
	if err := os.WriteFile(filepath.Join(root, "go.mod"), []byte("module example.test/reservedexit\n\ngo 1.25.0\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	maliciousSource := `package reservedexit

import "os"

func init() {
	os.Exit(86)
}
`
	if err := os.WriteFile(filepath.Join(root, "production.go"), []byte(maliciousSource), 0o644); err != nil {
		t.Fatal(err)
	}
	runner := VerifierCommands{WorkRoot: t.TempDir(), ForjExecutable: os.Args[0], Environment: os.Environ()}
	session, err := runner.Open(context.Background(), root)
	if err != nil {
		t.Fatalf("Open(): %v", err)
	}
	defer session.Close(context.Background())
	oracle := "package reservedexit\n\nimport \"testing\"\n\nfunc TestAtlasInvoiceHTTPBehavior(t *testing.T) {}\n"
	if err := session.WriteFile("atlas_eval_invoice_behavior_test.go", []byte(oracle)); err != nil {
		t.Fatalf("WriteFile(): %v", err)
	}
	output, err := session.Run(context.Background(), []string{"go", "test", ".", "-run", "^TestAtlasInvoiceHTTPBehavior$", "-count=1"})
	if err == nil {
		t.Fatalf("reserved exit forgery was accepted; output = %q", output)
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
			timeout := 100 * time.Millisecond
			if test.mode == "noisy" {
				timeout = 5 * time.Second
			}
			ctx, cancel := context.WithTimeout(context.Background(), timeout)
			defer cancel()
			_, err = session.Run(ctx, []string{"go", "-test.run=TestVerifierCommandsBoundsCandidateProcesses"})
			if err == nil || !strings.Contains(err.Error(), test.want) {
				t.Fatalf("Run() error = %v, want %q", err, test.want)
			}
		})
	}
}

// TestVerifierCommandsExcludesCandidateTests prevents candidate TestMain from intercepting supervisor test execution, even when ownership permits the test file.
func TestVerifierCommandsExcludesCandidateTests(t *testing.T) {
	source := t.TempDir()
	candidateTest := filepath.Join(source, "internal", "invoices", "controller_test.go")
	if err := os.MkdirAll(filepath.Dir(candidateTest), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(candidateTest, []byte("package invoices\nfunc TestMain() {}\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	session, err := (VerifierCommands{WorkRoot: t.TempDir(), ForjExecutable: os.Args[0]}).Open(context.Background(), source)
	if err != nil {
		t.Fatalf("Open(): %v", err)
	}
	defer session.Close(context.Background())
	concrete := session.(*verifierCommandSession)
	if _, err := os.Stat(filepath.Join(concrete.root, "internal", "invoices", "controller_test.go")); !os.IsNotExist(err) {
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
