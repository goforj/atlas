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
		"JSON API":              jsonAPIFeatureBehaviorProbe,
		"local inspect capture": runtimeObservabilityBehaviorProbe,
		"route middleware":      tokenPolicyBehaviorProbe,
		"upload":                uploadWorkflowBehaviorProbe,
		"validated write":       invoiceValidationBehaviorProbe,
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
	for _, id := range []string{"add-named-cache/v1", "add-named-resource/v1", "add-named-storage/v1"} {
		contract := contracts[id]
		if !slices.Contains(contract.requiredChanges, "app/wire/inject_services_app.go") {
			t.Fatalf("%s does not require service registration: %#v", id, contract)
		}
		structural := false
		for _, source := range contract.sources {
			if source.providerConnection != nil {
				structural = true
				if source.providerConnection.accessor == "" || len(source.providerConnection.wirePaths) == 0 {
					t.Fatalf("%s provider connection is incomplete: %#v", id, source)
				}
			}
		}
		if !structural {
			t.Fatalf("%s does not accept structurally equivalent application services", id)
		}
	}
	for _, requirement := range []string{"disk.putContext != ctx", "nested/../../hello.txt", "context.WithCancel(ctx)", "reflect.ValueOf(NewService)"} {
		if !strings.Contains(uploadWorkflowBehaviorProbe, requirement) {
			t.Fatalf("upload probe omits resilient storage behavior %q:\n%s", requirement, uploadWorkflowBehaviorProbe)
		}
	}
}

// TestNamedResourceProviderConnectionRejectsDisconnectedAccessorHelpers keeps the selected accessor attached to the provider App Wire constructs.
func TestNamedResourceProviderConnectionRejectsDisconnectedAccessorHelpers(t *testing.T) {
	for _, resource := range []struct {
		name          string
		accessor      string
		managerImport string
	}{
		{name: "queue", accessor: "Reports", managerImport: "queues"},
		{name: "cache", accessor: "Profiles", managerImport: "caches"},
		{name: "storage", accessor: "Avatars", managerImport: "storages"},
	} {
		t.Run(resource.name, func(t *testing.T) {
			root := t.TempDir()
			servicePath := filepath.Join(root, "internal", "invoices", "service.go")
			wirePath := filepath.Join(root, "app", "wire", "inject_services_app.go")
			if err := os.MkdirAll(filepath.Dir(servicePath), 0o755); err != nil {
				t.Fatal(err)
			}
			if err := os.MkdirAll(filepath.Dir(wirePath), 0o755); err != nil {
				t.Fatal(err)
			}
			service := "package invoices\n\nimport \"project/internal/" + resource.managerImport + "\"\n\nfunc NewConnected(manager *" + resource.managerImport + ".Manager) any { return manager." + resource.accessor + "() }\nfunc disconnected(manager *" + resource.managerImport + ".Manager) any { return manager." + resource.accessor + "() }\nfunc NewOther() any { return nil }\n"
			if err := os.WriteFile(servicePath, []byte(service), 0o600); err != nil {
				t.Fatal(err)
			}
			contract := sourceContract{id: "named-resource", paths: []string{"internal/*/*.go"}, providerConnection: &providerConnectionContract{accessor: resource.accessor, managerImportSuffix: "/internal/" + resource.managerImport, wirePaths: []string{"app/wire/inject_services_app.go"}}}
			if err := os.WriteFile(wirePath, []byte("package wire\n\nimport (\n\t\"github.com/google/wire\"\n\t\"project/internal/invoices\"\n)\n\nvar appSet = wire.NewSet(invoices.NewConnected)\n"), 0o600); err != nil {
				t.Fatal(err)
			}
			if result := verifySurfaceSource(root, contract); result.Status != EndpointPassed {
				t.Fatalf("connected provider = %#v", result)
			}
			unrelated := strings.ReplaceAll(service, "project/internal/"+resource.managerImport, "project/internal/unrelated")
			unrelated = strings.ReplaceAll(unrelated, resource.managerImport+".Manager", "unrelated.Manager")
			if err := os.WriteFile(servicePath, []byte(unrelated), 0o600); err != nil {
				t.Fatal(err)
			}
			if result := verifySurfaceSource(root, contract); result.Status != EndpointFailed {
				t.Fatalf("unrelated manager = %#v, want failure", result)
			}
			if err := os.WriteFile(servicePath, []byte(service), 0o600); err != nil {
				t.Fatal(err)
			}
			if err := os.WriteFile(wirePath, []byte("package wire\n\nimport (\n\t\"github.com/google/wire\"\n\t\"project/internal/invoices\"\n)\n\nvar appSet = wire.NewSet(invoices.NewOther)\n"), 0o600); err != nil {
				t.Fatal(err)
			}
			if result := verifySurfaceSource(root, contract); result.Status != EndpointFailed {
				t.Fatalf("disconnected helper = %#v, want failure", result)
			}
		})
	}
}

