package eval

import (
	"context"
	"fmt"
	"go/ast"
	"go/parser"
	"go/token"
	"os"
	"path/filepath"
	"strings"
)

const runtimeObservabilityBehaviorProbe = `package inspects

import (
	"bufio"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// TestAtlasLocalInspectCaptureBehavior proves the generated local configuration enables the inspect manager Lighthouse uses.
func TestAtlasLocalInspectCaptureBehavior(t *testing.T) {
	file, err := os.Open(filepath.Join("..", "..", ".env.local"))
	if err != nil {
		t.Fatalf("open .env.local: %v", err)
	}
	defer file.Close()
	value := ""
	scanner := bufio.NewScanner(file)
	for scanner.Scan() {
		key, candidate, found := strings.Cut(scanner.Text(), "=")
		if found && key == "LIGHTHOUSE_INSPECT_ENABLED" {
			value = candidate
		}
	}
	if err := scanner.Err(); err != nil {
		t.Fatalf("read .env.local: %v", err)
	}
	t.Setenv("LIGHTHOUSE_INSPECT_ENABLED", value)
	if !NewConfig().Enabled {
		t.Fatalf("LIGHTHOUSE_INSPECT_ENABLED=%q leaves local inspect capture disabled", value)
	}
}
`

const lifecycleReadinessBehaviorProbe = `package app

import (
	"context"
	"errors"
	"testing"

	"example.com/invoiceeval/internal/invoices"
	"example.com/invoiceeval/internal/runtime"
)

// atlasReadinessRepository records the lifecycle call and can model an unavailable dependency.
type atlasReadinessRepository struct {
	context context.Context
	id      string
	err     error
}

// Find records readiness input while satisfying the application repository contract.
func (repository *atlasReadinessRepository) Find(ctx context.Context, id string) (invoices.Invoice, error) {
	repository.context = ctx
	repository.id = id
	if repository.err != nil {
		return invoices.Invoice{}, repository.err
	}
	return invoices.Invoice{ID: id}, nil
}

// TestAtlasLifecycleReadinessBehavior proves the registered readiness hook preserves context and stops startup on failure.
func TestAtlasLifecycleReadinessBehavior(t *testing.T) {
	type contextKey string
	ctx := context.WithValue(context.Background(), contextKey("probe"), "ready")
	repository := &atlasReadinessRepository{}
	registry := NewLifecycleRegistry(invoices.NewService(repository))
	lifecycle := runtime.NewLifecycle(runtime.NewTimeouts())
	registry.Register(lifecycle)
	if err := lifecycle.Start(ctx); err != nil {
		t.Fatalf("lifecycle.Start(): %v", err)
	}
	if repository.context != ctx || repository.id == "" {
		t.Fatalf("readiness call = context %v id present %v", repository.context == ctx, repository.id != "")
	}
	failure := errors.New("invoice repository unavailable")
	failingRegistry := NewLifecycleRegistry(invoices.NewService(&atlasReadinessRepository{err: failure}))
	failingLifecycle := runtime.NewLifecycle(runtime.NewTimeouts())
	failingRegistry.Register(failingLifecycle)
	if err := failingLifecycle.Start(ctx); !errors.Is(err, failure) {
		t.Fatalf("lifecycle.Start() error = %v, want readiness failure", err)
	}
}
`

const taxRateHTTPBehaviorProbe = `package taxrates

import (
	"context"
	"fmt"
	"net/http"
	"net/http/httptest"
	"testing"
)

// TestAtlasTaxRateHTTPBehavior proves typed decoding, path escaping, and caller cancellation at the remote boundary.
func TestAtlasTaxRateHTTPBehavior(t *testing.T) {
	paths := make(chan string, 2)
	server := httptest.NewServer(http.HandlerFunc(func(response http.ResponseWriter, request *http.Request) {
		paths <- request.URL.EscapedPath()
		response.Header().Set("Content-Type", "application/json")
		_, _ = fmt.Fprint(response, ` + "`" + `{"country":"US","percent":7.25}` + "`" + `)
	}))
	t.Cleanup(server.Close)
	ctx, cancel := context.WithCancel(context.Background())
	rate, err := NewClient(server.URL).Find(ctx, "United States")
	if err != nil || rate.Country != "US" || rate.Percent != 7.25 {
		t.Fatalf("Find() = %#v, %v", rate, err)
	}
	if path := <-paths; path != "/rates/United%20States" {
		t.Fatalf("path = %q", path)
	}
	cancel()
	if _, err := NewClient(server.URL).Find(ctx, "US"); err == nil {
		t.Fatal("Find() ignored caller cancellation")
	}
}
`

