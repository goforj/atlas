package eval

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/goforj/atlas/project"
)

// TestResolveSeparatesControlAndBaselineTreatments protects the one-variable diagnostic comparison.
func TestResolveSeparatesControlAndBaselineTreatments(t *testing.T) {
	fixture := project.Project{Name: "Invoice Eval", GoForjVersion: "0.24.0"}
	none, err := ResolveProjectGuidance(GuidanceProfileNone, fixture)
	if err != nil {
		t.Fatalf("ResolveProjectGuidance(none): %v", err)
	}
	agents, err := ResolveProjectGuidance(GuidanceProfileAgents, fixture)
	if err != nil {
		t.Fatalf("ResolveProjectGuidance(agents): %v", err)
	}
	if len(none.Files) != 0 {
		t.Fatalf("none files = %#v", none.Files)
	}
	body := string(agents.Files["AGENTS.md"])
	for _, token := range []string{"forj make:*", "Never edit `wire_gen.go`", "can stand on its own", "Keep controllers, commands, jobs, and schedules thin"} {
		if !strings.Contains(body, token) {
			t.Fatalf("AGENTS.md missing %q", token)
		}
	}
	if _, err := ResolveProjectGuidance("everything", fixture); err == nil {
		t.Fatal("unknown profile was accepted")
	}
}

// TestProjectResolverUsesPreparedProjectFacts prevents callers from substituting treatment metadata before preparation.
func TestProjectResolverUsesPreparedProjectFacts(t *testing.T) {
	root := t.TempDir()
	if err := os.WriteFile(filepath.Join(root, "go.mod"), []byte("module example.com/invoiceeval\n\ngo 1.25.0\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	resolved, err := (ProjectGuidanceResolver{}).Resolve(context.Background(), GuidanceProfileAgents, PreparationResult{ProjectRoot: root})
	if err != nil {
		t.Fatalf("ResolveProjectGuidance(): %v", err)
	}
	if !strings.Contains(string(resolved.Files["AGENTS.md"]), "project: `invoiceeval`") {
		t.Fatalf("AGENTS.md did not use prepared Project facts:\n%s", resolved.Files["AGENTS.md"])
	}
}