// TestStructuralDeclarationContractAcceptsApplicationOwnedNaming keeps semantic evidence independent from a preferred service name.
func TestStructuralDeclarationContractAcceptsApplicationOwnedNaming(t *testing.T) {
	root := t.TempDir()
	path := filepath.Join(root, "delivery.go")
	if err := os.WriteFile(path, []byte(`package delivery
type Manager struct{}
func (Manager) Reports() any { return nil }
func assembleDelivery(manager *Manager) any { return manager.Reports() }
`), 0o600); err != nil {
		t.Fatal(err)
	}
	contract := sourceContract{
		id:    "named-resource",
		paths: []string{"delivery.go"},
		declarations: []declarationContract{{
			anyName:       true,
			identifiers:   []string{"Manager"},
			selectorCalls: []string{"Reports"},
		}},
	}
	if result := verifySurfaceSource(root, contract); result.Status != EndpointPassed {
		t.Fatalf("result = %#v, want structurally equivalent constructor accepted", result)
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

// TestCorrectedOwnershipContractsAcceptRelatedFrameworkFiles keeps focused tests and generated registration points inside their feature boundary.
func TestCorrectedOwnershipContractsAcceptRelatedFrameworkFiles(t *testing.T) {
	contracts := make(map[string]surfaceContract)
	for _, contract := range promotedSurfaceContracts() {
		contracts[contract.id] = contract
	}
	tests := []struct {
		name     string
		contract string
		path     string
	}{
		{name: "mail accessor registration", contract: "add-mail-workflow/v1", path: "app/wire/app.go"},
		{name: "outbound wiring test", contract: "add-outbound-http-integration/v1", path: "app/wire/taxrates_test.go"},
		{name: "domain subscriber registration", contract: "publish-domain-event/v1", path: "app/wire/inject_subscribers_app.go"},
		{name: "followup subscriber registration", contract: "dispatch-event-followup-job/v1", path: "app/wire/inject_subscribers_app.go"},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			contract, ok := contracts[test.contract]
			if !ok {
				t.Fatalf("contract %q is absent", test.contract)
			}
			patterns := append(append([]string(nil), contract.allowedChanges...), contract.qualityTestPatterns...)
			result := verifySurfaceOwnership([]ProjectChange{{Path: test.path, After: ProjectPathState{Kind: "file"}}}, patterns)
			if result.Status != EndpointPassed {
				t.Fatalf("ownership result = %#v", result)
			}
		})
	}
	for _, id := range []string{"add-job/v1", "add-schedule/v1", "add-event-subscriber/v1"} {
		contract := promotedContract(t, id)
		if len(contract.commands) != 2 || !contract.commands[0].standard || contract.commands[1].id == "" {
			t.Fatalf("scaffold contract %q must retain a gating generated-shape behavior probe: %#v", id, contract.commands)
		}
	}
}

