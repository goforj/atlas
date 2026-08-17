package eval

const namedQueueBehaviorProbe = `package invoices

import (
	"context"
	"testing"

	"example.com/invoiceeval/internal/queues"
	"github.com/goforj/queue"
)

// TestAtlasNamedQueueBehavior proves the dispatcher sends report work through the configured reports queue.
func TestAtlasNamedQueueBehavior(t *testing.T) {
	t.Setenv("QUEUE_DRIVER", "null")
	t.Setenv("QUEUE_REPORTS_DRIVER", "sync")
	manager, err := queues.NewManager()
	if err != nil {
		t.Fatalf("queues.NewManager(): %v", err)
	}
	runtimeQueue := manager.Reports()
	t.Cleanup(func() {
		if err := runtimeQueue.Shutdown(context.Background()); err != nil {
			t.Errorf("reports queue shutdown: %v", err)
		}
	})
	manager.Register("reports:generate", func(context.Context, queue.Message) error { return nil })
	if err := runtimeQueue.StartWorkers(context.Background()); err != nil {
		t.Fatalf("reports queue start: %v", err)
	}
	if err := NewReportDispatcher(manager).Dispatch(context.Background(), "inv-42"); err != nil {
		t.Fatalf("dispatcher.Dispatch(): %v", err)
	}
}
`

const namedCacheBehaviorProbe = `package invoices

import (
	"context"
	"testing"

	"example.com/invoiceeval/internal/caches"
)

// TestAtlasNamedCacheBehavior proves the boundary writes through the configured profiles cache.
func TestAtlasNamedCacheBehavior(t *testing.T) {
	t.Setenv("CACHE_DRIVER", "null")
	t.Setenv("CACHE_PROFILES_DRIVER", "memory")
	manager, err := caches.NewManager()
	if err != nil {
		t.Fatalf("caches.NewManager(): %v", err)
	}
	if err := NewProfileCache(manager).Store(context.Background(), "users:42", "Ada"); err != nil {
		t.Fatalf("profiles.Store(): %v", err)
	}
	got, ok, err := manager.Profiles().GetString("users:42")
	if err != nil || !ok || got != "Ada" {
		t.Fatalf("profiles cache = %q, %t, %v; want Ada, true, nil", got, ok, err)
	}
}
`

const namedStorageBehaviorProbe = `package invoices

import (
	"bytes"
	"context"
	"path/filepath"
	"testing"

	"example.com/invoiceeval/internal/storages"
)

// TestAtlasNamedStorageBehavior proves the boundary writes avatar bytes through the configured avatars disk.
func TestAtlasNamedStorageBehavior(t *testing.T) {
	t.Setenv("STORAGE_DRIVER", "local")
	t.Setenv("STORAGE_ROOT", filepath.Join(t.TempDir(), "default"))
	t.Setenv("STORAGE_AVATARS_DRIVER", "local")
	t.Setenv("STORAGE_AVATARS_ROOT", filepath.Join(t.TempDir(), "avatars"))
	manager, err := storages.NewManager()
	if err != nil {
		t.Fatalf("storages.NewManager(): %v", err)
	}
	t.Cleanup(func() {
		if err := manager.Close(); err != nil {
			t.Errorf("storage manager close: %v", err)
		}
	})
	want := []byte("avatar")
	if err := NewAvatarStorage(manager).Store(context.Background(), "users/42/avatar.txt", want); err != nil {
		t.Fatalf("avatars.Store(): %v", err)
	}
	got, err := manager.Avatars().Get("users/42/avatar.txt")
	if err != nil || !bytes.Equal(got, want) {
		t.Fatalf("avatars disk = %q, %v; want %q, nil", got, err, want)
	}
}
`

const receiptJobBehaviorProbe = `package invoices

import (
	"context"
	"testing"

	"example.com/invoiceeval/internal/queues"
)

// TestAtlasReceiptJobBehavior proves typed receipt work reloads the requested invoice through the service.
func TestAtlasReceiptJobBehavior(t *testing.T) {
	t.Setenv("QUEUE_DRIVER", "sync")
	manager, err := queues.NewManager()
	if err != nil {
		t.Fatalf("queues.NewManager(): %v", err)
	}
	runtimeQueue := manager.Default()
	t.Cleanup(func() {
		if err := runtimeQueue.Shutdown(context.Background()); err != nil {
			t.Errorf("runtimeQueue.Shutdown(): %v", err)
		}
	})
	job := NewReceiptJob(manager, NewService(&MemoryRepository{}))
	manager.Register(ReceiptJobTypeName, job.HandleTask)
	if err := runtimeQueue.StartWorkers(context.Background()); err != nil {
		t.Fatalf("runtimeQueue.StartWorkers(): %v", err)
	}
	if err := job.Queue(context.Background(), ReceiptJobPayload{InvoiceID: "inv-42"}); err != nil {
		t.Fatalf("job.Queue(): %v", err)
	}
	if err := job.Queue(context.Background(), ReceiptJobPayload{InvoiceID: "missing"}); err == nil {
		t.Fatal("job.Queue() ignored the payload invoice identity")
	}
}
`

