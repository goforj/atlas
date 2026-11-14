package eval

import (
	"context"
	"go/parser"
	"go/token"
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
