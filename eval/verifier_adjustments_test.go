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
		"named cache":           namedCacheBehaviorProbe,
		"named queue":           namedQueueBehaviorProbe,
		"named storage":         namedStorageBehaviorProbe,
		"JSON API":              jsonAPIFeatureBehaviorProbe,
		"local inspect capture": runtimeObservabilityBehaviorProbe,
		"resilient job":         resilientJobBehaviorProbe,
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

// TestScheduleBehaviorTargetFollowsApplicationNaming keeps the runtime oracle
// coupled to the selected constructor without requiring the golden spelling.
func TestScheduleBehaviorTargetFollowsApplicationNaming(t *testing.T) {
	root := t.TempDir()
	writeVerifierFile(t, root, "internal/reports/reconciler.go", `package reports
type DailySchedule struct{}
func NewDailySchedule() *DailySchedule { return &DailySchedule{} }
func (*DailySchedule) Interval() string { return "daily" }
func (*DailySchedule) Handle() error { return nil }
`)
	constructor, directory, packageName, err := scheduleBehaviorTarget(root)
	if err != nil {
		t.Fatalf("schedule target: %v", err)
	}
	if constructor != "NewDailySchedule" || directory != "internal/reports" || packageName != "reports" {
		t.Fatalf("target = %q, %q, %q", constructor, directory, packageName)
	}
	writeVerifierFile(t, root, "internal/reports/other.go", `package reports
type OtherSchedule struct{}
func NewOtherSchedule() *OtherSchedule { return &OtherSchedule{} }
func (*OtherSchedule) Interval() string { return "hourly" }
func (*OtherSchedule) Handle() error { return nil }
`)
	if _, _, _, err := scheduleBehaviorTarget(root); err == nil {
		t.Fatal("ambiguous schedule constructors passed")
	}
}

// TestNamedResourceProbeResolutionDoesNotMutateReviewedContract keeps paired treatments isolated when providers choose different packages.
func TestNamedResourceProbeResolutionDoesNotMutateReviewedContract(t *testing.T) {
	contract := promotedContract(t, "add-named-cache/v1").commands[1]
	first := t.TempDir()
	writeVerifierFile(t, first, "app/cache.go", `package app
import "example.test/internal/caches"
func NewProfileCache(manager *caches.Manager) any { return manager.Profiles() }
`)
	writeVerifierFile(t, first, "app/wire/inject_services_app.go", `package wire
import (
	"example.test/app"
	"github.com/goforj/wire"
)
var appSet = wire.NewSet(app.NewProfileCache)
`)
	resolvedFirst, details := resolveNamedResourceProbe(first, contract)
	if details != "" {
		t.Fatalf("resolve first probe: %s", details)
	}
	if got := resolvedFirst.supervisorFiles[0].path; got != "app/atlas_eval_named_cache_test.go" {
		t.Fatalf("first probe path = %q", got)
	}

	second := t.TempDir()
	writeVerifierFile(t, second, "internal/profiles/cache.go", `package profiles
import "example.test/internal/caches"
func NewProfileCache(manager *caches.Manager) any { return manager.Profiles() }
`)
	writeVerifierFile(t, second, "app/wire/inject_services_app.go", `package wire
import (
	"example.test/internal/profiles"
	"github.com/goforj/wire"
)
var appSet = wire.NewSet(profiles.NewProfileCache)
`)
	resolvedSecond, details := resolveNamedResourceProbe(second, contract)
	if details != "" {
		t.Fatalf("resolve second probe: %s", details)
	}
	if got := resolvedSecond.supervisorFiles[0].path; got != "internal/profiles/atlas_eval_named_cache_test.go" {
		t.Fatalf("second probe path = %q", got)
	}
	if !strings.HasPrefix(resolvedSecond.supervisorFiles[0].body, "package profiles\n") {
		t.Fatalf("second probe package was contaminated by first resolution:\n%s", resolvedSecond.supervisorFiles[0].body)
	}
	if contract.arguments[2] != "./internal/invoices" || contract.supervisorFiles[0].path != "internal/invoices/atlas_eval_named_cache_test.go" {
		t.Fatalf("reviewed contract mutated: arguments=%q files=%#v", contract.arguments, contract.supervisorFiles)
	}
}