const receiptMailBehaviorProbe = `package invoices

import (
	"context"
	"encoding/json"
	"os"
	"strings"
	"testing"

	appmail "example.com/invoiceeval/internal/mail"
)

// atlasReceiptRepository provides invoice state without depending on a scenario fixture identity.
type atlasReceiptRepository struct{}

// Find supplies a receipt-ready invoice through the existing application repository contract.
func (atlasReceiptRepository) Find(_ context.Context, id string) (Invoice, error) {
	return Invoice{ID: id, TotalCents: 12500}, nil
}

// TestAtlasReceiptMailBehavior proves a receipt workflow addresses one invoice-derived delivery through the configured default mailer.
func TestAtlasReceiptMailBehavior(t *testing.T) {
	t.Setenv("MAIL_DRIVER", "log")
	t.Setenv("MAIL_FROM_ADDRESS", "no-reply@example.test")
	t.Setenv("MAIL_LOG_BODIES", "true")
	output, err := os.CreateTemp(t.TempDir(), "receipt-mail-*.jsonl")
	if err != nil {
		t.Fatalf("create mail log: %v", err)
	}
	originalStdout := os.Stdout
	os.Stdout = output
	manager, managerErr := appmail.NewManager()
	os.Stdout = originalStdout
	if managerErr != nil {
		t.Fatalf("mail.NewManager(): %v", managerErr)
	}
	const recipient = "customer@example.test"
	mailer := NewReceiptMailer(NewService(atlasReceiptRepository{}), manager)
	if err := mailer.Send(context.Background(), "invoice-42", recipient); err != nil {
		t.Fatalf("Send(): %v", err)
	}
	if err := output.Close(); err != nil {
		t.Fatalf("close mail log: %v", err)
	}
	body, err := os.ReadFile(output.Name())
	if err != nil {
		t.Fatalf("read mail log: %v", err)
	}
	lines := strings.Split(strings.TrimSpace(string(body)), "\n")
	if len(lines) != 1 {
		t.Fatalf("mail log entries = %d, want 1", len(lines))
	}
	var delivery struct {
		To      []struct{ Email string }
		Subject string
		Text    string
	}
	if err := json.Unmarshal([]byte(lines[0]), &delivery); err != nil {
		t.Fatalf("decode mail log: %v", err)
	}
	if len(delivery.To) != 1 || delivery.To[0].Email != recipient {
		t.Fatalf("recipients = %#v, want %q", delivery.To, recipient)
	}
	amountPresent := strings.Contains(delivery.Text, "12500") || strings.Contains(delivery.Text, "125.00")
	if !strings.Contains(delivery.Subject, "invoice-42") || !amountPresent {
		t.Fatalf("receipt content = subject %q text %q", delivery.Subject, delivery.Text)
	}
}
`

