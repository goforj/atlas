package eval

import (
	"net"
	"os"
	"strconv"
	"strings"
)

// tenantRateLimitAppRuntimeContract captures only a supervisor-selected loopback Redis endpoint.
func tenantRateLimitAppRuntimeContract() commandContract {
	address := strings.TrimSpace(os.Getenv("ATLAS_EVAL_REDIS_ADDR"))
	host, port, err := net.SplitHostPort(address)
	number, portErr := strconv.Atoi(port)
	ip := net.ParseIP(host)
	if err != nil || portErr != nil || number < 1 || number > 65535 || ip == nil || !ip.IsLoopback() {
		address = ""
	}
	source := tenantRateLimitAppRuntimeProbe + tenantNativeAppOutputSource + "\nconst atlasRedisAddress = " + strconv.Quote(address) + "\n"
	return appResourceRuntimeContract("tenant-rate-limit-app-runtime", "requests", source, tenantRateLimitAppRuntimeCommand)
}

// tenantRateLimitAppRuntimeProbe drives two generated App processes through their actual HTTP listeners.
const tenantRateLimitAppRuntimeProbe = `package requests
import (
 "context"
 "fmt"
 "io"
 "net"
 "net/http"
 "os"
 "os/exec"
 "path/filepath"
 "strconv"
 "strings"
 "sync"
 "testing"
 "time"
)
// atlasNativeRedisProxy isolates a connection outage to this verifier's App replicas.
func atlasNativeRedisProxy(t *testing.T) (string,func()) {
 t.Helper()
 listener,err:=net.Listen("tcp","127.0.0.1:0");if err!=nil {t.Fatal(err)}
 var mutex sync.Mutex
 var once sync.Once
 connections:=map[net.Conn]struct{}{}
 closed:=false
 disconnect:=func(){once.Do(func(){
  listener.Close()
  mutex.Lock();defer mutex.Unlock()
  closed=true
  for connection:=range connections {connection.Close()}
 })}
 t.Cleanup(disconnect)
 go func(){
  for {
   downstream,err:=listener.Accept();if err!=nil {return}
   go func(){
    upstream,err:=net.DialTimeout("tcp",atlasRedisAddress,2*time.Second)
    if err!=nil {downstream.Close();return}
    mutex.Lock()
    if closed {mutex.Unlock();downstream.Close();upstream.Close();return}
    connections[downstream]=struct{}{};connections[upstream]=struct{}{}
    mutex.Unlock()
    defer func(){
     downstream.Close();upstream.Close()
     mutex.Lock();delete(connections,downstream);delete(connections,upstream);mutex.Unlock()
    }()
    go func(){io.Copy(upstream,downstream);upstream.Close()}()
    io.Copy(downstream,upstream)
   }()
  }
 }()
 return listener.Addr().String(),disconnect
}

// TestAtlasResourceApp requires a native shared backend and the App-owned route and resource graph.
func TestAtlasResourceApp(t *testing.T) {
 if atlasRedisAddress == "" { t.Fatal("native App check requires supervisor ATLAS_EVAL_REDIS_ADDR as a loopback Redis host:port") }
 connection, err := net.DialTimeout("tcp",atlasRedisAddress,2*time.Second)
 if err != nil { t.Fatalf("native Redis prerequisite: %v",err) }
 connection.SetDeadline(time.Now().Add(2*time.Second))
 if _,err := io.WriteString(connection,"INFO server\r\n");err!=nil { connection.Close();t.Fatal(err) }
 info := make([]byte,8192)
 size,err := connection.Read(info)
 connection.Close()
 if err!=nil || !strings.Contains(string(info[:size]),"redis_version:") { t.Fatalf("native Redis INFO server: %v %q",err,info[:size]) }
 proxyAddress,disconnect:=atlasNativeRedisProxy(t)
 root := atlasAppRoot(t)
 environment := []string{"CACHE_DRIVER=memory","CACHE_REQUESTS_DRIVER=redis","CACHE_REQUESTS_ADDR="+proxyAddress,"CACHE_REQUESTS_PREFIX=atlas-app-"+filepath.Base(t.TempDir())}
 atlasBuildResourceApp(t,root,environment)
 binary := filepath.Join(t.TempDir(),"atlas-app")
 if output,err := atlasRunResourceTool(t,root,environment,"go","build","-o",binary,"./cmd/app");err!=nil { t.Fatalf("compile HTTP App: %v\n%s",err,output) }
 client := &http.Client{Timeout:10*time.Second}
 request := func(base,path,token,asserted string) (int,string,string) {
  t.Helper()
  req,err := http.NewRequestWithContext(t.Context(),http.MethodGet,base+path,nil);if err!=nil { t.Fatal(err) }
  req.Header.Set("Authorization",token);req.Header.Set("X-Tenant-ID",asserted)
  response,err:=client.Do(req);if err!=nil { t.Fatal(err) };defer response.Body.Close()
  body,err:=io.ReadAll(io.LimitReader(response.Body,8192));if err!=nil { t.Fatal(err) }
  return response.StatusCode,response.Header.Get("Retry-After"),string(body)
 }
 bases:=make([]string,0,2)
 for index:=0;index<2;index++ {
  listener,err:=net.Listen("tcp","127.0.0.1:0");if err!=nil {t.Fatal(err)}
  port:=strconv.Itoa(listener.Addr().(*net.TCPAddr).Port);listener.Close()
  ctx,cancel:=context.WithCancel(t.Context())
  command:=exec.CommandContext(ctx,binary,"hello:world")
  command.Dir=root;command.Env=append(append(os.Environ(),environment...),"ATLAS_APP_PORT="+port)
  command.WaitDelay=2*time.Second
  output:=&atlasNativeAppOutput{cancel:cancel}
  command.Stdout=output;command.Stderr=output
  if err:=command.Start();err!=nil {cancel();t.Fatal(err)}
  t.Cleanup(func(){
   cancel();_ = command.Wait()
   if _,exceeded:=output.snapshot();exceeded {t.Error("App output exceeds 1048576 bytes")}
  })
  base:="http://127.0.0.1:"+port
  ready:=false
  deadline:=time.Now().Add(20*time.Second)
  for time.Now().Before(deadline) {
   response,err:=client.Get(base+"/api/v1/health")
   if err==nil {io.Copy(io.Discard,response.Body);response.Body.Close();if response.StatusCode==http.StatusOK {ready=true;break}}
   select {case <-t.Context().Done():t.Fatal(t.Context().Err());case <-time.After(20*time.Millisecond):}
  }
  if !ready { body,_:=output.snapshot();t.Fatalf("App HTTP not ready: %s",body) }
  bases=append(bases,base)
 }
 // A native App uses its production clock; begin away from the real minute rollover.
 if remaining:=time.Until(time.Now().Truncate(time.Minute).Add(time.Minute));remaining<5*time.Second {
  select {case <-t.Context().Done():t.Fatal(t.Context().Err());case <-time.After(remaining):}
 }
 for _,base:=range bases {
  if status,_,body:=request(base,"/api/v1/exports","Bearer tenant-a-token","tenant-b");status!=http.StatusOK {t.Fatalf("allowed export=%d %s",status,body)}
 }
 status,retry,body:=request(bases[0],"/api/v1/exports","Bearer tenant-a-token","tenant-b")
 seconds,err:=strconv.Atoi(retry)
 if status!=http.StatusTooManyRequests || err!=nil || seconds<1 || seconds>60 || !strings.Contains(body,"rate limit exceeded") {t.Fatalf("shared App budget=%d retry=%q %s",status,retry,body)}
 if status,_,body:=request(bases[1],"/api/v1/exports","Bearer tenant-b-token","tenant-a");status!=http.StatusOK {t.Fatalf("tenant isolation=%d %s",status,body)}
 for _,token:=range []string{"","Bearer invalid"} {
  if status,_,body:=request(bases[0],"/api/v1/exports",token,"tenant-a");status!=http.StatusUnauthorized {t.Fatalf("unverified export=%d %s",status,body)}
 }
 if status,_,body:=request(bases[0],"/api/v1/health","","");status!=http.StatusOK {t.Fatalf("public health=%d %s",status,body)}
 disconnect()
 if status,_,body:=request(bases[1],"/api/v1/exports","Bearer tenant-b-token","");status!=http.StatusServiceUnavailable || !strings.Contains(body,"rate limit unavailable") {t.Fatalf("native counter outage=%d %s",status,body)}
 if status,_,body:=request(bases[0],"/api/v1/health","","");status!=http.StatusOK {t.Fatalf("health during outage=%d %s",status,body)}
 fmt.Println("atlas-tenant-app-ok")
}
`

