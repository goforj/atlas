package eval

import (
	"slices"
	"testing"
)

// TestLoadCoverageCatalogMapsEveryCoreEvaluation keeps the framework inventory and promoted suite from drifting apart.
func TestLoadCoverageCatalogMapsEveryCoreEvaluation(t *testing.T) {
	catalog, err := LoadCoverageCatalog()
	if err != nil {
		t.Fatalf("LoadCoverageCatalog(): %v", err)
	}
	if len(catalog.Capabilities) < 50 {
		t.Fatalf("capability count = %d, want a broad framework inventory", len(catalog.Capabilities))
	}
	var planned []string
	for _, capability := range catalog.Capabilities {
		if !capability.Covered() {
			planned = append(planned, capability.ID)
		}
	}
	for _, required := range []string{"async.worker-failure", "data.multi-connection", "frontend.component-loading", "operations.deployment", "runtime.graceful-shutdown"} {
		if !slices.Contains(planned, required) {
			t.Fatalf("planned capabilities = %v, want %q", planned, required)
		}
	}
}

// TestLoadCoverageCatalogReturnsIndependentSlices prevents report callers from mutating later catalog reads.
func TestLoadCoverageCatalogReturnsIndependentSlices(t *testing.T) {
	first, err := LoadCoverageCatalog()
	if err != nil {
		t.Fatal(err)
	}
	first.Capabilities[0].Evaluations[0] = "mutated"
	second, err := LoadCoverageCatalog()
	if err != nil {
		t.Fatal(err)
	}
	if second.Capabilities[0].Evaluations[0] == "mutated" {
		t.Fatal("LoadCoverageCatalog() returned shared evaluation storage")
	}
}

// TestCoverageEvaluationIDsBuildsCumulativeStablePortfolios keeps fast and complete runs tied to the capability model.
func TestCoverageEvaluationIDsBuildsCumulativeStablePortfolios(t *testing.T) {
	catalog, err := LoadCoverageCatalog()
	if err != nil {
		t.Fatal(err)
	}
	smoke, err := catalog.EvaluationIDs(CoverageTierSmoke)
	if err != nil {
		t.Fatal(err)
	}
	core, err := catalog.EvaluationIDs(CoverageTierCore)
	if err != nil {
		t.Fatal(err)
	}
	extended, err := catalog.EvaluationIDs(CoverageTierExtended)
	if err != nil {
		t.Fatal(err)
	}
	if len(smoke) == 0 || len(smoke) >= len(core) || len(core) > len(extended) {
		t.Fatalf("portfolio sizes = smoke:%d core:%d extended:%d", len(smoke), len(core), len(extended))
	}
	for _, id := range smoke {
		if !slices.Contains(core, id) || !slices.Contains(extended, id) {
			t.Fatalf("smoke evaluation %q is absent from a broader portfolio", id)
		}
	}
	if _, err := catalog.EvaluationIDs("daily"); err == nil {
		t.Fatal("unknown coverage tier was accepted")
	}
}
