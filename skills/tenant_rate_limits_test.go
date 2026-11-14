package skills

import (
	"slices"
	"strings"
	"testing"

	"github.com/goforj/atlas/project"
)

// TestTenantRateLimitGuidanceIsDiscoverableBeforeCacheInstall lets requirement selection precede dependency installation.
func TestTenantRateLimitGuidanceIsDiscoverableBeforeCacheInstall(t *testing.T) {
	if !slices.Contains(RecommendedNames(project.Project{Name: "plain"}), "goforj-tenant-request-rate-limits") {
		t.Fatal("missing guidance before cache installation")
	}
	skill, ok := ByName("goforj-tenant-request-rate-limits")
	if !ok {
		t.Fatal("missing skill")
	}
	for _, term := range []string{"process-local", "shared backend", "authentication", "failure", "Increment", "injected clock", "unaffected", "Cache locks"} {
		if !strings.Contains(skill.Content, term) {
			t.Fatalf("missing policy concept %q", term)
		}
	}
}