const uploadWorkflowBehaviorProbe = `package uploads

import (
	"context"
	"path/filepath"
	"reflect"
	"strings"
	"testing"

	"example.com/scenarioapp/internal/storages"
	"github.com/goforj/storage"
)

// atlasUploadStorage records the context-bound write without selecting a storage driver the scenario did not declare.
type atlasUploadStorage struct {
	context context.Context
	putContext context.Context
	path    string
	body    []byte
}

// WithContext records the context supplied to the portable storage boundary.
func (disk *atlasUploadStorage) WithContext(ctx context.Context) storage.Storage {
	disk.context = ctx
	return disk
}

// Get returns the stored body when the probe requests the recorded path.
func (disk *atlasUploadStorage) Get(path string) ([]byte, error) {
	if path != disk.path {
		return nil, storage.ErrNotFound
	}
	return disk.body, nil
}

// Put records a portable storage write for the behavior assertion.
func (disk *atlasUploadStorage) Put(path string, body []byte) error {
	disk.putContext = disk.context
	disk.path = path
	disk.body = append([]byte(nil), body...)
	return nil
}

// MakeDir satisfies the storage contract; upload behavior does not create directories.
func (*atlasUploadStorage) MakeDir(string) error { return nil }

// Delete satisfies the storage contract; upload behavior does not delete objects.
func (*atlasUploadStorage) Delete(string) error { return nil }

// Stat satisfies the storage contract; upload behavior returns its own metadata.
func (*atlasUploadStorage) Stat(string) (storage.Entry, error) { return storage.Entry{}, storage.ErrNotFound }

// Exists satisfies the storage contract; upload behavior does not query existence.
func (*atlasUploadStorage) Exists(string) (bool, error) { return false, nil }

// List satisfies the storage contract; upload behavior does not list objects.
func (*atlasUploadStorage) List(string) ([]storage.Entry, error) { return nil, nil }

// Walk satisfies the storage contract; upload behavior does not walk objects.
func (*atlasUploadStorage) Walk(string, func(storage.Entry) error) error { return storage.ErrUnsupported }

// Copy satisfies the storage contract; upload behavior does not copy objects.
func (*atlasUploadStorage) Copy(string, string) error { return storage.ErrUnsupported }

// Move satisfies the storage contract; upload behavior does not move objects.
func (*atlasUploadStorage) Move(string, string) error { return storage.ErrUnsupported }

// URL satisfies the storage contract; upload behavior does not request object URLs.
func (*atlasUploadStorage) URL(string) (string, error) { return "", storage.ErrUnsupported }

// TestAtlasUploadWorkflowBehavior proves unsafe names are normalized and decoded bytes reach a context-bound portable storage boundary.
func TestAtlasUploadWorkflowBehavior(t *testing.T) {
	type contextKey string
	ctx := context.WithValue(context.Background(), contextKey("probe"), "upload")
	t.Setenv("STORAGE_DRIVER", "local")
	t.Setenv("STORAGE_ROOT", filepath.Join(t.TempDir(), "default"))
	t.Setenv("STORAGE_UPLOADS_DRIVER", "local")
	t.Setenv("STORAGE_UPLOADS_ROOT", filepath.Join(t.TempDir(), "uploads"))
	manager, err := storages.NewManager()
	if err != nil {
		t.Fatalf("storages.NewManager(): %v", err)
	}
	var managerPutContext context.Context
	var managerPutPath string
	manager.WithObserver(storages.ObserverFunc(func(observed context.Context, event storages.StorageOpEvent) {
		if event.Operation == "put" {
			managerPutContext = observed
			managerPutPath = event.Path
		}
	}))
	t.Cleanup(func() {
		if err := manager.Close(); err != nil {
			t.Errorf("manager.Close(): %v", err)
		}
	})
	disk := &atlasUploadStorage{}
	service, directStorage := atlasUploadService(t, disk, manager)
	for _, filename := range []string{"../hello.txt", "nested/../../hello.txt", ` + "`" + `..\hello.txt` + "`" + `} {
		upload, err := service.Store(ctx, StoreInput{Filename: filename, ContentType: "text/plain", BodyBase64: "aGVsbG8="})
		if err != nil {
			t.Fatalf("Store(%q): %v", filename, err)
		}
		if !atlasSafeUploadPath(upload.Path) || upload.Size != 5 || upload.ContentType != "text/plain" {
			t.Fatalf("upload for %q = %#v", filename, upload)
		}
		if directStorage && (disk.putContext != ctx || disk.path != upload.Path || string(disk.body) != "hello") {
			t.Fatalf("storage write = context preserved %v path %q body %q", disk.putContext == ctx, disk.path, disk.body)
		}
		if !directStorage {
			body, err := manager.Uploads().Get(upload.Path)
			if err != nil || string(body) != "hello" || managerPutContext != ctx || managerPutPath != upload.Path {
				t.Fatalf("manager upload = bytes %q err %v context preserved %v path %q", body, err, managerPutContext == ctx, managerPutPath)
			}
		}
	}
	if !directStorage {
		cancelled, cancel := context.WithCancel(ctx)
		cancel()
		if _, err := service.Store(cancelled, StoreInput{Filename: "cancelled.txt", ContentType: "text/plain", BodyBase64: "aGVsbG8="}); err == nil {
			t.Fatal("Store() succeeded after its caller context was cancelled")
		}
	}
}

// atlasUploadService supports either a directly injected disk or a service that retains the generated storage manager.
func atlasUploadService(t *testing.T, disk storage.Storage, manager *storages.Manager) (*Service, bool) {
	t.Helper()
	factory := reflect.ValueOf(NewService)
	if factory.Type().NumIn() != 1 || factory.Type().NumOut() != 1 {
		t.Fatalf("NewService signature = %s, want one dependency and one service", factory.Type())
	}
	parameter := factory.Type().In(0)
	storageValue := reflect.ValueOf(disk)
	managerValue := reflect.ValueOf(manager)
	dependency := reflect.Value{}
	directStorage := false
	switch {
	case managerValue.Type().AssignableTo(parameter):
		dependency = managerValue
	case storageValue.Type().AssignableTo(parameter), parameter.Kind() == reflect.Interface && storageValue.Type().Implements(parameter):
		dependency = storageValue
		directStorage = true
	default:
		t.Fatalf("NewService dependency = %s, want storage.Storage or *storages.Manager", parameter)
	}
	result := factory.Call([]reflect.Value{dependency})[0].Interface()
	service, ok := result.(*Service)
	if !ok || service == nil {
		t.Fatalf("NewService result = %T, want *Service", result)
	}
	return service, directStorage
}

// atlasSafeUploadPath rejects every traversal segment after normalizing platform separators.
func atlasSafeUploadPath(path string) bool {
	if path == "" {
		return false
	}
	for _, segment := range strings.Split(strings.ReplaceAll(path, "\\", "/"), "/") {
		if segment == "" || segment == "." || segment == ".." {
			return false
		}
	}
	return true
}
`

