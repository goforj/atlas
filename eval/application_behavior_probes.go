package eval

const lifecycleReadinessBehaviorProbe = `package app

import (
	"context"
	"testing"

	"example.com/invoiceeval/internal/invoices"
)

// atlasReadinessRepository records the exact context and identity crossing the lifecycle boundary.
type atlasReadinessRepository struct {
	context context.Context
	id      string
}

// Find records readiness input while satisfying the application repository contract.
func (repository *atlasReadinessRepository) Find(ctx context.Context, id string) (invoices.Invoice, error) {
	repository.context = ctx
	repository.id = id
	return invoices.Invoice{ID: id}, nil
}

// TestAtlasLifecycleReadinessBehavior proves startup readiness delegates through the service with its caller context.
func TestAtlasLifecycleReadinessBehavior(t *testing.T) {
	type contextKey string
	ctx := context.WithValue(context.Background(), contextKey("probe"), "ready")
	repository := &atlasReadinessRepository{}
	registry := NewLifecycleRegistry(invoices.NewService(repository))
	if err := registry.BeforeStartup(ctx); err != nil {
		t.Fatalf("BeforeStartup(): %v", err)
	}
	if repository.context != ctx || repository.id != "readiness" {
		t.Fatalf("readiness call = context %v id %q", repository.context == ctx, repository.id)
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
	"testing"

	appmail "example.com/invoiceeval/internal/mail"
)

// TestAtlasReceiptMailBehavior proves current invoice state becomes one configured mail delivery.
func TestAtlasReceiptMailBehavior(t *testing.T) {
	t.Setenv("MAIL_DRIVER", "log")
	t.Setenv("MAIL_FROM_ADDRESS", "no-reply@example.test")
	manager, err := appmail.NewManager()
	if err != nil {
		t.Fatalf("mail.NewManager(): %v", err)
	}
	mailer := NewReceiptMailer(NewService(NewRepository()), manager)
	if err := mailer.Send(context.Background(), "inv-42", "customer@example.test"); err != nil {
		t.Fatalf("Send(): %v", err)
	}
	subject, body := receiptContent(Invoice{ID: "inv-42", TotalCents: 12500})
	if subject != "Invoice inv-42" || body != "Total: 12500 cents" {
		t.Fatalf("receipt content = %q, %q", subject, body)
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

// TestAtlasJSONAPIFeatureBehavior proves the application boundary returns the documented user and rejects missing identity.
func TestAtlasJSONAPIFeatureBehavior(t *testing.T) {
	service := NewService()
	user, err := service.Find(context.Background(), "42")
	if err != nil || user.ID != "42" || user.Email != "ada@example.test" {
		t.Fatalf("Find(42) = %#v, %v", user, err)
	}
	if _, err := service.Find(context.Background(), ""); err == nil {
		t.Fatal("Find(empty) succeeded")
	}
}
`

const domainEventBehaviorProbe = `package notifications

import (
	"context"
	"testing"

	appevents "example.com/scenarioapp/internal/events"
)

// atlasUserCreatedHandler records delivery without relying on candidate-authored tests.
type atlasUserCreatedHandler struct {
	userID string
	email  string
}

// HandleUserCreated records the typed event payload delivered by the subscriber.
func (handler *atlasUserCreatedHandler) HandleUserCreated(_ context.Context, userID, email string) error {
	handler.userID = userID
	handler.email = email
	return nil
}

// TestAtlasDomainEventBehavior proves the configured bus delivers the typed fact to the application subscriber.
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
	if err := bus.WithContext(ctx).Publish(appevents.UserCreated{UserID: "43", Email: "grace@example.test"}); err != nil {
		t.Fatalf("Publish(): %v", err)
	}
	if handler.userID != "43" || handler.email != "grace@example.test" {
		t.Fatalf("delivered payload = %q, %q", handler.userID, handler.email)
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
