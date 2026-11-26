package eval

import (
	"context"
	"go/parser"
	"go/token"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
)

// TestTenantRateLimitLibraryUseIsQuality prevents library preference from failing correct request behavior.
func TestTenantRateLimitLibraryUseIsQuality(t *testing.T) {
	for _, fail := range []bool{false, true} {
		runner := &fakeCommandRunner{}
		if fail {
			runner.failCommand = "test"
		}
		result := runTenantRateLimitLibraryProbe(context.Background(), runner, VerifierProject{})
		want := EndpointPassed
		if fail {
			want = EndpointFailed
		}
		if result.Kind != RequirementQuality || result.Status != want {
			t.Fatalf("quality result: %#v", result)
		}
		if outcome := summarizeSurfaceChecks("tenant-request-rate-limits/v1", []EndpointResult{result}); outcome.Status != EndpointPassed {
			t.Fatalf("quality changed outcome: %#v", outcome)
		}
	}
}

// TestTenantRateLimitProbeAndManifestStayAddressable checks sealed source syntax and independent catalog registration.
func TestTenantRateLimitProbeAndManifestStayAddressable(t *testing.T) {
	for _, file := range tenantRateLimitSupervisorFiles() {
		if _, err := parser.ParseFile(token.NewFileSet(), file.path, file.body, parser.ParseComments); err != nil {
			t.Fatal(err)
		}
	}
	definition, err := LoadPromotedDefinition("tenant-request-rate-limits")
	if err != nil {
		t.Fatal(err)
	}
	if definition.ProjectScenario != "tenant-request-rate-limits" || definition.VerifierID != "tenant-request-rate-limits/v1" {
		t.Fatalf("manifest: %#v", definition)
	}
}

// TestTenantRateLimitNativeEndpointRequiresLoopback keeps native backend access supervisor-owned and local.
func TestTenantRateLimitNativeEndpointRequiresLoopback(t *testing.T) {
	for _, address := range []string{"", "redis.example:6379", "192.0.2.1:6379", "127.0.0.1:0", "127.0.0.1:65536", "127.0.0.1:no", "127.0.0.1:6379", "[::1]:6379"} {
		t.Run(address, func(t *testing.T) {
			t.Setenv("ATLAS_EVAL_REDIS_ADDR", address)
			contract := tenantRateLimitAppRuntimeContract()
			source := contract.supervisorFiles[0].body
			accepted := address == "127.0.0.1:6379" || address == "[::1]:6379"
			if strings.Contains(source, `const atlasRedisAddress = ""`) == accepted {
				t.Fatalf("unexpected endpoint acceptance: %q", address)
			}
			if _, err := parser.ParseFile(token.NewFileSet(), "app_test.go", source, parser.ParseComments); err != nil {
				t.Fatal(err)
			}
			if _, err := parser.ParseFile(token.NewFileSet(), "command.go", tenantRateLimitAppRuntimeCommand, parser.ParseComments); err != nil {
				t.Fatal(err)
			}
		})
	}
}

// TestTenantNativeAppOutputBounds executes the exact supervisor sink at its boundary and under concurrent overflow.
func TestTenantNativeAppOutputBounds(t *testing.T) {
	actualGo, err := exec.LookPath("go")
	if err != nil {
		t.Fatal(err)
	}
	root := t.TempDir()
	source := `package outputprobe
import ("context";"strings";"sync")
` + tenantNativeAppOutputSource
	tests := `package outputprobe
import ("strings";"sync";"sync/atomic";"testing")
// TestOutputBoundary distinguishes the permitted ceiling from the first excess byte.
func TestOutputBoundary(t *testing.T) {
 var canceled atomic.Int32
 output:=&atlasNativeAppOutput{cancel:func(){canceled.Add(1)}}
 if n,err:=output.Write([]byte(strings.Repeat("x",1<<20)));err!=nil || n!=1<<20 {t.Fatal(n,err)}
 if body,exceeded:=output.snapshot();len(body)!=1<<20 || exceeded || canceled.Load()!=0 {t.Fatal("boundary rejected")}
 if n,err:=output.Write([]byte("overflow"));err!=nil || n!=8 {t.Fatal(n,err)}
 if body,exceeded:=output.snapshot();len(body)!=1<<20 || !exceeded || canceled.Load()!=1 {t.Fatal("overflow not bounded and canceled")}
}
// TestConcurrentOutputAndSnapshots exercises the shared stdout and stderr ceiling during readiness inspection.
func TestConcurrentOutputAndSnapshots(t *testing.T) {
 var canceled atomic.Int32
 output:=&atlasNativeAppOutput{cancel:func(){canceled.Add(1)}}
 var workers sync.WaitGroup
 for index:=0;index<32;index++ {
  workers.Add(1)
  go func(){defer workers.Done();output.Write([]byte(strings.Repeat("x",65536)));output.snapshot()}()
 }
 workers.Wait()
 if body,exceeded:=output.snapshot();len(body)!=1<<20 || !exceeded || canceled.Load()==0 {t.Fatal("concurrent output escaped ceiling")}
}
`
	for name, body := range map[string]string{"go.mod": "module example.test/outputprobe\n\ngo 1.26\n", "output.go": source, "output_test.go": tests} {
		if err := os.WriteFile(filepath.Join(root, name), []byte(body), 0600); err != nil {
			t.Fatal(err)
		}
	}
	command := exec.CommandContext(t.Context(), actualGo, "test", "-count=1", ".")
	command.Dir = root
	if output, err := command.CombinedOutput(); err != nil {
		t.Fatalf("generated output sink: %v\n%s", err, output)
	}
}

// TestTenantRateLimitOwnsAppProvidersAndTheirTests keeps route policy composition valid without permitting unrelated runtime changes.
func TestTenantRateLimitOwnsAppProvidersAndTheirTests(t *testing.T) {
	var contract surfaceContract
	for _, candidate := range promotedSurfaceContracts() {
		if candidate.id == "tenant-request-rate-limits/v1" {
			contract = candidate
			break
		}
	}
	patterns := append(append([]string(nil), contract.allowedChanges...), contract.qualityTestPatterns...)
	for _, test := range []struct {
		path   string
		status EndpointStatus
	}{
		{"app/wire/inject_http_controllers_app.go", EndpointPassed},
		{"app/wire/requests_test.go", EndpointPassed},
		{"internal/requests/limiter_test.go", EndpointPassed},
		{"app/routes.go", EndpointFailed},
		{".env", EndpointFailed},
		{"internal/caches/manager.go", EndpointFailed},
	} {
		t.Run(test.path, func(t *testing.T) {
			result := verifySurfaceOwnership([]ProjectChange{{Path: test.path, After: ProjectPathState{Kind: "file"}}}, patterns)
			if result.Status != test.status {
				t.Fatalf("ownership %s=%#v, want %s", test.path, result, test.status)
			}
		})
	}
}