const jsonAPIFeatureBehaviorProbe = `package users

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/goforj/web/webtest"
)

// TestAtlasJSONAPIFeatureBehavior proves the registered HTTP route returns the requested user and rejects invalid identities.
func TestAtlasJSONAPIFeatureBehavior(t *testing.T) {
	controller := NewController(NewService())
	response := atlasUserResponse(t, controller, "42")
	if response.Code != http.StatusOK {
		t.Fatalf("GET user status = %d, want %d", response.Code, http.StatusOK)
	}
	var user struct{ ID string }
	if err := json.Unmarshal(response.Body.Bytes(), &user); err != nil || user.ID != "42" {
		t.Fatalf("GET user response = %#v, %v", user, err)
	}
	if response := atlasUserResponse(t, controller, "missing"); response.Code < http.StatusBadRequest {
		t.Fatalf("GET missing user status = %d, want an HTTP error", response.Code)
	}
}

// atlasUserResponse invokes the registered user route so the probe remains independent of the handler method name.
func atlasUserResponse(t *testing.T, controller *Controller, id string) *httptest.ResponseRecorder {
	t.Helper()
	request := httptest.NewRequest(http.MethodGet, "/users/"+id, nil)
	response := httptest.NewRecorder()
	for _, route := range controller.Routes() {
		if route.Method() != http.MethodGet || route.Path() != "/users/:id" {
			continue
		}
		if err := route.Handler()(webtest.NewContext(request, response, route.Path(), webtest.PathParams{"id": id})); err != nil {
			t.Fatalf("GET user: %v", err)
		}
		return response
	}
	t.Fatal("GET /users/:id route is absent")
	return nil
}
`

const domainEventBehaviorProbe = `package notifications

import (
	"context"
	"testing"

	appevents "example.com/scenarioapp/internal/events"
	"example.com/scenarioapp/internal/users"
)

// atlasUserCreatedHandler records delivery without relying on candidate-authored tests.
type atlasUserCreatedHandler struct {
	userID string
}

// HandleUserCreated records the typed event payload delivered by the subscriber.
func (handler *atlasUserCreatedHandler) HandleUserCreated(_ context.Context, userID string) error {
	handler.userID = userID
	return nil
}

// TestAtlasDomainEventBehavior proves user creation publishes an ID-only event to a registered subscriber.
func TestAtlasDomainEventBehavior(t *testing.T) {
	ctx := context.Background()
	t.Setenv("EVENTS_DRIVER", "inproc")
	manager, err := appevents.NewManagerWithContext(ctx)
	if err != nil {
		t.Fatalf("events.NewManagerWithContext(): %v", err)
	}
	bus := manager.Default()
	if err := bus.Start(ctx); err != nil {
		t.Fatalf("bus.Start(): %v", err)
	}
	t.Cleanup(func() { _ = bus.Close(context.Background()) })
	handler := &atlasUserCreatedHandler{}
	subscription, err := NewSubscribers(handler).Register(ctx, bus)
	if err != nil {
		t.Fatalf("Register(): %v", err)
	}
	t.Cleanup(func() { _ = subscription.Close() })
	service := users.NewService(users.NewMemoryUserRepository(), {{USER_EVENT_PUBLISHER}}(bus))
	user, err := service.Create(ctx, users.CreateUserInput{Name: "Grace Hopper", Email: "grace@example.test"})
	if err != nil {
		t.Fatalf("Create(): %v", err)
	}
	if user.ID == "" || handler.userID != user.ID {
		t.Fatalf("delivered user ID = %q, want %q", handler.userID, user.ID)
	}
}
`

