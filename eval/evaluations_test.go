package eval

import (
	"path/filepath"
	"strings"
	"testing"
)

// TestLoadPromotedDefinitionKeepsBundledEvaluationAddressable verifies consumers need no source-tree path at runtime.
func TestLoadPromotedDefinitionKeepsBundledEvaluationAddressable(t *testing.T) {
	definition, err := LoadPromotedDefinition("add-http-controller")
	if err != nil {
		t.Fatalf("LoadPromotedDefinition(): %v", err)
	}
	if definition.ID != "add-http-controller" || definition.ProjectScenario != "invoice-http-route" || definition.PromptDigest == "" {
		t.Fatalf("definition = %#v", definition)
	}
	if _, err := LoadPromotedDefinition("../private"); err == nil {
		t.Fatal("unknown promoted evaluation was accepted")
	}
}

// TestAddHTTPControllerEvaluationLoads keeps the prompt compact and policy-free while preserving promoted contract references.
func TestAddHTTPControllerEvaluationLoads(t *testing.T) {
	definition, err := LoadDefinition(filepath.Join("evaluations", "add_http_controller"))
	if err != nil {
		t.Fatalf("LoadDefinition(): %v", err)
	}
	if definition.ProjectScenario != "invoice-http-route" || definition.WorkflowID != "goforj-add-http-route/v1" || definition.VerifierID != "add-http-controller/v1" {
		t.Fatalf("definition = %#v", definition)
	}
	for _, leakedRecipe := range []string{"make:controller", "inject_http_controllers_app.go", "wire_gen.go"} {
		if strings.Contains(definition.Prompt, leakedRecipe) {
			t.Fatalf("prompt leaks workflow recipe %q", leakedRecipe)
		}
	}
}

// TestPromotedEvaluationPromptsDiscloseProbeContracts keeps hidden executable
// probes from requiring application APIs that the natural task never names.
func TestPromotedEvaluationPromptsDiscloseProbeContracts(t *testing.T) {
	tests := []struct {
		id       string
		required []string
	}{
		{
			id: "add-http-controller",
			required: []string{
				"GET /api/v1/invoices/:id", "Find(ctx", "error", "404",
			},
		},
		{
			id: "add-app-lifecycle-hook",
			required: []string{
				"app/lifecycle.go", "LifecycleRegistry", "NewLifecycleRegistry", "runtime.BeforeStartup", "BeforeStartup(ctx",
			},
		},
		{
			id: "add-outbound-http-integration",
			required: []string{
				"internal/taxrates", "NewClient(baseURL string)", "Find(ctx, country string) (Rate, error)", "GET /rates/{country}", `{"country":"US","percent":7.25}`,
			},
		},
		{
			id: "add-validated-write-endpoint",
			required: []string{
				"internal/invoices/controller.go", "Store", "CreateInput", "invalid_payload", "validation_failed", "customer_id", "total_cents",
			},
		},
		{
			id: "add-route-middleware",
			required: []string{
				"internal/invoices/middleware.go", "RequireToken", "X-Invoice-Token", "INVOICE_HTTP_TOKEN", "provideInvoiceController",
			},
		},
		{
			id: "add-database-transaction",
			required: []string{
				"internal/accounts/repository.go", "internal/accounts/service.go", "WithTransaction", "AdjustBalance", "Transfer(ctx, fromID, toID string, amount",
			},
		},
		{
			id: "add-mail-workflow",
			required: []string{
				"internal/invoices/receipt_mailer.go", "NewReceiptMailer", "Send(ctx, invoiceID, recipient) error", "MAIL_DRIVER", "log",
			},
		},
		{
			id: "choose-storage-for-files",
			required: []string{
				"STORAGE_ATTACHMENTS_DRIVER", "STORAGE_ATTACHMENTS_ROOT", "Attachments", "NewAttachmentService", "Store(ctx", "Read(ctx",
			},
		},
		{
			id: "serve-cacheable-image",
			required: []string{
				"GET /api/v1/avatars/:id", "internal/avatars", "NewController(NewService(manager.Avatars()))", "Controller.Show", "Cache-Control", "ETag", "If-None-Match", "304",
			},
		},
		{
			id: "build-json-api-feature",
			required: []string{
				"GET /api/v1/users/:id", "internal/users", "NewService()", "Controller", "Routes", "Find(ctx", "42", "unknown ID", "/users/:id",
			},
		},
		{
			id: "add-upload-workflow",
			required: []string{
				"POST /api/v1/uploads", "internal/uploads/service.go", "internal/uploads/controller.go", "StoreInput", "StoredUpload", "Uploads", "BodyBase64",
			},
		},
		{
			id: "publish-domain-event",
			required: []string{
				"Create(ctx, CreateUserInput)", "users.created", "UserCreated", "UserID", "UserEvents", "NewUserEventPublisher", "NewSubscribers",
			},
		},
		{
			id: "dispatch-event-followup-job",
			required: []string{
				"internal/reports/generate_job.go", "GeneratePayload", "GenerateJob", "GenerateJobTypeName", "ReportQueue", "UserID",
			},
		},
		{
			id: "add-resilient-job",
			required: []string{
				"reports:generate", "user ID", "retry budget", "per-attempt timeout", "deterministic", "caller cancellation",
			},
		},
		{
			id: "schedule-existing-job",
			required: []string{
				"internal/reports/daily_schedule.go", "DailyTargetRepository", "DailyRunner", "DailySchedule", "ListDailyReportTargets", "reports:daily", "24-hour",
			},
		},
		{
			id: "runtime-observability",
			required: []string{
				"metrics", "inspects", "Lighthouse", "/metrics", "LIGHTHOUSE_INSPECT_ENABLED", "route:list",
			},
		},
	}

	for _, test := range tests {
		t.Run(test.id, func(t *testing.T) {
			definition, err := LoadPromotedDefinition(test.id)
			if err != nil {
				t.Fatalf("LoadPromotedDefinition(): %v", err)
			}
			for _, required := range test.required {
				if !strings.Contains(definition.Prompt, required) {
					t.Errorf("prompt omits disclosed contract detail %q", required)
				}
			}
		})
	}
}

