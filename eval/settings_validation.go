package eval

import "context"

// settingsValidationContract keeps behavioral validation and actual library calls separately visible.
func settingsValidationContract() surfaceContract {
	return surfaceContract{
		id:                  "patch-customer-settings/v1",
		allowedChanges:      []string{"internal/settings/*.go", "app/routes.go", "app/wire/inject_http_controllers_app.go", "app/wire/*_test.go"},
		qualityTestPatterns: []string{"internal/settings/*_test.go", "app/wire/*_test.go"},
		sources: []sourceContract{
			{id: "settings-library-path", kind: RequirementQuality, paths: []string{"internal/settings/*.go"}, stringLiterals: []string{"github.com/goforj/validation"}, declarations: []declarationContract{
				{name: "Patch", receiver: "Controller", selectorCalls: []string{"Update"}, forbiddenCalls: []string{"Background"}},
				{name: "Update", receiver: "Service", selectorCalls: []string{"ValidatePatch"}, forbiddenCalls: []string{"Background"}},
				{name: "ValidatePatch", receiver: "Service", selectorCalls: []string{"ValidateContext", "Presence", "Check"}, forbiddenCalls: []string{"Background"}},
			}},
		},
		commands: append(standardSurfaceCommands(commandContract{
			id:              "settings-validation-behavior",
			arguments:       []string{"go", "test", "./internal/settings", "-run", "^(TestAtlasSettingsValidationBehavior|TestAtlasRequiredNamePolicy)$", "-count=1"},
			supervisorFiles: []supervisorFile{{path: "internal/settings/atlas_eval_settings_test.go", body: settingsValidationBehaviorProbe + settingsRequiredNamePolicyProbe}},
		}), appResourceRuntimeContract("settings-app-runtime", "settings", settingsAppRuntimeProbe, settingsAppRuntimeCommand), commandContract{id: "settings-library-runtime", probe: runSettingsLibraryProbe}, commandContract{id: "settings-patch-route", arguments: []string{"forj", "route:list"}, contains: []string{"PATCH", "/api/v1/customers/settings"}}),
	}
}

// runSettingsLibraryProbe reports executed library adoption separately from the application outcome.
func runSettingsLibraryProbe(ctx context.Context, runner CommandRunner, project VerifierProject) EndpointResult {
	result := runIsolatedCommand(ctx, runner, project, commandContract{
		id:              "settings-library-runtime",
		arguments:       []string{"go", "test", "./internal/settings", "-run", "^TestAtlasSettingsLibraryPath$", "-count=1"},
		supervisorFiles: []supervisorFile{{path: "internal/settings/atlas_eval_library_test.go", body: settingsLibraryProbe}},
	})
	result.Kind = RequirementQuality
	return result
}

const settingsLibraryProbe = `package settings
import (
 "context"
 "encoding/json"
 "errors"
 "testing"
 "github.com/goforj/validation"
)
// TestAtlasSettingsLibraryPath proves real library issues reach the executed application update path.
func TestAtlasSettingsLibraryPath(t *testing.T) {
 service := NewService(NewNameCheck())
 var input PatchInput
 if err := json.Unmarshal([]byte("{\"limit\":-1}"), &input); err != nil { t.Fatal(err) }
 _,err := service.Update(context.Background(),input)
 var failures *validation.Errors
 if !errors.As(err,&failures) || len(failures.Issues()) == 0 || failures.Issues()[0].Path != "limit" || failures.Issues()[0].Code != "min" { t.Fatalf("library update errors = %v",err) }
}
`

