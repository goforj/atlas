package eval

import (
	"context"
	"testing"
)

// TestCatalogSearchLibraryUseIsQuality separates executed library preference from correct application behavior.
func TestCatalogSearchLibraryUseIsQuality(t *testing.T) {
	for _, fail := range []bool{false, true} {
		runner := &fakeCommandRunner{}
		if fail {
			runner.failCommand = "test"
		}
		result := runCatalogSearchLibraryUseProbe(context.Background(), runner, VerifierProject{})
		want := EndpointPassed
		if fail {
			want = EndpointFailed
		}
		if result.Kind != RequirementQuality || result.Status != want {
			t.Fatalf("adoption result = %#v", result)
		}
		if outcome := summarizeSurfaceChecks("catalog-search/v1", []EndpointResult{result}); outcome.Status != EndpointPassed {
			t.Fatalf("adoption changed application outcome: %#v", outcome)
		}
	}
}