// TestScheduleContractAcceptsAppOwnedSchedule keeps the established app-level
// generator output eligible without widening ownership to unrelated app files.
func TestScheduleContractAcceptsAppOwnedSchedule(t *testing.T) {
	root := t.TempDir()
	contract := promotedContract(t, "add-schedule/v1")
	writeVerifierFile(t, root, "app/invoices_reconcile_schedule.go", `package app

import "context"

const ReconcileScheduleTypeName = "invoices:reconcile"
type Service struct{}
type InvoiceReconcileSchedule struct{ service *Service }
func NewInvoiceReconcileSchedule(service *Service) *InvoiceReconcileSchedule { return &InvoiceReconcileSchedule{service: service} }
func (schedule *InvoiceReconcileSchedule) Interval() string { return "1h" }
func (schedule *InvoiceReconcileSchedule) Handle(ctx context.Context) error { return schedule.service.Find(ctx) }
func (*Service) Find(context.Context) error { return nil }
`)
	if result := verifySurfaceSource(root, contract.sources[0]); result.Status != EndpointPassed {
		t.Fatalf("app-owned schedule shape = %#v", result)
	}
	constructor, directory, packageName, err := scheduleBehaviorTarget(root)
	if err != nil || constructor != "NewInvoiceReconcileSchedule" || directory != "app" || packageName != "app" {
		t.Fatalf("app-owned target = %q, %q, %q, %v", constructor, directory, packageName, err)
	}
	if result := verifySurfaceOwnership([]ProjectChange{{Path: "app/invoices_reconcile_schedule.go", After: ProjectPathState{Kind: "file"}}}, contract.allowedChanges); result.Status != EndpointPassed {
		t.Fatalf("app-owned schedule ownership = %#v", result)
	}
	if result := verifySurfaceOwnership([]ProjectChange{{Path: "app/unrelated.go", After: ProjectPathState{Kind: "file"}}}, contract.allowedChanges); result.Status != EndpointFailed {
		t.Fatalf("unrelated app ownership = %#v, want failure", result)
	}
}