const paidSubscriberBehaviorProbe = `package invoices

import (
	"context"
	"testing"
)

// TestAtlasPaidSubscriberBehavior proves the subscriber delegates the event's invoice identity.
func TestAtlasPaidSubscriberBehavior(t *testing.T) {
	subscriber := NewPaidSubscriber(NewService(&MemoryRepository{}))
	if err := subscriber.Handle(context.Background(), PaidEvent{InvoiceID: "inv-42"}); err != nil {
		t.Fatalf("subscriber.Handle(): %v", err)
	}
	if err := subscriber.Handle(context.Background(), PaidEvent{InvoiceID: "missing"}); err == nil {
		t.Fatal("subscriber.Handle() ignored the event invoice identity")
	}
}
`

const reconcileScheduleBehaviorProbe = `package invoices

import (
	"context"
	"testing"
	"time"
)

// TestAtlasReconcileScheduleBehavior proves the generated cadence and registered work remain executable.
func TestAtlasReconcileScheduleBehavior(t *testing.T) {
	schedule := NewReconcileSchedule(NewService(&MemoryRepository{}))
	interval, err := schedule.Interval()
	if err != nil || interval != time.Hour {
		t.Fatalf("schedule.Interval() = %s, %v; want 1h", interval, err)
	}
	if err := schedule.Handle(context.Background()); err != nil {
		t.Fatalf("schedule.Handle(): %v", err)
	}
}
`

const attachmentStorageBehaviorProbe = `package invoices

import (
	"bytes"
	"context"
	"path/filepath"
	"testing"

	"example.com/invoiceeval/internal/storages"
)

// TestAtlasAttachmentStorageBehavior proves the service persists and retrieves bytes through the named disk.
func TestAtlasAttachmentStorageBehavior(t *testing.T) {
	t.Setenv("STORAGE_DRIVER", "local")
	t.Setenv("STORAGE_ROOT", filepath.Join(t.TempDir(), "default"))
	t.Setenv("STORAGE_ATTACHMENTS_DRIVER", "local")
	t.Setenv("STORAGE_ATTACHMENTS_ROOT", filepath.Join(t.TempDir(), "attachments"))
	manager, err := storages.NewManager()
	if err != nil {
		t.Fatalf("storages.NewManager(): %v", err)
	}
	t.Cleanup(func() {
		if err := manager.Close(); err != nil {
			t.Errorf("manager.Close(): %v", err)
		}
	})
	service := NewAttachmentService(manager)
	want := []byte("invoice attachment")
	attachment, err := service.Store(context.Background(), "inv-42", "receipt.txt", want)
	if err != nil {
		t.Fatalf("service.Store(): %v", err)
	}
	got, err := service.Read(context.Background(), attachment.Path)
	if err != nil {
		t.Fatalf("service.Read(): %v", err)
	}
	if !bytes.Equal(got, want) {
		t.Fatalf("service.Read() = %q, want %q", got, want)
	}
}
`

const avatarRevalidationBehaviorProbe = `package avatars

import (
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"testing"

	"example.com/invoiceeval/internal/storages"
	"github.com/goforj/web/webtest"
)

// TestAtlasAvatarRevalidationBehavior proves matching validators return an empty 304 response.
func TestAtlasAvatarRevalidationBehavior(t *testing.T) {
	t.Setenv("STORAGE_DRIVER", "local")
	t.Setenv("STORAGE_ROOT", filepath.Join(t.TempDir(), "default"))
	t.Setenv("STORAGE_AVATARS_DRIVER", "local")
	t.Setenv("STORAGE_AVATARS_ROOT", filepath.Join(t.TempDir(), "avatars"))
	manager, err := storages.NewManager()
	if err != nil {
		t.Fatalf("storages.NewManager(): %v", err)
	}
	t.Cleanup(func() {
		if err := manager.Close(); err != nil {
			t.Errorf("manager.Close(): %v", err)
		}
	})
	if err := manager.Avatars().Put("users/42/avatars/current.png", []byte("avatar")); err != nil {
		t.Fatalf("store avatar: %v", err)
	}
	controller := NewController(NewService(manager.Avatars()))
	firstRequest := httptest.NewRequest(http.MethodGet, "/api/v1/avatars/42", nil)
	firstResponse := httptest.NewRecorder()
	firstContext := webtest.NewContext(firstRequest, firstResponse, "/avatars/:id", webtest.PathParams{"id": "42"})
	if err := controller.Show(firstContext); err != nil {
		t.Fatalf("controller.Show(first): %v", err)
	}
	etag := firstResponse.Header().Get("ETag")
	if firstResponse.Code != http.StatusOK || etag == "" {
		t.Fatalf("first response = status %d ETag %q, want 200 and validator", firstResponse.Code, etag)
	}
	conditionalRequest := httptest.NewRequest(http.MethodGet, "/api/v1/avatars/42", nil)
	conditionalRequest.Header.Set("If-None-Match", etag)
	conditionalResponse := httptest.NewRecorder()
	conditionalContext := webtest.NewContext(conditionalRequest, conditionalResponse, "/avatars/:id", webtest.PathParams{"id": "42"})
	if err := controller.Show(conditionalContext); err != nil {
		t.Fatalf("controller.Show(conditional): %v", err)
	}
	if conditionalResponse.Code != http.StatusNotModified || conditionalResponse.Body.Len() != 0 {
		t.Fatalf("conditional response = status %d body %q, want 304 with no body", conditionalResponse.Code, conditionalResponse.Body.String())
	}
}
`

