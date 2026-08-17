package eval

import (
	"go/parser"
	"go/token"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"testing"
)

// TestCorrectedBehaviorProbesParse catches malformed supervisor code before a rendered calibration spends setup time.
func TestCorrectedBehaviorProbesParse(t *testing.T) {
	for name, source := range map[string]string{
		"JSON API":         jsonAPIFeatureBehaviorProbe,
		"route middleware": tokenPolicyBehaviorProbe,
		"upload":           uploadWorkflowBehaviorProbe,
		"validated write":  invoiceValidationBehaviorProbe,
	} {
		t.Run(name, func(t *testing.T) {
			if _, err := parser.ParseFile(token.NewFileSet(), "probe_test.go", source, parser.AllErrors); err != nil {
				t.Fatalf("parse behavior probe: %v", err)
			}
		})
	}
}

// TestCorrectedVerifierContractsPreserveBehaviorOverImplementationSpelling locks the reviewed verifier boundaries to their public contracts.
func TestCorrectedVerifierContractsPreserveBehaviorOverImplementationSpelling(t *testing.T) {
	contracts := make(map[string]surfaceContract)
	for _, contract := range promotedSurfaceContracts() {
		contracts[contract.id] = contract
	}

	outbound := contracts["add-outbound-http-integration/v1"]
	if outbound.id == "" {
		t.Fatal("outbound HTTP contract is absent")
	}
	if result := verifySurfaceOwnership([]ProjectChange{{Path: ".env.example", After: ProjectPathState{Kind: "file"}}}, outbound.allowedChanges); result.Status != EndpointPassed {
		t.Fatalf("outbound environment example ownership = %#v", result)
	}

	jsonAPI := contracts["build-json-api-feature/v1"]
	if jsonAPI.id == "" {
		t.Fatal("JSON API contract is absent")
	}
	for _, source := range jsonAPI.sources {
		if source.id != "users-application-boundary" {
			continue
		}
		for _, choices := range source.identifierChoices {
			for _, choice := range choices {
				if choice == "Show" {
					t.Fatalf("JSON API contract still requires a handler name: %#v", source)
				}
			}
		}
		if len(source.declarations) != 1 || source.declarations[0].name != "Routes" {
			t.Fatalf("JSON API contract should retain only route composition shape: %#v", source)
		}
		if !slices.Contains(source.forbiddenCalls, "Background") {
			t.Fatalf("JSON API contract permits request context detachment: %#v", source)
		}
	}

	middleware := contracts["add-route-middleware/v1"]
	if middleware.id == "" {
		t.Fatal("route middleware contract is absent")
	}
	for _, source := range middleware.sources {
		for _, declaration := range source.declarations {
			for _, selector := range declaration.selectorCalls {
				if selector == "Request" || selector == "Get" {
					t.Fatalf("middleware contract still pins request or environment accessor spelling: %#v", declaration)
				}
			}
		}
	}
	for _, requirement := range []string{"calledNext != test.next", `value == "unauthorized"`} {
		if !strings.Contains(tokenPolicyBehaviorProbe, requirement) {
			t.Fatalf("middleware behavior probe omits %q:\n%s", requirement, tokenPolicyBehaviorProbe)
		}
	}

	for _, requirement := range []string{"NewService(&MemoryRepository{})", "json.Unmarshal(response.Body.Bytes(), &created)", "created.TotalCents != 12500"} {
		if !strings.Contains(invoiceValidationBehaviorProbe, requirement) {
			t.Fatalf("invoice validation probe does not preserve the full normalized result %q:\n%s", requirement, invoiceValidationBehaviorProbe)
		}
	}
	for _, requirement := range []string{"controller.Routes()", "route.Handler()", "GET missing user status"} {
		if !strings.Contains(jsonAPIFeatureBehaviorProbe, requirement) {
			t.Fatalf("JSON API probe does not exercise the registered HTTP route %q:\n%s", requirement, jsonAPIFeatureBehaviorProbe)
		}
	}
	if strings.Contains(jsonAPIFeatureBehaviorProbe, ".Show(") || strings.Contains(jsonAPIFeatureBehaviorProbe, ".Get(") {
		t.Fatalf("JSON API probe still depends on a handler method name:\n%s", jsonAPIFeatureBehaviorProbe)
	}
	for _, requirement := range []string{"disk.putContext != ctx", "nested/../../hello.txt", "context.WithCancel(ctx)", "reflect.ValueOf(NewService)"} {
		if !strings.Contains(uploadWorkflowBehaviorProbe, requirement) {
			t.Fatalf("upload probe omits resilient storage behavior %q:\n%s", requirement, uploadWorkflowBehaviorProbe)
		}
	}
}

// TestTokenProviderFlowAcceptsAccessorsAndRejectsDisconnectedValues keeps configuration lookup spelling flexible without losing constructor wiring evidence.
func TestTokenProviderFlowAcceptsAccessorsAndRejectsDisconnectedValues(t *testing.T) {
	root := t.TempDir()
	path := filepath.Join(root, "provider.go")
	contract := sourceContract{
		id:    "token-provider",
		paths: []string{"provider.go"},
		declarations: []declarationContract{{
			name:          "provideInvoiceController",
			argumentFlows: []callArgumentFlowContract{{call: "NewController", literal: "INVOICE_HTTP_TOKEN"}},
		}},
	}
	for name, source := range map[string]string{
		"environment accessor": `package wire
func provideInvoiceController(service any) any {
	token := environment.String("INVOICE_HTTP_TOKEN")
	return NewController(service, token)
}
`,
		"configuration accessor": `package wire
func provideInvoiceController(service any) any {
	return NewController(service, configuration.Get("INVOICE_HTTP_TOKEN"))
}
`,
	} {
		t.Run(name, func(t *testing.T) {
			if err := os.WriteFile(path, []byte(source), 0o600); err != nil {
				t.Fatal(err)
			}
			if result := verifySurfaceSource(root, contract); result.Status != EndpointPassed {
				t.Fatalf("result = %#v", result)
			}
		})
	}
	if err := os.WriteFile(path, []byte(`package wire
func provideInvoiceController(service any) any {
	token := configuration.Get("INVOICE_HTTP_TOKEN")
	return NewController(service, "not-the-token")
}
`), 0o600); err != nil {
		t.Fatal(err)
	}
	if result := verifySurfaceSource(root, contract); result.Status != EndpointFailed {
		t.Fatalf("disconnected token result = %#v, want failure", result)
	}
}
