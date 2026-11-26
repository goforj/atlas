package eval

// payloadAppRuntimeProbe checks the configured key set through actual App constructor injection.
const payloadAppRuntimeProbe = `package partnerpayloads

import (
 "context"
 "time"
 "bytes"
 "encoding/base64"
 "os"
 "os/exec"
 "path/filepath"
 "strings"
 "testing"
)

// TestAtlasResourceApp proves the App graph consumes the configured cipher without exposing key material.
func TestAtlasResourceApp(t *testing.T) {
 root := atlasAppRoot(t)
 key := func(value byte) string { return "base64:" + base64.StdEncoding.EncodeToString(bytes.Repeat([]byte{value},32)) }
 environment := []string{"APP_KEY="+key(0x22),"APP_PREVIOUS_KEYS="+key(0x11)}
 atlasBuildResourceApp(t,root,environment)
 if output,err := atlasRunResourceApp(t,root,environment); err != nil { t.Fatalf("configured encryption App failed: %v\n%s",err,output) }
 for _,environment := range [][]string{
  {"APP_KEY=", "APP_PREVIOUS_KEYS="},
  {"APP_KEY=base64:???", "APP_PREVIOUS_KEYS="},
  {"APP_KEY="+key(0x22), "APP_PREVIOUS_KEYS=base64:???"},
 } {
  output,err := atlasRunResourceApp(t,root,environment)
  if err == nil { t.Fatal("invalid configured App reached readiness") }
  for _,entry := range environment { value := strings.SplitN(entry,"=",2)[1]; if value != "" && strings.Contains(output,value) { t.Fatal("App key failure exposed configured key material") } }
 }
}
`

// payloadAppRuntimeCommand observes the same repository and cipher resolved for the App-owned service.
const payloadAppRuntimeCommand = `package cmd

import (
 "context"
 "bytes"
 "fmt"
 "github.com/goforj/crypt"
 "example.com/invoiceeval/internal/partnerpayloads"
)

// HelloWorldCmd exercises payload protection through the existing App command registration.
type HelloWorldCmd struct { service *partnerpayloads.Service; repository partnerpayloads.Repository }

// Signature preserves the registered command name used by the isolated supervisor.
func (*HelloWorldCmd) Signature() string { return ` + "`name:\"hello:world\" help:\"Exercise configured payload encryption\"`" + ` }

// NewHelloWorldCmd requires Wire to resolve the submitted service and shared repository providers.
func NewHelloWorldCmd(service *partnerpayloads.Service,repository partnerpayloads.Repository) *HelloWorldCmd { return &HelloWorldCmd{service:service,repository:repository} }

// Run validates current-key writes, old-key reads, and authentication through the configured App service.
func (command *HelloWorldCmd) Run() error {
 ctx := context.Background()
 current,err := crypt.New(bytes.Repeat([]byte{0x22},32)); if err != nil { return err }
 previous,err := crypt.New(bytes.Repeat([]byte{0x11},32)); if err != nil { return err }
 if err := command.service.Save(ctx,"app-current","synthetic App payload"); err != nil { return err }
 stored,err := command.repository.Get(ctx,"app-current"); if err != nil { return err }
 if value,err := current.Decrypt(stored); err != nil || value != "synthetic App payload" { return fmt.Errorf("App did not use its current key") }
 if _,err := previous.Decrypt(stored); err == nil { return fmt.Errorf("App encrypted a new record with its previous key") }
 old,err := previous.Encrypt("synthetic historical payload"); if err != nil { return err }
 if err := command.repository.Put(ctx,"app-old",old); err != nil { return err }
 if value,err := command.service.Load(ctx,"app-old"); err != nil || value != "synthetic historical payload" { return fmt.Errorf("App lost configured previous-key reads") }
 if err := command.repository.Put(ctx,"app-corrupt","not-an-authenticated-envelope"); err != nil { return err }
 if value,err := command.service.Load(ctx,"app-corrupt"); err == nil || value != "" { return fmt.Errorf("App returned unauthenticated plaintext") }
 return nil
}
`
