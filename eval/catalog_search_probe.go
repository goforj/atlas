package eval

const catalogSearchBehaviorProbe = `package catalog

import (
 "context"
 "errors"
 "testing"

 "github.com/goforj/search"
 "github.com/goforj/search/driver/blevesearch"
)

// atlasCatalogBackend observes root API calls while executing a real embedded backend.
type atlasCatalogBackend struct {
 *blevesearch.Driver
 searches int
 writes int
 waits int
 failWait bool
}

// Search records calls on the executed application path.
func (backend *atlasCatalogBackend) Search(ctx context.Context, definition search.Definition, query search.Query) (search.Result, error) {
 backend.searches++
 return backend.Driver.Search(ctx, definition, query)
}

// Upsert records writes without substituting a fake index.
func (backend *atlasCatalogBackend) Upsert(ctx context.Context, definition search.Definition, document search.Document) (search.Receipt, error) {
 backend.writes++
 return backend.Driver.Upsert(ctx, definition, document)
}

// Wait allows a terminal visibility failure to be checked independently of acceptance.
func (backend *atlasCatalogBackend) Wait(ctx context.Context, receipt search.Receipt) (search.Outcome, error) {
 backend.waits++
 if backend.failWait { return search.Outcome{State: search.OutcomeFailed, Error: search.ErrMutationFailed}, nil }
 return backend.Driver.Wait(ctx, receipt)
}

// TestAtlasCatalogSearchBehavior exercises filters, deterministic pages, replacement visibility, and actual library calls.
func TestAtlasCatalogSearchBehavior(t *testing.T) {
 ctx := context.Background()
 driver, err := blevesearch.Open(ctx, blevesearch.Config{Memory: true})
 if err != nil { t.Fatal(err) }
 t.Cleanup(func() { _ = driver.Close() })
 if err := driver.CreateIndex(ctx, Definition()); err != nil { t.Fatal(err) }
 backend := &atlasCatalogBackend{Driver: driver}
 index, err := search.NewIndex(backend, Definition())
 if err != nil { t.Fatal(err) }
 service := NewService(index)
 for _, product := range []Product{
  {ID:"p3", Description:"Red trail boot", Category:"outdoors"},
  {ID:"p1", Description:"Red hiking boot", Category:"outdoors"},
  {ID:"p2", Description:"Red city boot", Category:"fashion"},
  {ID:"p4", Description:"Blue hiking shoe", Category:"outdoors"},
 } {
  if err := service.IndexProduct(ctx, product); err != nil { t.Fatal(err) }
 }
 for offset, id := range []string{"p1", "p3"} {
  page, err := service.Find(ctx, "red boot", "outdoors", uint64(offset), 1)
  if err != nil { t.Fatal(err) }
  if page.Total.Value != 2 || page.Total.Relation != search.TotalExact || len(page.Hits) != 1 || string(page.Hits[0].ID) != id {
   t.Fatalf("category pagination offset %d: %#v", offset, page)
  }
 }
 empty, err := service.Find(ctx, "red boot", "outdoors", 2, 1)
 if err != nil || empty.Total.Value != 2 || len(empty.Hits) != 0 { t.Fatalf("end page = %#v, %v", empty, err) }
 excluded, err := service.Find(ctx, "red boot", "missing", 0, 10)
 if err != nil || excluded.Total.Value != 0 { t.Fatalf("category isolation = %#v, %v", excluded, err) }
 if err := service.IndexProduct(ctx, Product{ID:"p1", Description:"Green camp tent", Category:"camping"}); err != nil { t.Fatal(err) }
 old, err := service.Find(ctx, "red boot", "outdoors", 0, 10)
 if err != nil || old.Total.Value != 1 || len(old.Hits) != 1 || old.Hits[0].ID != "p3" { t.Fatalf("replacement retained stale match: %#v, %v", old, err) }
 updated, err := service.Find(ctx, "green tent", "camping", 0, 10)
 if err != nil || updated.Total.Value != 1 || len(updated.Hits) != 1 || updated.Hits[0].ID != "p1" { t.Fatalf("replacement invisible: %#v, %v", updated, err) }
 backend.failWait = true
 if err := service.IndexProduct(ctx, Product{ID:"failed", Description:"accepted write", Category:"outdoors"}); !errors.Is(err, search.ErrMutationFailed) { t.Fatalf("terminal failure reported as visible: %v", err) }
 canceled, cancel := context.WithCancel(ctx)
 cancel()
 if _, err := service.Find(canceled, "red", "outdoors", 0, 1); !errors.Is(err, context.Canceled) { t.Fatalf("query cancellation: %v", err) }
 if err := service.IndexProduct(canceled, Product{ID:"cancelled", Description:"red boot", Category:"outdoors"}); !errors.Is(err, context.Canceled) { t.Fatalf("write cancellation: %v", err) }
 if _, err := service.Find(ctx, "red", "outdoors", 0, 100000); err == nil { t.Fatal("unbounded page accepted") }
}

// TestAtlasCatalogSearchLibraryPath records adoption separately from shopper-visible behavior.
func TestAtlasCatalogSearchLibraryPath(t *testing.T) {
 ctx := context.Background()
 driver, err := blevesearch.Open(ctx, blevesearch.Config{Memory: true})
 if err != nil { t.Fatal(err) }
 t.Cleanup(func() { _ = driver.Close() })
 if err := driver.CreateIndex(ctx, Definition()); err != nil { t.Fatal(err) }
 backend := &atlasCatalogBackend{Driver: driver}
 index, err := search.NewIndex(backend, Definition())
 if err != nil { t.Fatal(err) }
 service := NewService(index)
 if err := service.IndexProduct(ctx, Product{ID:"path", Description:"blue trail shoe", Category:"outdoors"}); err != nil { t.Fatal(err) }
 if _, err := service.Find(ctx, "trail", "outdoors", 0, 1); err != nil { t.Fatal(err) }
 if backend.searches != 1 || backend.writes != 1 || backend.waits != 1 {
  t.Fatalf("library calls on application path: search=%d upsert=%d wait=%d", backend.searches, backend.writes, backend.waits)
 }
}
`
