package eval

// dispatchEnvironmentContract separates startup correctness from optional env API attribution.
func dispatchEnvironmentContract() surfaceContract {
	return surfaceContract{
		id:                  "configure-dispatch-startup/v1",
		allowedChanges:      []string{"internal/dispatch/*.go", "app/lifecycle.go", "app/wire/inject_services_app.go"},
		qualityTestPatterns: []string{"internal/dispatch/*_test.go"},
		sources: []sourceContract{
			{id: "dispatch-config-composition", paths: []string{"app/wire/inject_services_app.go"}, identifiers: []string{"LoadConfig", "NewService"}},
			{id: "dispatch-env-api", kind: RequirementQuality, paths: []string{"internal/dispatch/*.go"}, stringLiterals: []string{"github.com/goforj/env/v2"}, declarations: []declarationContract{{name: "LoadConfig", selectorCalls: []string{"Load", "WithPrefix", "GetInt", "GetDuration", "GetBool"}}}},
		},
		commands: append(standardSurfaceCommands(dispatchEnvironmentBehaviorContract()), appResourceRuntimeContract("dispatch-app-runtime", "dispatch", dispatchEnvironmentProbe, dispatchAppRuntimeCommand)),
	}
}

// dispatchEnvironmentBehaviorContract shares supervised definitions while selecting only the independent loader cases.
func dispatchEnvironmentBehaviorContract() commandContract {
	contract := appResourceRuntimeContract("dispatch-startup-behavior", "dispatch", dispatchEnvironmentProbe, dispatchAppRuntimeCommand)
	contract.arguments = []string{"go", "test", "./internal/dispatch", "-run", "^TestAtlasDispatchEnvironmentBehavior$", "-count=1"}
	return contract
}

