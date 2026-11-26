package eval

import (
	"go/parser"
	"go/token"
	"reflect"
	"strings"
	"testing"
)

// TestFeatureAppRuntimeContract keeps the App oracle required and its generated sources parseable.
func TestFeatureAppRuntimeContract(t *testing.T) {
	contract := appResourceRuntimeContract("feature-app-runtime", "checkout", featureAppRuntimeProbe, featureAppRuntimeCommand)
	if !reflect.DeepEqual(contract.arguments, []string{"go", "test", "./internal/checkout", "-run", "^TestAtlasResourceApp$", "-count=1"}) {
		t.Fatalf("runtime command = %#v", contract.arguments)
	}
	if len(contract.supervisorFiles) != 1 || contract.supervisorFiles[0].path != "internal/checkout/atlas_eval_app_runtime_test.go" {
		t.Fatalf("runtime supervisor files = %#v", contract.supervisorFiles)
	}
	for name, source := range map[string]string{"supervisor_test.go": contract.supervisorFiles[0].body, "hello_world_cmd.go": featureAppRuntimeCommand} {
		if _, err := parser.ParseFile(token.NewFileSet(), name, source, parser.AllErrors); err != nil {
			t.Fatalf("generated %s: %v", name, err)
		}
	}
	if strings.Contains(featureAppRuntimeCommand, "checkout.NewService") || strings.Contains(featureAppRuntimeCommand, "features.NewManager") {
		t.Fatal("App oracle bypasses registered service providers")
	}
	for _, surface := range promotedSurfaceContracts() {
		if surface.id != "target-feature-rollout/v1" {
			continue
		}
		for _, command := range surface.commands {
			if command.id == contract.id {
				return
			}
		}
	}
	t.Fatal("feature App runtime contract is absent")
}