const settingsValidationBehaviorProbe = `package settings

import (
 "context"
 "encoding/json"
 "errors"
 "net/http"
 "net/http/httptest"
 "reflect"
 "strings"
 "testing"
 "github.com/goforj/validation"
 "github.com/goforj/web/webtest"
)

// settingsProbeKey keeps caller context attribution independent from production keys.
type settingsProbeKey struct{}

// TestAtlasSettingsValidationBehavior exercises the application path independently from candidate tests.
func TestAtlasSettingsValidationBehavior(t *testing.T) {
 tests := []struct { name, body string; status int; fields map[string]string; want Settings }{
  {name:"omitted",body:"{}",status:200,want:Settings{Name:"Existing",Limit:25,Enabled:true}},
  {name:"zero-false-null",body:"{\"limit\":0,\"enabled\":false,\"note\":null}",status:200,want:Settings{Name:"Existing"}},
  {name:"normalized",body:"{\"name\":\" Alice \",\"note\":\"new\"}",status:200,want:Settings{Name:"Alice",Limit:25,Enabled:true}},
  {name:"nulls",body:"{\"name\":null,\"limit\":null,\"enabled\":null}",status:422,fields:map[string]string{"name":"null","limit":"null","enabled":"null"}},
  {name:"blank-negative",body:"{\"name\":\"  \",\"limit\":-1}",status:422,fields:map[string]string{"name":"required","limit":"min"}},
  {name:"maximum",body:"{\"limit\":101}",status:422,fields:map[string]string{"limit":"max"}},
  {name:"inclusive-limit",body:"{\"limit\":100}",status:200,want:Settings{Name:"Existing",Limit:100,Enabled:true}},
  {name:"atomic",body:"{\"name\":\" Alice \",\"limit\":101}",status:422,fields:map[string]string{"limit":"max"}},
  {name:"note-long",body:"{\"note\":\""+strings.Repeat("x",81)+"\"}",status:422,fields:map[string]string{"note":"max"}},
  {name:"wrong-type",body:"{\"limit\":false}",status:400},
  {name:"malformed",body:"{",status:400},
 }
 for _, test := range tests { t.Run(test.name,func(t *testing.T) {
  called := 0
  ctx := context.WithValue(context.Background(), settingsProbeKey{}, "caller")
  service := NewService(func(ruleContext context.Context, value string) error {
   called++
   if ruleContext.Value(settingsProbeKey{}) != "caller" || value != "Alice" { t.Errorf("policy context/value = %v/%q",ruleContext,value) }
   return nil
  })
  request := httptest.NewRequest(http.MethodPatch,"/customers/settings",strings.NewReader(test.body)).WithContext(ctx)
  request.Header.Set("Content-Type","application/json")
  response := httptest.NewRecorder()
  if err := NewController(service).Patch(webtest.NewContext(request,response,"/customers/settings",nil)); err != nil { t.Fatal(err) }
  if response.Code != test.status { t.Fatalf("status = %d, body = %s",response.Code,response.Body.String()) }
  if test.status == 200 {
   var got Settings
   if err := json.Unmarshal(response.Body.Bytes(),&got); err != nil { t.Fatal(err) }
   if got.Name != test.want.Name || got.Limit != test.want.Limit || got.Enabled != test.want.Enabled { t.Fatalf("settings = %#v",got) }
   switch test.name { case "omitted": if got.Note == nil || *got.Note != "keep me" { t.Fatal("omitted note changed") }; case "zero-false-null": if got.Note != nil { t.Fatal("null note not cleared") }; case "normalized": if got.Note == nil || *got.Note != "new" || called != 1 { t.Fatal("normalization/policy was skipped") } }
  } else {
   var got struct { Error string; Fields map[string]string }
   if err := json.Unmarshal(response.Body.Bytes(),&got); err != nil { t.Fatal(err) }
   expected := "validation_failed"; if test.status == 400 { expected = "invalid_payload" }
   if got.Error != expected || !reflect.DeepEqual(got.Fields,test.fields) { t.Fatalf("errors = %#v",got) }
   result,err := service.Update(context.Background(),PatchInput{})
   if err != nil || result.Name != "Existing" || result.Limit != 25 || !result.Enabled || result.Note == nil || *result.Note != "keep me" { t.Fatalf("failed patch mutated state: %#v, %v",result,err) }
  }
 }) }
 t.Run("reusable-policy",func(t *testing.T) {
  service := NewService(NewNameCheck())
  var input PatchInput
  if err := json.Unmarshal([]byte("{\"limit\":-1}"), &input); err != nil { t.Fatal(err) }
  err := service.ValidatePatch(context.Background(),input)
  if err == nil { t.Fatal("direct policy accepted a negative limit") }
 })
 for _, failure := range []error{errors.New("private backend failure"), context.Canceled, &validation.ConfigurationError{Problem:"private configuration"}, validation.Invalid("reserved","reserved name")} {
  t.Run(failure.Error(),func(t *testing.T) {
   service := NewService(func(context.Context,string) error { return failure })
   request := httptest.NewRequest(http.MethodPatch,"/customers/settings",strings.NewReader("{\"name\":\"Alice\"}"))
   request.Header.Set("Content-Type","application/json")
   response := httptest.NewRecorder()
   err := NewController(service).Patch(webtest.NewContext(request,response,"/customers/settings",nil))
   var violation *validation.Violation
   if errors.As(failure,&violation) {
    var got struct { Error string; Fields map[string]string }
    if decodeErr := json.Unmarshal(response.Body.Bytes(),&got); decodeErr != nil { t.Fatal(decodeErr) }
    if err != nil || response.Code != 422 || got.Error != "validation_failed" || got.Fields["name"] != "reserved" { t.Fatalf("client violation = %v, %s",err,response.Body.String()) }
   } else if !errors.Is(err,failure) || strings.Contains(response.Body.String(),failure.Error()) { t.Fatalf("operational error = %v, body = %s",err,response.Body.String()) }
  })
 }
 t.Run("canceled-before-policy",func(t *testing.T) {
  ctx,cancel := context.WithCancel(context.Background()); cancel()
  service := NewService(func(context.Context,string) error { t.Fatal("policy ran after cancellation"); return nil })
  var input PatchInput
  if err := json.Unmarshal([]byte("{\"name\":\"Alice\"}"), &input); err != nil { t.Fatal(err) }
  if _,err := service.Update(ctx,input); !errors.Is(err,context.Canceled) { t.Fatalf("cancel = %v",err) }
 })
}
`

// settingsRequiredNamePolicyProbe rejects silent policy disablement without prescribing a constructor error API.
const settingsRequiredNamePolicyProbe = `
// TestAtlasRequiredNamePolicy keeps invalid wiring operational instead of accepting a patch without required policy.
func TestAtlasRequiredNamePolicy(t *testing.T) {
 for _,entrypoint:=range []string{"validate","update"} {t.Run(entrypoint,func(t *testing.T) {
  rejected:=false
  func() {
   defer func(){if recover()!=nil {rejected=true}}()
   service:=NewService(nil)
   var input PatchInput
   if err:=json.Unmarshal([]byte("{\"name\":\"Alice\"}"),&input);err!=nil {t.Fatal(err)}
   var err error
   if entrypoint=="validate" {err=service.ValidatePatch(context.Background(),input)} else {_,err=service.Update(context.Background(),input)}
   var clientErrors *validation.Errors
   rejected=err!=nil && !errors.As(err,&clientErrors)
  }()
  if !rejected {t.Fatal("missing required name policy was silently skipped or became a client field issue")}
 })}
}
`