const dispatchEnvironmentProbe = `package dispatch_test

import (
 "context"
 "encoding/json"
 "fmt"
 "os"
 "os/exec"
 "path/filepath"
 "strings"
 "testing"
 "time"

 "example.com/invoiceeval/internal/dispatch"
)

// atlasDispatchCase describes one independent process environment and expected startup policy.
type atlasDispatchCase struct {
 file string
 process []string
 want dispatch.Config
 wantError string
}

// atlasDispatchCases retains fake values while distinguishing required failures from tolerant optional fallbacks.
func atlasDispatchCases() map[string]atlasDispatchCase {
 defaults := dispatch.Config{Token:"fake-env-token-keep-private",Workers:4,Timeout:2*time.Second,Enabled:true,Region:"local"}
 return map[string]atlasDispatchCase{
  "defaults":{process:[]string{"DISPATCH_TOKEN="+defaults.Token,"DISPATCH_WORKERS=4"},want:defaults},
  "file":{file:"DISPATCH_TOKEN=file-private-token\nDISPATCH_WORKERS=2\nDISPATCH_TIMEOUT=3s\nDISPATCH_ENABLED=off\nDISPATCH_REGION=file\n",want:dispatch.Config{Token:"file-private-token",Workers:2,Timeout:3*time.Second,Region:"file"}},
  "process-precedence":{file:"DISPATCH_TOKEN=file-private-token\nDISPATCH_WORKERS=2\nDISPATCH_TIMEOUT=3s\nDISPATCH_ENABLED=off\nDISPATCH_REGION=file\n",process:[]string{"DISPATCH_TOKEN="+defaults.Token,"DISPATCH_WORKERS=7","DISPATCH_TIMEOUT=5s","DISPATCH_ENABLED= ON ","DISPATCH_REGION=deployment"},want:dispatch.Config{Token:defaults.Token,Workers:7,Timeout:5*time.Second,Enabled:true,Region:"deployment"}},
  "empty-process-token":{file:"DISPATCH_TOKEN=file-private-token\nDISPATCH_WORKERS=2\n",process:[]string{"DISPATCH_TOKEN="},wantError:"DISPATCH_TOKEN"},
  "missing-token":{process:[]string{"DISPATCH_WORKERS=4"},wantError:"DISPATCH_TOKEN"},
  "blank-token":{process:[]string{"DISPATCH_TOKEN=   ","DISPATCH_WORKERS=4"},wantError:"DISPATCH_TOKEN"},
  "missing-workers":{process:[]string{"DISPATCH_TOKEN="+defaults.Token},wantError:"DISPATCH_WORKERS"},
  "malformed-workers":{process:[]string{"DISPATCH_TOKEN="+defaults.Token,"DISPATCH_WORKERS=bad-workers-private"},wantError:"DISPATCH_WORKERS"},
  "overflow-workers":{process:[]string{"DISPATCH_TOKEN="+defaults.Token,"DISPATCH_WORKERS=99999999999999999999999999999"},wantError:"DISPATCH_WORKERS"},
  "zero-workers":{process:[]string{"DISPATCH_TOKEN="+defaults.Token,"DISPATCH_WORKERS=0"},wantError:"DISPATCH_WORKERS"},
  "negative-workers":{process:[]string{"DISPATCH_TOKEN="+defaults.Token,"DISPATCH_WORKERS=-1"},wantError:"DISPATCH_WORKERS"},
  "too-many-workers":{process:[]string{"DISPATCH_TOKEN="+defaults.Token,"DISPATCH_WORKERS=65"},wantError:"DISPATCH_WORKERS"},
  "maximum-workers":{process:[]string{"DISPATCH_TOKEN="+defaults.Token,"DISPATCH_WORKERS=64"},want:dispatch.Config{Token:defaults.Token,Workers:64,Timeout:2*time.Second,Enabled:true,Region:"local"}},
  "malformed-optionals":{process:[]string{"DISPATCH_TOKEN="+defaults.Token,"DISPATCH_WORKERS=4","DISPATCH_TIMEOUT=bad-duration-private","DISPATCH_ENABLED=bad-boolean-private","DISPATCH_REGION="},want:defaults},
  "zero-timeout":{process:[]string{"DISPATCH_TOKEN="+defaults.Token,"DISPATCH_WORKERS=4","DISPATCH_TIMEOUT=0s"},wantError:"DISPATCH_TIMEOUT"},
  "negative-timeout":{process:[]string{"DISPATCH_TOKEN="+defaults.Token,"DISPATCH_WORKERS=4","DISPATCH_TIMEOUT=-1s"},wantError:"DISPATCH_TIMEOUT"},
  "false":{process:[]string{"DISPATCH_TOKEN="+defaults.Token,"DISPATCH_WORKERS=4","DISPATCH_ENABLED=no"},want:dispatch.Config{Token:defaults.Token,Workers:4,Timeout:2*time.Second,Region:"local"}},
  "token-bytes":{process:[]string{"DISPATCH_TOKEN=  fake-env-token-keep-private  ","DISPATCH_WORKERS=4"},want:dispatch.Config{Token:"  fake-env-token-keep-private  ",Workers:4,Timeout:2*time.Second,Enabled:true,Region:"local"}},
  "malformed-file":{file:"DISPATCH_TOKEN=\"file-private-token\n",wantError:"*"},
 }
}

// TestAtlasDispatchEnvironmentBehavior gives each loader-precedence case a fresh process and private files.
func TestAtlasDispatchEnvironmentBehavior(t *testing.T) {
 executable,err := os.Executable(); if err != nil { t.Fatal(err) }
 t.Run("bounded-child-output",func(t *testing.T) {
  output,err:=atlasRunDispatchChild(t,executable,t.TempDir(),append(os.Environ(),"ATLAS_DISPATCH_NOISY_CHILD=1"),"-test.run=^TestAtlasDispatchEnvironmentNoisyChild$")
  if err==nil || !strings.Contains(err.Error(),"output exceeds") || len(output)>1<<20 {t.Fatalf("noisy child was not bounded: bytes=%d error=%v",len(output),err)}
 })
 for name,test := range atlasDispatchCases() { t.Run(name,func(t *testing.T) {
  root := t.TempDir()
  if err := os.WriteFile(filepath.Join(root,".env"),[]byte("APP_ENV=local\n"+test.file),0600); err != nil { t.Fatal(err) }
  environment := []string{}
  for _,entry := range os.Environ() {
   key,_,_ := strings.Cut(entry,"=")
   if !strings.HasPrefix(key,"DISPATCH_") && key != "APP_ENV" && key != "ENV_DEBUG" && key != "ATLAS_DISPATCH_CASE" { environment = append(environment,entry) }
  }
  environment = append(environment,"APP_ENV=local","ENV_DEBUG=0","ATLAS_DISPATCH_CASE="+name)
  environment = append(environment,test.process...)
  if output,err := atlasRunDispatchChild(t,executable,root,environment,"-test.run=^TestAtlasDispatchEnvironmentChild$","-test.count=1"); err != nil { t.Fatalf("isolated startup failed: %v\n%s",err,output) }
 }) }
}

// atlasRunDispatchChild bounds candidate-controlled startup output before retaining it in the supervisor process.
func atlasRunDispatchChild(t *testing.T, executable, root string, environment []string, args ...string) (string,error) {
 t.Helper()
 ctx,cancel:=context.WithCancel(t.Context()); defer cancel()
 command:=exec.CommandContext(ctx,executable,args...); command.Dir=root;command.Env=environment;command.WaitDelay=2*time.Second
 output:=&atlasResourceOutput{cancel:cancel};command.Stdout=output;command.Stderr=output
 err:=command.Run()
 if output.exceeded {return output.body.String(),fmt.Errorf("dispatch child output exceeds %d bytes",1<<20)}
 return output.body.String(),err
}

// TestAtlasDispatchEnvironmentNoisyChild exercises the real subprocess ceiling without trusting candidate output volume.
func TestAtlasDispatchEnvironmentNoisyChild(t *testing.T) {
 if os.Getenv("ATLAS_DISPATCH_NOISY_CHILD")!="1" {t.Skip("invoked by supervisor output calibration")}
 for {fmt.Print(strings.Repeat("x",32<<10))}
}

// TestAtlasDispatchEnvironmentChild verifies the provider and the real generated Wire initialization path.
func TestAtlasDispatchEnvironmentChild(t *testing.T) {
 name := os.Getenv("ATLAS_DISPATCH_CASE"); if name == "" { t.Skip("invoked by the supervisor parent") }
 test,ok := atlasDispatchCases()[name]; if !ok { t.Fatal("unknown startup case") }
 config,err := dispatch.LoadConfig()

 if test.wantError != "" {
  if err == nil { t.Fatalf("invalid settings did not stop provider and App composition: %v/%v",err,err) }
  if config != (dispatch.Config{}) { t.Fatal("failed provider returned a partially usable config") }
  for _,failure := range []error{err,err} {
   if test.wantError != "*" && !strings.Contains(failure.Error(),test.wantError) { t.Fatalf("error did not identify %q: %v",test.wantError,failure) }
   for _,private := range []string{"fake-env-token-keep-private","file-private-token","bad-workers-private","99999999999999999999999999999"} { if strings.Contains(failure.Error(),private) { t.Fatal("startup error leaked a submitted value") } }
  }
  return
 }
 if err != nil { t.Fatalf("valid settings rejected: %v/%v",err,err) }
 if config != test.want { t.Fatal("typed configuration or precedence differs from the requested startup policy") }
 for _,diagnostic := range []string{config.String(),config.GoString(),fmt.Sprint(config),fmt.Sprintf("%+v",config),fmt.Sprintf("%#v",config)} {
  if strings.Contains(diagnostic,strings.TrimSpace(config.Token)) { t.Fatal("configuration diagnostic leaked or omitted its credential redaction") }
 }
 service := dispatch.NewService(config)
 if err := os.Setenv("DISPATCH_WORKERS","63"); err != nil { t.Fatal(err) }
 if err := os.Setenv("DISPATCH_TOKEN","changed-after-startup"); err != nil { t.Fatal(err) }
 if service.Snapshot() != config { t.Fatal("service reread mutable process configuration") }
}
// TestAtlasResourceApp executes each startup policy through the submitted active App graph.
func TestAtlasResourceApp(t *testing.T) {
 root := atlasAppRoot(t)
 originalApp := filepath.Join(t.TempDir(),"original-app")
 if output,err := atlasRunResourceTool(t,root,nil,"forj","build"); err!=nil { t.Fatalf("build original App: %v\n%s",err,output) }
 if output,err := atlasRunResourceTool(t,root,nil,"go","build","-o",originalApp,"./cmd/app"); err!=nil { t.Fatalf("compile original App: %v\n%s",err,output) }
 atlasBuildResourceApp(t,root,nil)
 for name,test := range atlasDispatchCases() { t.Run(name,func(t *testing.T) {
  // The process environment wins over the fixture file while required missing keys stay explicit.
  env := []string{"DISPATCH_TOKEN=", "DISPATCH_WORKERS=", "DISPATCH_TIMEOUT=", "DISPATCH_ENABLED=", "DISPATCH_REGION="}
  env = append(env,test.process...)
  path := filepath.Join(root,".env")
  original,err := os.ReadFile(path); if err != nil { t.Fatal(err) }
  defer os.WriteFile(path,original,0600)
  if err := os.WriteFile(path,[]byte("APP_ENV=local\n"+test.file),0600); err != nil { t.Fatal(err) }
  if test.file != "" && len(test.process)==0 { env=nil }
  originalOutput,originalErr := atlasRunResourceTool(t,root,env,originalApp,"hello:world")
  if test.wantError != "" && originalErr==nil { t.Fatalf("original App startup accepted invalid configuration: %s",originalOutput) }
  if test.wantError == "" && originalErr!=nil { t.Fatalf("original App rejected valid configuration: %v\n%s",originalErr,originalOutput) }
  output,runErr := atlasRunResourceApp(t,root,env)
  if test.wantError != "" {
   if runErr==nil { t.Fatalf("App accepted invalid startup: %s",output) }
   if test.wantError!="*" && !strings.Contains(output,test.wantError) { t.Fatalf("App startup missing key %s: %s",test.wantError,output) }
   return
  }
  if runErr!=nil { t.Fatalf("App startup: %v\n%s",runErr,output) }
  marker := "atlas-dispatch-app:"
  index := strings.Index(output,marker); if index<0 { t.Fatalf("App did not consume dispatch service: %s",output) }
  var got dispatch.Config
  if err := json.Unmarshal([]byte(strings.Split(output[index+len(marker):],"\n")[0]),&got); err!=nil { t.Fatal(err) }
  if got!=test.want { t.Fatalf("App snapshot differs from startup policy") }
 }) }
}
`

// dispatchAppRuntimeCommand consumes the configured service through the generated command registry.
const dispatchAppRuntimeCommand = `package cmd
import (
 "encoding/json"
 "fmt"
 "example.com/invoiceeval/internal/dispatch"
)
// HelloWorldCmd requires the submitted App-owned dispatch service.
type HelloWorldCmd struct { service *dispatch.Service }
// NewHelloWorldCmd roots the submitted provider in the existing command graph.
func NewHelloWorldCmd(service *dispatch.Service) *HelloWorldCmd { return &HelloWorldCmd{service:service} }
// Signature preserves the generated command identity.
func (*HelloWorldCmd) Signature() string { return ` + "`" + `name:"hello:world" help:"Dispatch startup verification"` + "`" + ` }
// Run reports only supervisor-owned fake configuration for exact startup comparison.
func (command *HelloWorldCmd) Run() error {
 body,err := json.Marshal(command.service.Snapshot()); if err!=nil { return err }
 fmt.Printf("atlas-dispatch-app:%s\n",body)
 return nil
}
`
