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
 product := catalog.Product{ID:"p1", Description:"Red hiking boot", Category:"outdoors"}
 if err := command.service.IndexProduct(ctx, product); err != nil { return err }
 page, err := command.service.Find(ctx, "red boot", "outdoors", 0, 1)
 if err != nil { return err }
 if page.Total.Value != 1 || page.Total.Relation != search.TotalExact || len(page.Hits) != 1 || page.Hits[0].Value != product {
  return fmt.Errorf("expected catalog product through the App index: %#v", page)
 }
 fmt.Println("atlas-catalog-search-app-runtime-ok")
 return nil
}
`
