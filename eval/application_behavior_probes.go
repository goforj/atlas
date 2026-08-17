package eval

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
	"strings"
	"testing"

	"github.com/goforj/storage"
	"github.com/goforj/storage/driver/memorystorage"
)

// TestAtlasUploadWorkflowBehavior proves unsafe names are normalized and decoded bytes reach the portable storage boundary.
func TestAtlasUploadWorkflowBehavior(t *testing.T) {
	disk, err := storage.Build(memorystorage.Config{})
	if err != nil {
		t.Fatalf("storage.Build(): %v", err)
	}
	upload, err := NewService(disk).Store(context.Background(), StoreInput{Filename: "../hello.txt", ContentType: "text/plain", BodyBase64: "aGVsbG8="})
	if err != nil {
		t.Fatalf("Store(): %v", err)
	}
	if !strings.HasSuffix(upload.Path, "/hello.txt") || upload.Size != 5 || upload.ContentType != "text/plain" {
		t.Fatalf("upload = %#v", upload)
	}
	body, err := disk.Get(upload.Path)
	if err != nil || string(body) != "hello" {
		t.Fatalf("stored body = %q, %v", body, err)
	}
}
`

const jsonAPIFeatureBehaviorProbe = `package users

import (
	"context"
	"testing"
)

// TestAtlasJSONAPIFeatureBehavior proves the application boundary returns the requested user and rejects invalid identities.
func TestAtlasJSONAPIFeatureBehavior(t *testing.T) {
	service := NewService()
	user, err := service.Find(context.Background(), "42")
	if err != nil || user.ID != "42" {
		t.Fatalf("Find(42) = %#v, %v", user, err)
	}
	if _, err := service.Find(context.Background(), ""); err == nil {
		t.Fatal("Find(empty) succeeded")
	}
	if _, err := service.Find(context.Background(), "missing"); err == nil {
		t.Fatal("Find(unknown) succeeded")
	}
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
	service := users.NewService(users.NewMemoryUserRepository(), users.NewUserEventPublisher(bus))
	user, err := service.Create(ctx, users.CreateUserInput{Name: "Grace Hopper", Email: "grace@example.test"})
	if err != nil {
		t.Fatalf("Create(): %v", err)
	}
	if user.ID == "" || handler.userID != user.ID {
		t.Fatalf("delivered user ID = %q, want %q", handler.userID, user.ID)
	}
}
`

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
