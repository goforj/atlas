package eval

import (
	"context"
	"go/parser"
	"go/token"
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