// TestPromotedScheduleAndEventContractsAcceptEquivalentPackageShapes prevents golden file and type spellings from becoming requirements.
func TestPromotedScheduleAndEventContractsAcceptEquivalentPackageShapes(t *testing.T) {
	contracts := make(map[string]surfaceContract)
	for _, contract := range promotedSurfaceContracts() {
		contracts[contract.id] = contract
	}
	root := t.TempDir()
	write := func(path, body string) {
		t.Helper()
		path = filepath.Join(root, path)
		if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(path, []byte(body), 0o600); err != nil {
			t.Fatal(err)
		}
	}
	write("internal/reports/daily_runner.go", `package reports
type DailyTargetRepository interface { ListDailyReportTargets(any) ([]string, error) }
type DailyRunner struct{ targets DailyTargetRepository; queue interface{ Queue(any, string) error } }
func (runner DailyRunner) Run(ctx any) error { targets, _ := runner.targets.ListDailyReportTargets(ctx); for _, userID := range targets { _ = runner.queue.Queue(ctx, userID) }; return nil }
type DailySchedule struct{ runner DailyRunner }
func (schedule DailySchedule) Interval() (string, error) { return "reports:daily", nil }
func (schedule DailySchedule) Handle(ctx any) error { return schedule.runner.Run(ctx) }
`)
	if result := verifySurfaceSource(root, contracts["schedule-existing-job/v1"].sources[0]); result.Status != EndpointPassed {
		t.Fatalf("daily_runner.go shape = %#v", result)
	}
	write("internal/events/user_created_event.go", `package events
const topic = "users.created"
type UserCreatedEvent struct { UserID string }
func (UserCreatedEvent) Topic() string { return topic }
`)
	if result := verifySurfaceSource(root, contracts["publish-domain-event/v1"].sources[0]); result.Status != EndpointPassed {
		t.Fatalf("UserCreatedEvent shape = %#v", result)
	}
	write("internal/events/user_created_event.go", `package events
const topic = "users.created"
type UserCreatedEvent struct { UserID string; Email string }
func (UserCreatedEvent) Topic() string { return topic }
`)
	if result := verifySurfaceSource(root, contracts["publish-domain-event/v1"].sources[0]); result.Status != EndpointFailed {
		t.Fatalf("event payload mutant = %#v, want failure", result)
	}
	write("internal/notifications/subscribers.go", `package notifications
type UserCreatedHandler interface{ Handle(any, string) }
type Subscribers struct{ handler UserCreatedHandler }
func (subscribers Subscribers) Register(ctx any, bus interface{ Subscribe() }) { var UserID string; bus.Subscribe(); subscribers.handler.Handle(ctx, UserID) }
`)
	write("app/lifecycle.go", `package app
type LifecycleRegistry struct{ subscription interface{ Close() } }
func (registry LifecycleRegistry) Startup(ctx any) { Subscribers{}.Register(ctx, bus{}) }
func (registry LifecycleRegistry) Shutdown() { registry.subscription.Close() }
type Subscribers struct{}
func (Subscribers) Register(any, bus) {}
type bus struct{}
func (bus) Subscribe() {}
`)
	if result := verifySurfaceSource(root, contracts["publish-domain-event/v1"].sources[2]); result.Status != EndpointPassed {
		t.Fatalf("lifecycle-owned subscription shape = %#v", result)
	}
	write("app/lifecycle.go", `package app
type LifecycleRegistry struct{ subscription interface{ Close() } }
func (registry LifecycleRegistry) Startup(ctx any) { Subscribers{}.Register(ctx, bus{}) }
func (registry LifecycleRegistry) Shutdown() {}
type Subscribers struct{}
func (Subscribers) Register(any, bus) {}
type bus struct{}
func (bus) Subscribe() {}
`)
	if result := verifySurfaceSource(root, contracts["publish-domain-event/v1"].sources[2]); result.Status != EndpointFailed {
		t.Fatalf("unclosed subscription mutant = %#v, want failure", result)
	}
	write("internal/events/memory_bus.go", `package events
import "context"
func closeBus() { _ = context.Background() }
`)
	write("internal/users/events.go", `package users
type UserEventPublisher struct{ bus interface{ WithContext(any) interface{ Publish(any) error } } }
func NewUserEventPublisher(bus interface{ WithContext(any) interface{ Publish(any) error } }) *UserEventPublisher { return &UserEventPublisher{bus: bus} }
func (publisher UserEventPublisher) PublishCreated(ctx any, UserID string) error { return publisher.bus.WithContext(ctx).Publish(UserID) }
`)
	if result := verifySurfaceSource(root, contracts["publish-domain-event/v1"].sources[3]); result.Status != EndpointPassed {
		t.Fatalf("unrelated event package background context = %#v", result)
	}
	write("internal/users/events.go", `package users
import "context"
type UserEventPublisher struct{ bus interface{ WithContext(any) interface{ Publish(any) error } } }
func NewUserEventPublisher(bus interface{ WithContext(any) interface{ Publish(any) error } }) *UserEventPublisher { return &UserEventPublisher{bus: bus} }
func (publisher UserEventPublisher) PublishCreated(ctx any, UserID string) error { return publisher.bus.WithContext(context.Background()).Publish(UserID) }
`)
	if result := verifySurfaceSource(root, contracts["publish-domain-event/v1"].sources[3]); result.Status != EndpointFailed {
		t.Fatalf("publisher context replacement = %#v, want failure", result)
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
	if result := verifySurfaceOwnership([]ProjectChange{{Path: ".env.testing", After: ProjectPathState{Kind: "file"}}}, outbound.allowedChanges); result.Status != EndpointPassed {
		t.Fatalf("outbound test environment ownership = %#v", result)
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
	if subscriber := contracts["add-event-subscriber/v1"]; slices.Contains(subscriber.sources[2].selectorCalls, "Named") {
		t.Fatalf("subscriber registration still rejects Default(): %#v", subscriber.sources[2])
	}
	if route := contracts["add-named-app-route/v1"]; !slices.Contains(route.allowedChanges, "internal/audit/*.go") || !slices.Contains(route.allowedChanges, "internal/audits/*.go") {
		t.Fatalf("route ownership does not accept singular and plural feature packages: %#v", route.allowedChanges)
	}
	if transaction := contracts["add-database-transaction/v1"]; !slices.Contains(transaction.allowedChanges, "app/wire/app.go") || slices.Contains(transaction.sources[0].forbiddenCalls, "Background") {
		t.Fatalf("transaction contract rejects supported app wiring or nil-context normalization: %#v", transaction)
	}
	if additional := contracts["create-additional-app/v1"]; slices.Contains(additional.sources[0].text, "./bin/statuspage") {
		t.Fatalf("additional-app contract still requires the legacy binary watch: %#v", additional.sources[0])
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

// TestNamedResourceProbesFollowWireConnectedPackages keeps behavior evidence colocated with the boundary an application chooses to own.
func TestNamedResourceProbesFollowWireConnectedPackages(t *testing.T) {
	tests := []struct {
		name        string
		contractID  string
		directory   string
		packageName string
		provider    string
		accessor    string
		manager     string
	}{
		{name: "queue golden invoices", contractID: "add-named-resource/v1", directory: "internal/invoices", packageName: "invoices", provider: "NewReportDispatcher", accessor: "Reports", manager: "queues"},
		{name: "queue reports", contractID: "add-named-resource/v1", directory: "internal/reports", packageName: "reports", provider: "NewReportDispatcher", accessor: "Reports", manager: "queues"},
		{name: "cache profiles", contractID: "add-named-cache/v1", directory: "internal/profiles", packageName: "profiles", provider: "NewProfileCache", accessor: "Profiles", manager: "caches"},
		{name: "storage avatars", contractID: "add-named-storage/v1", directory: "internal/avatars", packageName: "avatars", provider: "NewAvatarStorage", accessor: "Avatars", manager: "storages"},
		{name: "cache app-owned", contractID: "add-named-cache/v1", directory: "app", packageName: "app", provider: "NewProfileCache", accessor: "Profiles", manager: "caches"},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			root := t.TempDir()
			wirePath := filepath.Join(root, "app", "wire", "inject_services_app.go")
			writeVerifierFile(t, root, filepath.ToSlash(filepath.Join(test.directory, "boundary.go")), "package "+test.packageName+"\n\nimport \"project/internal/"+test.manager+"\"\n\nfunc "+test.provider+"(manager *"+test.manager+".Manager) any { return manager."+test.accessor+"() }\n")
			if err := os.MkdirAll(filepath.Dir(wirePath), 0o755); err != nil {
				t.Fatal(err)
			}
			if err := os.WriteFile(wirePath, []byte("package wire\n\nimport (\n\t\"github.com/google/wire\"\n\t\"project/"+test.directory+"\"\n)\n\nvar appSet = wire.NewSet("+test.packageName+"."+test.provider+")\n"), 0o600); err != nil {
				t.Fatal(err)
			}
			contract := promotedContract(t, test.contractID)
			if result := verifySurfaceSource(root, contract.sources[2]); result.Status != EndpointPassed {
				t.Fatalf("provider connection = %#v", result)
			}
			resolved, details := resolveNamedResourceProbe(root, contract.commands[1])
			if details != "" {
				t.Fatalf("resolve named probe: %s", details)
			}
			if got, want := resolved.arguments[2], "./"+test.directory; got != want {
				t.Fatalf("probe package = %q, want %q", got, want)
			}
			if got, want := resolved.supervisorFiles[0].path, filepath.ToSlash(filepath.Join(test.directory, "atlas_eval_named_"+map[string]string{"add-named-resource/v1": "queue", "add-named-cache/v1": "cache", "add-named-storage/v1": "storage"}[test.contractID]+"_test.go")); got != want {
				t.Fatalf("probe path = %q, want %q", got, want)
			}
			if !strings.HasPrefix(resolved.supervisorFiles[0].body, "package "+test.packageName+"\n") {
				t.Fatalf("probe package declaration = %q", resolved.supervisorFiles[0].body[:min(len(resolved.supervisorFiles[0].body), 40)])
			}
			verifier := newSurfaceVerifier(&fakeCommandRunner{}, contract)
			patterns := verifier.namedResourceOwnershipPatterns(root)
			if result := verifySurfaceOwnership([]ProjectChange{{Path: test.directory, After: ProjectPathState{Kind: "directory"}}, {Path: filepath.ToSlash(filepath.Join(test.directory, "boundary.go")), After: ProjectPathState{Kind: "file"}}}, patterns); result.Status != EndpointPassed {
				t.Fatalf("selected package ownership = %#v", result)
			}
			if result := verifySurfaceOwnership([]ProjectChange{{Path: "internal/unrelated/escape.go", After: ProjectPathState{Kind: "file"}}}, patterns); result.Status != EndpointFailed {
				t.Fatalf("unrelated source ownership = %#v, want failure", result)
			}
		})
	}
}

// TestNamedResourceProbesRetainBehaviorMutants keeps the dynamic package selection from weakening the resource-specific oracles.
func TestNamedResourceProbesRetainBehaviorMutants(t *testing.T) {
	for name, probe := range map[string]struct {
		body       string
		mustRetain []string
	}{
		"queue job type": {body: namedQueueBehaviorProbe, mustRetain: []string{"manager.Register(\"reports:generate\"", "Dispatch(context.Background(), \"inv-42\")"}},
		"cache key":      {body: namedCacheBehaviorProbe, mustRetain: []string{"Store(context.Background(), \"users:42\", \"Ada\")", "GetString(\"users:42\")"}},
		"storage path":   {body: namedStorageBehaviorProbe, mustRetain: []string{"Store(context.Background(), \"users/42/avatar.txt\"", "Get(\"users/42/avatar.txt\")"}},
	} {
		t.Run(name, func(t *testing.T) {
			for _, required := range probe.mustRetain {
				if !strings.Contains(probe.body, required) {
					t.Fatalf("probe omits mutant guard %q:\n%s", required, probe.body)
				}
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
	return job.handleTask(ctx, task)
}
func (job *ReceiptJob) handleTask(ctx context.Context, task *Task) error {
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

// TestResilientJobContractRequiresAttemptAndTimeoutPolicy keeps retry safety explicit at the dispatch boundary.
func TestResilientJobContractRequiresAttemptAndTimeoutPolicy(t *testing.T) {
	root := t.TempDir()
	path := "internal/reports/generate_job.go"
	writeVerifierFile(t, root, path, `package reports

import "context"

const GenerateJobTypeName = "reports:generate"
type GeneratePayload struct{ UserID string }
type Task struct{}
type Builder struct{}
type QueueManager struct{}
type Repository struct{}
type Service struct{ repository Repository }
type GenerateJob struct{ queue QueueManager; service Service }
func (*Task) Bind(any) error { return nil }
func (Builder) Retry(int) Builder { return Builder{} }
func (Builder) Timeout(int) Builder { return Builder{} }
func (QueueManager) Dispatch(context.Context, Builder) error { return nil }
func (Repository) Find(context.Context, string) error { return nil }
func (Service) Put(context.Context, string) error { return nil }
func (service Service) GenerateForUser(ctx context.Context, id string) error {
	if err := service.repository.Find(ctx, id); err != nil { return err }
	return service.Put(ctx, "profile.json")
}
func (job GenerateJob) HandleTask(ctx context.Context, task *Task) error {
	var payload GeneratePayload
	if err := task.Bind(&payload); err != nil { return err }
	return job.service.GenerateForUser(ctx, payload.UserID)
}
func (job GenerateJob) Queue(ctx context.Context, _ string) error {
	return job.queue.Dispatch(ctx, Builder{}.Retry(3).Timeout(30))
}
`)
	contract := promotedSourceContract(t, "add-resilient-job/v1", "retry-safe-report-job")
	if result := verifySurfaceSource(root, contract); result.Status != EndpointPassed {
		t.Fatalf("resilient job result = %#v", result)
	}
	data, err := os.ReadFile(filepath.Join(root, path))
	if err != nil {
		t.Fatalf("read resilient job: %v", err)
	}
	mutant := strings.Replace(string(data), ".Retry(3)", "", 1)
	writeVerifierFile(t, root, path, mutant)
	if result := verifySurfaceSource(root, contract); result.Status != EndpointFailed {
		t.Fatalf("missing retry result = %#v, want failure", result)
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