// runDomainEventBehaviorProbe binds the oracle to either supported application package for the publisher implementation.
func runDomainEventBehaviorProbe(ctx context.Context, runner CommandRunner, project VerifierProject) EndpointResult {
	publisher, err := domainEventPublisherExpression(project.Root)
	if err != nil {
		return EndpointResult{ID: "domain-event-behavior", Status: EndpointFailed, Details: err.Error()}
	}
	body := strings.Replace(domainEventBehaviorProbe, "{{USER_EVENT_PUBLISHER}}", publisher, 1)
	return runIsolatedCommand(ctx, runner, project, commandContract{
		id:              "domain-event-behavior",
		arguments:       []string{"go", "test", "./internal/notifications", "-run", "^TestAtlasDomainEventBehavior$", "-count=1"},
		supervisorFiles: []supervisorFile{{path: "internal/notifications/atlas_eval_domain_event_test.go", body: body}},
	})
}

// domainEventPublisherExpression discovers the public constructor without prescribing whether the user or event package owns its adapter.
func domainEventPublisherExpression(root string) (string, error) {
	candidates := []struct {
		directory  string
		expression string
	}{
		{directory: filepath.Join(root, "internal", "users"), expression: "users.NewUserEventPublisher"},
		{directory: filepath.Join(root, "internal", "events"), expression: "appevents.NewUserEventPublisher"},
	}
	var expressions []string
	for _, candidate := range candidates {
		packages, err := parser.ParseDir(token.NewFileSet(), candidate.directory, func(info os.FileInfo) bool {
			return !strings.HasSuffix(info.Name(), "_test.go")
		}, 0)
		if err != nil {
			continue
		}
		for _, parsedPackage := range packages {
			if packageDeclaresFunction(parsedPackage, "NewUserEventPublisher") {
				expressions = append(expressions, candidate.expression)
			}
		}
	}
	if len(expressions) != 1 {
		return "", fmt.Errorf("expected one NewUserEventPublisher implementation in internal/users or internal/events, found %d", len(expressions))
	}
	return expressions[0], nil
}

// packageDeclaresFunction reports whether one package exposes the requested constructor.
func packageDeclaresFunction(parsedPackage *ast.Package, name string) bool {
	for _, file := range sortedPackageFiles(parsedPackage) {
		for _, declaration := range file.Decls {
			function, ok := declaration.(*ast.FuncDecl)
			if ok && function.Recv == nil && function.Name.Name == name {
				return true
			}
		}
	}
	return false
}

const eventFollowupJobBehaviorProbe = `package reports

import (
	"context"
	"encoding/json"
	"testing"

	"example.com/scenarioapp/internal/queues"
	"example.com/scenarioapp/internal/users"
	"github.com/goforj/queue"
	"github.com/goforj/storage"
	"github.com/goforj/storage/driver/memorystorage"
)

// TestAtlasEventFollowupJobBehavior proves an ID-only payload reloads current state before writing the report.
func TestAtlasEventFollowupJobBehavior(t *testing.T) {
	ctx := context.Background()
	t.Setenv("QUEUE_DRIVER", "sync")
	manager, err := queues.NewManager()
	if err != nil {
		t.Fatalf("queues.NewManager(): %v", err)
	}
	runtimeQueue := manager.Default()
	t.Cleanup(func() { _ = runtimeQueue.Shutdown(context.Background()) })
	disk, err := storage.Build(memorystorage.Config{})
	if err != nil {
		t.Fatalf("storage.Build(): %v", err)
	}
	repository := users.NewMemoryUserRepository()
	if _, err := repository.Save(ctx, users.User{ID: "42", Name: "Ada", Email: "current@example.test"}); err != nil {
		t.Fatalf("Save(): %v", err)
	}
	job := NewGenerateJob(manager, NewService(repository, disk))
	manager.Register(GenerateJobTypeName, job.HandleTask)
	if err := runtimeQueue.StartWorkers(ctx); err != nil {
		t.Fatalf("StartWorkers(): %v", err)
	}
	if _, err := manager.WithContext(ctx).Dispatch(queue.NewJob(GenerateJobTypeName).Payload([]byte(` + "`" + `{"user_id":"42"}` + "`" + `)).OnQueue("default")); err != nil {
		t.Fatalf("Dispatch(): %v", err)
	}
	body, err := disk.Get("users/42/profile.json")
	if err != nil {
		t.Fatalf("read report: %v", err)
	}
	var report UserReport
	if err := json.Unmarshal(body, &report); err != nil {
		t.Fatalf("decode report: %v", err)
	}
	if report.UserID != "42" || report.Email != "current@example.test" {
		t.Fatalf("report = %#v", report)
	}
}
`