// TestPromotedEvaluationPromptsDoNotLeakWorkflowRecipes keeps generator
// discovery attributable to framework guidance rather than the natural task.
func TestPromotedEvaluationPromptsDoNotLeakWorkflowRecipes(t *testing.T) {
	tests := []struct {
		id        string
		forbidden []string
	}{
		{id: "add-http-controller", forbidden: []string{"make:controller", "generated `Controller`"}},
		{id: "build-json-api-feature", forbidden: []string{"controller workflow", "make:controller"}},
		{id: "choose-storage-for-files", forbidden: []string{"forj generate --storage"}},
		{id: "dispatch-event-followup-job", forbidden: []string{"forj make:job"}},
		{id: "add-resilient-job", forbidden: []string{"forj make:job"}},
		{id: "schedule-existing-job", forbidden: []string{"forj make:schedule"}},
		{id: "add-upload-workflow", forbidden: []string{"controller generator", "make:controller"}},
	}

	for _, test := range tests {
		t.Run(test.id, func(t *testing.T) {
			definition, err := LoadPromotedDefinition(test.id)
			if err != nil {
				t.Fatalf("LoadPromotedDefinition(): %v", err)
			}
			for _, forbidden := range test.forbidden {
				if strings.Contains(definition.Prompt, forbidden) {
					t.Errorf("prompt leaks scored workflow recipe %q", forbidden)
				}
			}
		})
	}
}

// TestImplementationFlexiblePromptsAvoidAnswerLeakage keeps framework discovery and domain design distinct from hidden verifier details.
func TestImplementationFlexiblePromptsAvoidAnswerLeakage(t *testing.T) {
	tests := []struct {
		id        string
		forbidden []string
	}{
		{id: "add-named-resource", forbidden: []string{"internal/invoices/report_dispatcher.go", "manager.Reports()"}},
		{id: "add-named-cache", forbidden: []string{"internal/invoices/profile_cache.go", "manager.Profiles()"}},
		{id: "add-named-storage", forbidden: []string{"internal/invoices/avatar_storage.go", "manager.Avatars()"}},
		{id: "create-additional-app", forbidden: []string{"cmd/statuspage/main.go", "InitializeApplication", "app/statuspage/wire"}},
	}
	for _, test := range tests {
		t.Run(test.id, func(t *testing.T) {
			definition, err := LoadPromotedDefinition(test.id)
			if err != nil {
				t.Fatalf("LoadPromotedDefinition(): %v", err)
			}
			for _, forbidden := range test.forbidden {
				if strings.Contains(definition.Prompt, forbidden) {
					t.Errorf("prompt leaks an implementation answer %q", forbidden)
				}
			}
		})
	}
}

// TestPromotedMajorSurfaceEvaluationsResolve keeps every shipped manifest joined to reviewed workflow and verifier contracts.
func TestPromotedMajorSurfaceEvaluationsResolve(t *testing.T) {
	runner := &fakeCommandRunner{}
	registry, err := NewRegistry(PromotedWorkflows(), PromotedVerifiers(runner))
	if err != nil {
		t.Fatalf("NewRegistry(): %v", err)
	}
	ids, err := PromotedEvaluationIDs("")
	if err != nil {
		t.Fatalf("PromotedEvaluationIDs(): %v", err)
	}
	for _, id := range ids {
		t.Run(id, func(t *testing.T) {
			definition, err := LoadPromotedDefinition(id)
			if err != nil {
				t.Fatalf("LoadPromotedDefinition(): %v", err)
			}
			if definition.ID != id || definition.PromptDigest == "" || (definition.Suite != "core" && definition.Suite != "calibration") {
				t.Fatalf("definition = %#v", definition)
			}
			if _, err := registry.Resolve(definition); err != nil {
				t.Fatalf("Resolve(): %v", err)
			}
		})
	}
}

// TestPromotedEvaluationIDsReturnsStableSuiteCatalog keeps CLI discovery aligned with embedded manifests.
func TestPromotedEvaluationIDsReturnsStableSuiteCatalog(t *testing.T) {
	ids, err := PromotedEvaluationIDs("core")
	if err != nil {
		t.Fatalf("PromotedEvaluationIDs(): %v", err)
	}
	if len(ids) != 31 || ids[0] != "add-app-command" || ids[len(ids)-1] == "unknown-framework-shape" {
		t.Fatalf("ids = %v", ids)
	}
}
