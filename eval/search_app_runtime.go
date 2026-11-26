package eval

const catalogSearchAppRuntimeTest = `package catalog

import (
 "context"
 "time"
 "os"
 "os/exec"
 "path/filepath"
 "strings"
 "testing"
)

// TestAtlasResourceApp verifies search through the generated App's actual Wire graph.
func TestAtlasResourceApp(t *testing.T) {
 root := atlasAppRoot(t)
 environment := []string{"SEARCH_CATALOG_PATH="+filepath.Join(t.TempDir(), "catalog")}
 if output, err := atlasRunResourceTool(t, root, environment, "go", "run", "./tools/search-provision"); err != nil { t.Fatalf("provision catalog before App startup: %v\n%s", err, output) }
 atlasBuildResourceApp(t, root, environment)
 output, err := atlasRunResourceApp(t, root, environment)
 if err != nil { t.Fatalf("run catalog App: %v\n%s", err, output) }
 if !strings.Contains(string(output), "atlas-catalog-search-app-runtime-ok") { t.Fatalf("catalog App marker absent:\n%s", output) }
}
`

const catalogSearchAppRuntimeCommand = `package cmd

import (
 "context"
 "fmt"
 "time"

 "example.com/invoiceeval/internal/catalog"
 "github.com/goforj/search"
)

// HelloWorldCmd exercises the catalog service constructed by the native App graph.
type HelloWorldCmd struct { service *catalog.Service }

// Signature preserves the generated command registration.
func (*HelloWorldCmd) Signature() string { return ` + "`name:\"hello:world\" help:\"Verify catalog App runtime\"`" + ` }

// NewHelloWorldCmd requires the candidate's catalog provider to participate in App construction.
func NewHelloWorldCmd(service *catalog.Service) *HelloWorldCmd { return &HelloWorldCmd{service: service} }

// Run verifies real index visibility through the App-owned service and backend.
func (command *HelloWorldCmd) Run() error {
 ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
 defer cancel()
 for _, product := range []catalog.Product{
  {ID:"p3", Description:"Red trail boot", Category:"outdoors"},
  {ID:"p1", Description:"Red hiking boot", Category:"outdoors"},
  {ID:"p2", Description:"Red city boot", Category:"fashion"},
  {ID:"p4", Description:"Blue hiking shoe", Category:"outdoors"},
 } {
  if err := command.service.IndexProduct(ctx, product); err != nil { return err }
 }
 for offset, id := range []string{"p1", "p3"} {
  page, err := command.service.Find(ctx, "red boot", "outdoors", uint64(offset), 1)
  if err != nil { return err }
  if page.Total.Value != 2 || page.Total.Relation != search.TotalExact || len(page.Hits) != 1 || string(page.Hits[0].ID) != id { return fmt.Errorf("category pagination offset %d: %#v", offset, page) }
 }
 end, err := command.service.Find(ctx, "red boot", "outdoors", 2, 1)
 if err != nil { return err }
 if end.Total.Value != 2 || end.Total.Relation != search.TotalExact || len(end.Hits) != 0 { return fmt.Errorf("end page: %#v", end) }
 excluded, err := command.service.Find(ctx, "red boot", "missing", 0, 10)
 if err != nil { return err }
 if excluded.Total.Value != 0 || excluded.Total.Relation != search.TotalExact || len(excluded.Hits) != 0 { return fmt.Errorf("category isolation: %#v", excluded) }
 if err := command.service.IndexProduct(ctx, catalog.Product{ID:"p1", Description:"Green camp tent", Category:"camping"}); err != nil { return err }
 old, err := command.service.Find(ctx, "red boot", "outdoors", 0, 10)
 if err != nil { return err }
 if old.Total.Value != 1 || old.Total.Relation != search.TotalExact || len(old.Hits) != 1 || old.Hits[0].ID != "p3" { return fmt.Errorf("replacement retained stale match: %#v", old) }
 updated, err := command.service.Find(ctx, "green tent", "camping", 0, 10)
 if err != nil { return err }
 if updated.Total.Value != 1 || updated.Total.Relation != search.TotalExact || len(updated.Hits) != 1 || updated.Hits[0].ID != "p1" { return fmt.Errorf("replacement invisible: %#v", updated) }
 fmt.Println("atlas-catalog-search-app-runtime-ok")
 return nil
}
`