// tenantRateLimitAppRuntimeCommand serves the actual App router supplied by the active Wire graph.
const tenantRateLimitAppRuntimeCommand = `package cmd
import (
 "context"
 "os"
 "example.com/invoiceeval/internal/http"
)
// HelloWorldCmd uses the existing generated entry point to serve the App's registered routes.
type HelloWorldCmd struct { server *http.Server }
// Signature preserves the existing command registration.
func (*HelloWorldCmd) Signature() string { return ` + "`name:\"hello:world\" help:\"Exercise tenant App routes\"`" + ` }
// NewHelloWorldCmd borrows the framework server through constructor injection.
func NewHelloWorldCmd(server *http.Server) *HelloWorldCmd { return &HelloWorldCmd{server:server} }
// Run delegates the listener and middleware lifecycle to the actual App server.
func (command *HelloWorldCmd) Run(ctx context.Context) error { return command.server.Serve(ctx,"127.0.0.1",os.Getenv("ATLAS_APP_PORT")) }
`

// tenantNativeAppOutputSource applies one shared bounded sink to each App's stdout and stderr.
const tenantNativeAppOutputSource = `
// atlasNativeAppOutput bounds candidate output while readiness and child pumps run concurrently.
type atlasNativeAppOutput struct {
 mutex sync.Mutex
 body strings.Builder
 exceeded bool
 cancel context.CancelFunc
}
// Write cancels the child on aggregate overflow and retains no bytes above the ceiling.
func (output *atlasNativeAppOutput) Write(body []byte) (int,error) {
 output.mutex.Lock()
 defer output.mutex.Unlock()
 size:=len(body)
 remaining:=(1<<20)-output.body.Len()
 if size>remaining {
  output.body.Write(body[:remaining])
  output.exceeded=true
  output.cancel()
 } else {output.body.Write(body)}
 return size,nil
}
// snapshot returns bounded diagnostics without racing the active child output pump.
func (output *atlasNativeAppOutput) snapshot() (string,bool) {
 output.mutex.Lock()
 defer output.mutex.Unlock()
 return output.body.String(),output.exceeded
}
`