const resilientJobBehaviorProbe = `package reports

import (
	"context"
	"encoding/json"
	"testing"

	"example.com/scenarioapp/internal/queues"
	"example.com/scenarioapp/internal/users"
	"github.com/goforj/storage"
	"github.com/goforj/storage/driver/memorystorage"
)

// TestAtlasResilientJobBehavior proves repeated delivery reloads current state and overwrites one deterministic report.
func TestAtlasResilientJobBehavior(t *testing.T) {
	ctx := context.Background()
	t.Setenv("QUEUE_DRIVER", "sync")
	manager, err := queues.NewManager()
	if err != nil {
		t.Fatalf("queues.NewManager(): %v", err)
	}
	runtimeQueue := manager.Default()
	t.Cleanup(func() { _ = runtimeQueue.Shutdown(context.Background()) })
	disk, err := storage.Build(memorystorage.Config{})
	if err != nil {
		t.Fatalf("storage.Build(): %v", err)
	}
	repository := users.NewMemoryUserRepository()
	if _, err := repository.Save(ctx, users.User{ID: "42", Name: "Ada", Email: "first@example.test"}); err != nil {
		t.Fatalf("Save(): %v", err)
	}
	job := NewGenerateJob(manager, NewService(repository, disk))
	manager.Register(GenerateJobTypeName, job.HandleTask)
	if err := runtimeQueue.StartWorkers(ctx); err != nil {
		t.Fatalf("StartWorkers(): %v", err)
	}
	if err := job.Queue(ctx, "42"); err != nil {
		t.Fatalf("Queue(first): %v", err)
	}
	if _, err := repository.Save(ctx, users.User{ID: "42", Name: "Ada", Email: "current@example.test"}); err != nil {
		t.Fatalf("Save(current): %v", err)
	}
	if err := job.Queue(ctx, "42"); err != nil {
		t.Fatalf("Queue(retry): %v", err)
	}
	body, err := disk.Get("users/42/profile.json")
	if err != nil {
		t.Fatalf("read report: %v", err)
	}
	var report UserReport
	if err := json.Unmarshal(body, &report); err != nil {
		t.Fatalf("decode report: %v", err)
	}
	if report.UserID != "42" || report.Email != "current@example.test" {
		t.Fatalf("report = %#v", report)
	}
}
`

const dailyScheduleBehaviorProbe = `package reports

import (
	"context"
	"slices"
	"testing"
	"time"
)

// atlasDailyTargets returns deterministic identities to the scheduled workflow.
type atlasDailyTargets struct{}

// ListDailyReportTargets returns two stable targets for dispatch verification.
func (atlasDailyTargets) ListDailyReportTargets(context.Context) ([]string, error) {
	return []string{"42", "43"}, nil
}

// atlasReportQueue records the identities crossing the queue boundary.
type atlasReportQueue struct{ queued []string }

// Queue records one scheduled report dispatch.
func (queue *atlasReportQueue) Queue(_ context.Context, userID string) error {
	queue.queued = append(queue.queued, userID)
	return nil
}

// TestAtlasDailyScheduleBehavior proves cadence and execution delegate every target to the existing job boundary.
func TestAtlasDailyScheduleBehavior(t *testing.T) {
	queue := &atlasReportQueue{}
	schedule := NewDailySchedule(NewDailyRunner(atlasDailyTargets{}, queue))
	interval, err := schedule.Interval()
	if err != nil || interval != 24*time.Hour {
		t.Fatalf("Interval() = %s, %v", interval, err)
	}
	if err := schedule.Handle(context.Background()); err != nil {
		t.Fatalf("Handle(): %v", err)
	}
	if !slices.Equal(queue.queued, []string{"42", "43"}) {
		t.Fatalf("queued = %#v", queue.queued)
	}
}
`