const invoiceValidationBehaviorProbe = `package invoices

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/goforj/web/webtest"
)

// TestAtlasInvoiceValidationBehavior proves malformed, invalid, and valid payloads remain distinct.
func TestAtlasInvoiceValidationBehavior(t *testing.T) {
	tests := []struct {
		name       string
		body       string
		wantStatus int
	}{
		{name: "malformed", body: ` + "`" + `{"customer_id":` + "`" + `, wantStatus: http.StatusBadRequest},
		{name: "invalid", body: ` + "`" + `{"customer_id":" ","total_cents":0}` + "`" + `, wantStatus: http.StatusUnprocessableEntity},
		{name: "valid", body: ` + "`" + `{"customer_id":" customer-42 ","total_cents":12500}` + "`" + `, wantStatus: http.StatusCreated},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			controller := NewController(NewService(&MemoryRepository{}))
			request := httptest.NewRequest(http.MethodPost, "/invoices", strings.NewReader(test.body))
			request.Header.Set("Content-Type", "application/json")
			response := httptest.NewRecorder()
			requestContext := webtest.NewContext(request, response, "/invoices", nil)
			if err := controller.Store(requestContext); err != nil {
				t.Fatalf("Store(): %v", err)
			}
			if response.Code != test.wantStatus {
				t.Fatalf("status = %d, want %d", response.Code, test.wantStatus)
			}
			if test.name == "valid" {
				var created Invoice
				if err := json.Unmarshal(response.Body.Bytes(), &created); err != nil {
					t.Fatalf("decode created invoice: %v", err)
				}
				if created.CustomerID != "customer-42" || created.TotalCents != 12500 {
					t.Fatalf("created = %#v", created)
				}
			}
		})
	}
}
`

const transferTransactionBehaviorProbe = `package accounts

import (
	"context"
	"testing"

	"gorm.io/driver/sqlite"
	"gorm.io/gorm"
)

// TestAtlasTransferTransactionBehavior proves debit and credit commit or roll back together.
func TestAtlasTransferTransactionBehavior(t *testing.T) {
	db, err := gorm.Open(sqlite.Open("file::memory:?cache=shared"), &gorm.Config{})
	if err != nil {
		t.Fatalf("open database: %v", err)
	}
	if err := db.AutoMigrate(&Account{}); err != nil {
		t.Fatalf("migrate accounts: %v", err)
	}
	if err := db.Create(&[]Account{{ID: "from", BalanceCents: 1000}, {ID: "to", BalanceCents: 250}}).Error; err != nil {
		t.Fatalf("seed accounts: %v", err)
	}
	service := NewService(newRepository(db))
	if err := service.Transfer(context.Background(), "from", "to", 200); err != nil {
		t.Fatalf("Transfer(): %v", err)
	}
	atlasAssertBalance(t, db, "from", 800)
	atlasAssertBalance(t, db, "to", 450)
	if err := service.Transfer(context.Background(), "from", "to", 0); err == nil {
		t.Fatal("Transfer() accepted zero amount")
	}
	atlasAssertBalance(t, db, "from", 800)
	atlasAssertBalance(t, db, "to", 450)
	if err := service.Transfer(context.Background(), "from", "to", -100); err == nil {
		t.Fatal("Transfer() accepted negative amount")
	}
	atlasAssertBalance(t, db, "from", 800)
	atlasAssertBalance(t, db, "to", 450)
	if err := service.Transfer(context.Background(), "from", "missing", 100); err == nil {
		t.Fatal("Transfer() accepted a missing destination")
	}
	atlasAssertBalance(t, db, "from", 800)
}

// atlasAssertBalance reads durable state outside the transaction callback.
func atlasAssertBalance(t *testing.T, db *gorm.DB, id string, want int64) {
	t.Helper()
	var account Account
	if err := db.First(&account, "id = ?", id).Error; err != nil {
		t.Fatalf("find %s: %v", id, err)
	}
	if account.BalanceCents != want {
		t.Fatalf("balance %s = %d, want %d", id, account.BalanceCents, want)
	}
}
`
