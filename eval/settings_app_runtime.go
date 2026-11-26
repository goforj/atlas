package eval

// settingsAppRuntimeProbe exercises the submitted controller through the App-owned HTTP server and active Wire graph.
const settingsAppRuntimeProbe = `package settings
import (
 "context"
 "time"
 "os"
 "os/exec"
 "path/filepath"
 "strings"
 "testing"
 "net"
 "net/http"
 "encoding/json"
 "io"
 "fmt"
)
// TestAtlasResourceApp requires the actual App router to preserve presence and stable field errors.
func TestAtlasResourceApp(t *testing.T) {
 t.Run("bounded-http-response",func(t *testing.T) {
  var got any
  err:=atlasDecodeSettingsResponse(strings.NewReader(strings.Repeat("x",8193)),&got)
  if err==nil || !strings.Contains(err.Error(),"response exceeds") {t.Fatalf("oversized response was not rejected: %v",err)}
 })
 root := atlasAppRoot(t)
 listener,err := net.Listen("tcp","127.0.0.1:0"); if err != nil { t.Fatal(err) }
 port := listener.Addr().(*net.TCPAddr).Port; listener.Close()
 env := []string{fmt.Sprintf("ATLAS_SETTINGS_PORT=%d",port)}
 atlasBuildResourceApp(t,root,env)
 done := make(chan struct{})
 var output string; var runErr error

 go func(){ output,runErr = atlasRunResourceTool(t,root,env,"go","run","./cmd/app","hello:world"); close(done) }()
 defer func(){ select { case <-done: case <-time.After(20*time.Second): } }()
 client := &http.Client{Timeout:time.Second}
 url := fmt.Sprintf("http://127.0.0.1:%d/api/v1/customers/settings",port)
 deadline := time.Now().Add(30*time.Second)
 for {
  request,_ := http.NewRequest(http.MethodPatch,url,strings.NewReader("{}")); request.Header.Set("Content-Type","application/json")
  response,err := client.Do(request)
  if err == nil { response.Body.Close(); break }
  select { case <-done: t.Fatalf("App exited: %v\n%s",runErr,output); default: }
  if time.Now().After(deadline) { t.Fatalf("App HTTP readiness: %v",err) }; time.Sleep(50*time.Millisecond)
 }
 for _,test := range []struct{body string; status int; code string}{
  {body:"{\"limit\":0,\"enabled\":false,\"note\":null}",status:200},
  {body:"{\"limit\":101}",status:422,code:"max"},
  {body:"{}",status:200},
 } {
  request,_ := http.NewRequest(http.MethodPatch,url,strings.NewReader(test.body)); request.Header.Set("Content-Type","application/json")
  response,err := client.Do(request); if err != nil { t.Fatal(err) }
  var body struct { Name string; Limit int; Enabled bool; Note *string; Error string; Fields map[string]string }
  err = atlasDecodeSettingsResponse(response.Body,&body); response.Body.Close()
  if err != nil || response.StatusCode != test.status { t.Fatalf("App PATCH status=%d body=%#v error=%v",response.StatusCode,body,err) }
  if test.status==200 && (body.Name!="Existing" || body.Limit!=0 || body.Enabled || body.Note!=nil) { t.Fatalf("App presence/state=%#v",body) }
  if test.status==422 && (body.Error!="validation_failed" || body.Fields["limit"]!=test.code) { t.Fatalf("App field errors=%#v",body) }
 }
}
// atlasDecodeSettingsResponse rejects oversized candidate-controlled bodies before JSON decoding can allocate from them.
func atlasDecodeSettingsResponse(body io.Reader, target any) error {
 data,err:=io.ReadAll(io.LimitReader(body,8193));if err!=nil {return err}
 if len(data)>8192 {return fmt.Errorf("settings response exceeds %d bytes",8192)}
 return json.Unmarshal(data,target)
}
`

// settingsAppRuntimeCommand consumes the generated HTTP server without rebuilding the submitted route registry.
const settingsAppRuntimeCommand = `package cmd
import (
 "context"
 "os"
 "time"
 "example.com/invoiceeval/internal/http"
)
// HelloWorldCmd exposes the existing App HTTP server to the supervisor.
type HelloWorldCmd struct { server *http.Server }
// NewHelloWorldCmd requires the active App route and controller providers.
func NewHelloWorldCmd(server *http.Server) *HelloWorldCmd { return &HelloWorldCmd{server:server} }
// Signature preserves the existing command registration.
func (*HelloWorldCmd) Signature() string { return ` + "`" + `name:"hello:world" help:"Settings runtime verification"` + "`" + ` }
// Run serves the real App router within a bounded verification window.
func (command *HelloWorldCmd) Run() error {
 ctx,cancel := context.WithTimeout(context.Background(),15*time.Second); defer cancel()
 return command.server.Serve(ctx,"127.0.0.1",os.Getenv("ATLAS_SETTINGS_PORT"))
}
`