// TestScaffoldContractsAllowInternalVariations keeps static calibration bounded to generated scaffold shapes.
func TestScaffoldContractsAllowInternalVariations(t *testing.T) {
	tests := []struct {
		name       string
		contractID string
		sourceID   string
		path       string
		body       string
	}{
		{
			name:       "job service delegation",
			contractID: "add-job/v1",
			sourceID:   "typed-job",
			path:       "internal/invoices/receipt_job.go",
			body: `package invoices

import "context"

type ReceiptJobPayload struct{ InvoiceID string }
type Service struct{}
type Task struct{}
type QueueManager struct{}
type ReceiptJob struct{ service *Service; queues *QueueManager }

func (task *Task) Bind(any) error { return nil }
func (queues *QueueManager) Queue(string) *QueueManager { return queues }
func (queues *QueueManager) Dispatch(context.Context, any) error { return nil }
func (job *ReceiptJob) Queue(ctx context.Context, payload ReceiptJobPayload) error {
	return job.queues.Queue("invoices:receipt").Dispatch(ctx, payload)
}
func (job *ReceiptJob) HandleTask(ctx context.Context, task *Task) error {
	var payload ReceiptJobPayload
	if err := task.Bind(&payload); err != nil { return err }
	return job.reloadInvoice(ctx, payload)
}
func (job *ReceiptJob) reloadInvoice(context.Context, ReceiptJobPayload) error { return nil }
`,
		},
		{
			name:       "schedule direct service delegation",
			contractID: "add-schedule/v1",
			sourceID:   "schedule-shape",
			path:       "internal/invoices/reconcile_schedule.go",
			body: `package invoices

import "context"

const ReconcileScheduleTypeName = "invoices:reconcile"
type Service struct{}
type ReconcileSchedule struct{ service *Service }

func (schedule *ReconcileSchedule) Interval() string { return "1h" }
func (schedule *ReconcileSchedule) Handle(ctx context.Context) error {
	return schedule.service.Find(ctx)
}
func (*Service) Find(context.Context) error { return nil }
`,
		},
		{
			name:       "subscriber direct service delegation",
			contractID: "add-event-subscriber/v1",
			sourceID:   "subscriber-boundary",
			path:       "internal/invoices/paid_subscriber.go",
			body: `package invoices

import "context"

type Service struct{}
type PaidEvent struct{ InvoiceID string }
type PaidSubscriber struct{ service *Service }

func (subscriber *PaidSubscriber) Handle(ctx context.Context, event PaidEvent) error {
	return subscriber.service.Find(ctx, event.InvoiceID)
}
func (*Service) Find(context.Context, string) error { return nil }
`,
		},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			root := t.TempDir()
			writeVerifierFile(t, root, test.path, test.body)
			if result := verifySurfaceSource(root, promotedSourceContract(t, test.contractID, test.sourceID)); result.Status != EndpointPassed {
				t.Fatalf("internal variation result = %#v", result)
			}
		})
	}
}

// TestModelRelationshipContractAcceptsConventionalPackageAndSpacing avoids coupling generated relationships to a custom package or YAML spacing.
func TestModelRelationshipContractAcceptsConventionalPackageAndSpacing(t *testing.T) {
	root := t.TempDir()
	writeVerifierFile(t, root, ".db-relationships.yaml", "users:\n  - 1-many   id  ->  posts:user_id\n")
	writeVerifierFile(t, root, "internal/models/models.go", `package models

type Post struct{}
type User struct{ Posts []Post }
type UserRepo struct{}
type PostRepo struct{}
func (UserRepo) Relationships() []string { return []string{"Posts"} }
func (UserRepo) WithContext() UserRepo { return UserRepo{} }
`)
	writeVerifierFile(t, root, "app/wire/inject_repositories_app.go", `package wire
func NewUserRepo() {}
func NewPostRepo() {}
`)
	contract := promotedContract(t, "model-relationships/v1")
	for _, source := range contract.sources {
		if result := verifySurfaceSource(root, source); result.Status != EndpointPassed {
			t.Fatalf("source %q result = %#v", source.id, result)
		}
	}
}

// TestModelRelationshipContractRejectsWrongLocalKey ensures spacing tolerance does not weaken relationship direction or key identity.
func TestModelRelationshipContractRejectsWrongLocalKey(t *testing.T) {
	root := t.TempDir()
	writeVerifierFile(t, root, ".db-relationships.yaml", "users:\n  - 1-many wrong -> posts:user_id\n")
	contract := promotedSourceContract(t, "model-relationships/v1", "relationship-contract")
	result := verifySurfaceSource(root, contract)
	if result.Status != EndpointFailed || !strings.Contains(result.Details, "1-many id -> posts:user_id") {
		t.Fatalf("wrong local key result = %#v", result)
	}
}

// TestModelRelationshipContractRejectsSplitLocalKey prevents formatting tolerance from joining malformed identifiers.
func TestModelRelationshipContractRejectsSplitLocalKey(t *testing.T) {
	root := t.TempDir()
	writeVerifierFile(t, root, ".db-relationships.yaml", "users:\n  - 1-many i d -> posts:user_id\n")
	contract := promotedSourceContract(t, "model-relationships/v1", "relationship-contract")
	result := verifySurfaceSource(root, contract)
	if result.Status != EndpointFailed || !strings.Contains(result.Details, "1-many id -> posts:user_id") {
		t.Fatalf("split local key result = %#v", result)
	}
}

