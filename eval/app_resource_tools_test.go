package eval

import (
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
)

// TestAppResourceToolsExecutesBoundedSelectedTools runs the generated helper against real child processes.
func TestAppResourceToolsExecutesBoundedSelectedTools(t *testing.T) {
	actualGo, err := exec.LookPath("go")
	if err != nil {
		t.Fatal(err)
	}
	root := t.TempDir()
	source := `package runtimeprobe
import (
 "context"
 "os"
 "os/exec"
 "path/filepath"
 "runtime"
 "strings"
 "testing"
 "time"
)
const atlasAppCommandSource = "package cmd"

// TestAtlasResourceToolChild emits controlled output as a real nested process.
func TestAtlasResourceToolChild(t *testing.T) {
 if os.Getenv("ATLAS_RESOURCE_CHILD") != "1" { return }
 if os.Getenv("ATLAS_RESOURCE_OVERFLOW") == "1" {
  os.Stdout.WriteString(strings.Repeat("o", 700000))
  os.Stderr.WriteString(strings.Repeat("e", 700000))
  os.Exit(1)
 }
 os.Stdout.WriteString("selected tool reached")
 os.Exit(0)
}

// TestAtlasResourceToolScenario exercises executable selection, fallback, and aggregate output limits.
func TestAtlasResourceToolScenario(t *testing.T) {
 executable, err := os.Executable()
 if err != nil { t.Fatal(err) }
 root := t.TempDir()
 t.Setenv("PATH", root)
 t.Setenv("ATLAS_VERIFIER_GO", executable)
 t.Setenv("ATLAS_VERIFIER_FORJ", executable)
 for _, tool := range []string{"go", "forj"} {
  output, err := atlasRunResourceTool(t, root, []string{"ATLAS_RESOURCE_CHILD=1"}, tool, "-test.run=^TestAtlasResourceToolChild$")
  if err != nil || !strings.Contains(output, "selected tool reached") { t.Fatalf("selected %s: %v: %s", tool, err, output) }
 }
 if os.Getenv("ATLAS_RESOURCE_OVERFLOW") == "1" { t.Fatal("overflow was accepted as a business error") }
 t.Setenv("ATLAS_VERIFIER_GO", "")
 if _, err := atlasRunResourceTool(t, root, nil, "go"); err == nil { t.Fatal("missing PATH executable accepted") }
 fallback := filepath.Join(root, "go")
 if runtime.GOOS == "windows" { fallback += ".exe" }
 body, err := os.ReadFile(executable)
 if err != nil { t.Fatal(err) }
 if err := os.WriteFile(fallback, body, 0700); err != nil { t.Fatal(err) }
 output, err := atlasRunResourceTool(t, root, []string{"ATLAS_RESOURCE_CHILD=1"}, "go", "-test.run=^TestAtlasResourceToolChild$")
 if err != nil || !strings.Contains(output, "selected tool reached") { t.Fatalf("PATH fallback: %v: %s", err, output) }
}
` + appResourceRuntimeHelpers
	for name, body := range map[string]string{"go.mod": "module example.test/runtimeprobe\n\ngo 1.26\n", "runtime_test.go": source} {
		if err := os.WriteFile(filepath.Join(root, name), []byte(body), 0600); err != nil {
			t.Fatal(err)
		}
	}
	binary := filepath.Join(root, "probe.test")
	if runtime.GOOS == "windows" {
		binary += ".exe"
	}
	build := exec.CommandContext(t.Context(), actualGo, "test", "-c", "-o", binary, ".")
	build.Dir = root
	if output, err := build.CombinedOutput(); err != nil {
		t.Fatalf("compile generated helper: %v\n%s", err, output)
	}
	for _, overflow := range []bool{false, true} {
		t.Run(map[bool]string{false: "selected-and-fallback", true: "aggregate-overflow"}[overflow], func(t *testing.T) {
			command := exec.CommandContext(t.Context(), binary, "-test.run=^TestAtlasResourceToolScenario$", "-test.timeout=15s")
			command.Env = append(os.Environ(), "ATLAS_RESOURCE_OVERFLOW=0")
			if overflow {
				command.Env = append(command.Env, "ATLAS_RESOURCE_OVERFLOW=1")
			}
			output, err := command.CombinedOutput()
			if overflow {
				if err == nil || !strings.Contains(string(output), "output exceeds 1048576 bytes") || strings.Contains(string(output), "overflow was accepted") {
					t.Fatalf("overflow result: %v\n%s", err, output)
				}
			} else if err != nil {
				t.Fatalf("helper execution: %v\n%s", err, output)
			}
		})
	}
}

// TestVerifierCommandsProtectsNestedToolSelection prevents caller environment metadata from replacing explicit tool choices.
func TestVerifierCommandsProtectsNestedToolSelection(t *testing.T) {
	selected := os.Args[0]
	runner := VerifierCommands{WorkRoot: t.TempDir(), GoExecutable: selected, ForjExecutable: selected, Environment: []string{"PATH=" + t.TempDir(), "ATLAS_VERIFIER_GO=/forged/go", "ATLAS_VERIFIER_FORJ=/forged/forj", "ATLAS_VERIFIER_GO=/also-forged/go"}}
	session, err := runner.Open(t.Context(), VerifierProject{Root: t.TempDir()})
	if err != nil {
		t.Fatal(err)
	}
	defer session.Close(t.Context())
	environment := session.(*verifierCommandSession).environment
	for _, name := range []string{"ATLAS_VERIFIER_GO", "ATLAS_VERIFIER_FORJ"} {
		count := 0
		for _, entry := range environment {
			if value, found := strings.CutPrefix(entry, name+"="); found {
				count++
				if value != selected {
					t.Fatalf("%s = %q, want %q", name, value, selected)
				}
			}
		}
		if count != 1 {
			t.Fatalf("%s occurs %d times", name, count)
		}
	}
}