// TestReportJobContractAllowsConsumerOwnedQueueInterface keeps the producer contract independent from the notification package's boundary.
func TestReportJobContractAllowsConsumerOwnedQueueInterface(t *testing.T) {
	root := t.TempDir()
	writeVerifierFile(t, root, "internal/reports/report.go", `package reports

import "context"

const GenerateJobTypeName = "reports:generate"
type GeneratePayload struct{ UserID string }
type Task struct{}
type QueueManager struct{}
type Repository struct{}
type Service struct{ repository Repository }
type GenerateJob struct{ queue QueueManager; service Service }
func (*Task) Bind(any) error { return nil }
func (QueueManager) Dispatch(context.Context, any) error { return nil }
func (Repository) Find(context.Context, string) error { return nil }
func (service Service) GenerateForUser(ctx context.Context, id string) error { return service.repository.Find(ctx, id) }
func (job GenerateJob) HandleTask(ctx context.Context, task *Task) error {
	var payload GeneratePayload
	if err := task.Bind(&payload); err != nil { return err }
	return job.service.GenerateForUser(ctx, payload.UserID)
}
func (job GenerateJob) Queue(ctx context.Context, id string) error {
	return job.queue.Dispatch(ctx, GeneratePayload{UserID: id})
}
`)
	contract := promotedSourceContract(t, "dispatch-event-followup-job/v1", "typed-report-job")
	if result := verifySurfaceSource(root, contract); result.Status != EndpointPassed {
		t.Fatalf("consumer-owned queue boundary result = %#v", result)
	}
}

// TestAvatarContractAcceptsHeaderAccessAndControllerRegistration keeps the static gate neutral to equivalent request and Wire composition APIs.
func TestAvatarContractAcceptsHeaderAccessAndControllerRegistration(t *testing.T) {
	root := t.TempDir()
	writeVerifierFile(t, root, "internal/avatars/controller.go", `package avatars

import "context"

type Image struct{ Digest string }
type Storage struct{}
type Service struct{ storage Storage }
type Controller struct{ service *Service }
type Context struct{}
func (Storage) WithContext(context.Context) Storage { return Storage{} }
func (Storage) Get(string) ([]byte, error) { return nil, nil }
func (service *Service) Find(ctx context.Context, id string) (Image, error) {
	_, _ = service.storage.WithContext(ctx).Get(id)
	return Image{Digest: id}, nil
}
func (Context) Header(string) string { return "" }
func (Context) SetHeader(string, string) {}
func (Context) NoContent(int) error { return nil }
func (Context) Blob(int, string, []byte) error { return nil }
func (controller *Controller) Show(ctx Context) error {
	var requestContext context.Context
	image, _ := controller.service.Find(requestContext, "avatar")
	ctx.SetHeader("Cache-Control", "public")
	ctx.SetHeader("ETag", image.Digest)
	if ctx.Header("If-None-Match") == image.Digest { return ctx.NoContent(304) }
	return ctx.Blob(200, "image/png", nil)
}
func (controller *Controller) Routes() []string { return []string{"/avatars/:id"} }
func NewController(*Service) *Controller { return &Controller{} }
`)
	writeVerifierFile(t, root, "app/routes.go", `package app
const avatarRoute = "/avatars/:id"
`)
	writeVerifierFile(t, root, "app/wire/inject_http_controllers_app.go", `package wire
func registerAvatars() { _ = NewController(NewService(manager.Avatars())) }
`)
	contract := promotedContract(t, "serve-cacheable-image/v1")
	for _, source := range contract.sources {
		if result := verifySurfaceSource(root, source); result.Status != EndpointPassed {
			t.Fatalf("source %q result = %#v", source.id, result)
		}
	}
}

// promotedContract returns one reviewed contract by identifier for focused verifier regression tests.
func promotedContract(t *testing.T, id string) surfaceContract {
	t.Helper()
	for _, contract := range promotedSurfaceContracts() {
		if contract.id == id {
			return contract
		}
	}
	t.Fatalf("contract %q is absent", id)
	return surfaceContract{}
}

// promotedSourceContract returns one source boundary from a reviewed contract.
func promotedSourceContract(t *testing.T, contractID string, sourceID string) sourceContract {
	t.Helper()
	contract := promotedContract(t, contractID)
	for _, source := range contract.sources {
		if source.id == sourceID {
			return source
		}
	}
	t.Fatalf("source %q is absent from contract %q", sourceID, contractID)
	return sourceContract{}
}

// writeVerifierFile writes one candidate file while preserving the package paths exercised by the contract.
func writeVerifierFile(t *testing.T, root string, path string, body string) {
	t.Helper()
	fullPath := filepath.Join(root, filepath.FromSlash(path))
	if err := os.MkdirAll(filepath.Dir(fullPath), 0o755); err != nil {
		t.Fatalf("create %s parent: %v", path, err)
	}
	if err := os.WriteFile(fullPath, []byte(body), 0o600); err != nil {
		t.Fatalf("write %s: %v", path, err)
	}
}
