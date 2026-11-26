package eval

// partnerPayloadEncryptionContract measures authenticated storage behavior independently from source API attribution.
func partnerPayloadEncryptionContract() surfaceContract {
	return surfaceContract{
		id:                  "encrypt-partner-payloads/v1",
		allowedChanges:      []string{"internal/partnerpayloads/*.go", "app/lifecycle.go", "app/wire/inject_services_app.go"},
		qualityTestPatterns: []string{"internal/partnerpayloads/*_test.go"},
		sources: []sourceContract{
			{id: "payload-cipher-composition", paths: []string{"app/wire/inject_services_app.go"}, identifiers: []string{"NewCipher", "NewRepository", "NewService"}},
			{id: "payload-crypt-api", kind: RequirementQuality, paths: []string{"internal/partnerpayloads/*.go"}, stringLiterals: []string{"github.com/goforj/crypt"}, declarations: []declarationContract{
				{name: "NewCipher", selectorCallChoices: [][]string{{"NewFromEnv", "New"}}},
				{name: "Save", receiver: "Service", selectorCalls: []string{"Encrypt", "Put"}},
				{name: "Load", receiver: "Service", selectorCalls: []string{"Get", "Decrypt"}},
			}},
		},
		commands: standardSurfaceCommands(commandContract{
			id:              "payload-encryption-behavior",
			arguments:       []string{"go", "test", "./internal/partnerpayloads", "-run", "^TestAtlasPayloadEncryptionBehavior$", "-count=1"},
			supervisorFiles: []supervisorFile{{path: "internal/partnerpayloads/atlas_eval_encryption_test.go", body: payloadEncryptionProbe}},
		}, appResourceRuntimeContract("payload-app-runtime", "partnerpayloads", payloadAppRuntimeProbe, payloadAppRuntimeCommand)),
	}
}

const payloadEncryptionProbe = `package partnerpayloads_test

import (
 "bytes"
 "context"
 "encoding/base64"
 "encoding/json"
 "errors"
 "os"
 "strings"
 "testing"

 "github.com/goforj/crypt"
 "example.com/invoiceeval/internal/partnerpayloads"
)

// atlasPayloadContextKey tracks caller attribution without constraining derived context implementations.
type atlasPayloadContextKey struct{}

// atlasPayloadRepository models storage failures and records the opaque application boundary.
type atlasPayloadRepository struct {
 value string
 putError,getError error
 putCalls,getCalls int
 context context.Context
 id string
}

// Put records the value received by storage, including when storage reports failure.
func (repository *atlasPayloadRepository) Put(ctx context.Context,id,value string) error {
 repository.putCalls++; repository.context = ctx; repository.id = id; repository.value = value
 return repository.putError
}

// Get records caller attribution and can return untrusted bytes together with an operational error.
func (repository *atlasPayloadRepository) Get(ctx context.Context,id string) (string,error) {
 repository.getCalls++; repository.context = ctx; repository.id = id
 return repository.value,repository.getError
}

// atlasPayloadCipher models operational errors independently from the concrete encryption implementation.
type atlasPayloadCipher struct {
 encryptError,decryptError error
 encryptCalls,decryptCalls int
}

// Encrypt records cipher invocation before supplying an opaque fixture value.
func (cipher *atlasPayloadCipher) Encrypt(string) (string,error) { cipher.encryptCalls++; return "opaque",cipher.encryptError }

// Decrypt supplies partial plaintext deliberately so callers must discard it on error.
func (cipher *atlasPayloadCipher) Decrypt(string) (string,error) { cipher.decryptCalls++; return "partial plaintext",cipher.decryptError }

// atlasPayloadKey builds explicitly fake APP_KEY syntax without touching account credentials.
func atlasPayloadKey(value byte,size int) string { return "base64:"+base64.StdEncoding.EncodeToString(bytes.Repeat([]byte{value},size)) }

// atlasPayloadReference creates an independent envelope reader or historical writer.
func atlasPayloadReference(t *testing.T,value byte,size int) *crypt.Cipher {
 t.Helper(); cipher,err := crypt.New(bytes.Repeat([]byte{value},size)); if err != nil { t.Fatal(err) }; return cipher
}

// atlasTamperPayload changes one authenticated component while preserving its envelope syntax.
func atlasTamperPayload(t *testing.T,encoded,component string) string {
 t.Helper()
 raw,err := base64.StdEncoding.DecodeString(encoded); if err != nil { t.Fatal("stored value is not the established envelope") }
 var envelope map[string]string
 if err := json.Unmarshal(raw,&envelope); err != nil { t.Fatal("stored envelope is incompatible") }
 value := envelope[component]
 if value == "" { t.Fatal("authenticated envelope component is absent") }
 if component == "mac" {
  replacement := byte('0'); if value[0] == replacement { replacement = '1' }; envelope[component] = string(replacement)+value[1:]
 } else {
  decoded,err := base64.StdEncoding.DecodeString(value); if err != nil || len(decoded) == 0 { t.Fatal("invalid authenticated envelope component") }
  decoded[0] ^= 1; envelope[component] = base64.StdEncoding.EncodeToString(decoded)
 }
 changed,err := json.Marshal(envelope); if err != nil { t.Fatal(err) }; return base64.StdEncoding.EncodeToString(changed)
}

// TestAtlasPayloadEncryptionBehavior checks persisted-format compatibility, rotation, authentication, and operational boundaries.
func TestAtlasPayloadEncryptionBehavior(t *testing.T) {
 t.Setenv("APP_KEY",atlasPayloadKey(0x22,32))
 t.Setenv("APP_PREVIOUS_KEYS",atlasPayloadKey(0x11,32)+", "+atlasPayloadKey(0x33,32))
 cipher,err := partnerpayloads.NewCipher(); if err != nil { t.Fatal(err) }
 repository := partnerpayloads.NewRepository()
 service := partnerpayloads.NewService(repository,cipher)
 current := atlasPayloadReference(t,0x22,32)
 old := atlasPayloadReference(t,0x11,32)
 otherOld := atlasPayloadReference(t,0x33,32)
 ctx := context.Background()
 t.Run("round-trips-and-current-writes",func(t *testing.T) {
  for _,plaintext := range []string{"", "private partner payload", "Unicode: café 日本", "binary\x00payload",strings.Repeat("x",16),strings.Repeat("y",17)} {
   if err := service.Save(ctx,"current",plaintext); err != nil { t.Fatal(err) }
   raw,err := repository.Get(ctx,"current"); if err != nil { t.Fatal(err) }
   if raw == plaintext || (plaintext != "" && strings.Contains(raw,plaintext)) { t.Fatal("plaintext reached storage") }
   decoded,err := current.Decrypt(raw); if err != nil || decoded != plaintext { t.Fatal("new write is not authenticated by the current key alone in the established format") }
   if _,err := old.Decrypt(raw); err == nil { t.Fatal("previous key authenticated a new write") }
   restored,err := service.Load(ctx,"current"); if err != nil || restored != plaintext { t.Fatal("application round trip changed payload bytes") }
  }
 })
 t.Run("randomization",func(t *testing.T) {
  if err := service.Save(ctx,"first","equal payload"); err != nil { t.Fatal(err) }
  if err := service.Save(ctx,"second","equal payload"); err != nil { t.Fatal(err) }
  first,err := repository.Get(ctx,"first"); if err != nil { t.Fatal(err) }; second,err := repository.Get(ctx,"second"); if err != nil { t.Fatal(err) }
  if first == second { t.Fatal("equal payloads reused a deterministic ciphertext") }
 })
 t.Run("old-key-reads",func(t *testing.T) {
  for _,historical := range []*crypt.Cipher{old,otherOld} {
   encoded,err := historical.Encrypt("old-key record"); if err != nil { t.Fatal(err) }
   if err := repository.Put(ctx,"historical",encoded); err != nil { t.Fatal(err) }
   restored,err := service.Load(ctx,"historical"); if err != nil || restored != "old-key record" { t.Fatal("rotation lost a configured previous key") }
  }
  body,err := os.ReadFile("testdata/legacy.json"); if err != nil { t.Fatal(err) }
  var fixture struct { Key,Plaintext,Ciphertext string }; if err := json.Unmarshal(body,&fixture); err != nil { t.Fatal(err) }
  if err := repository.Put(ctx,"compatibility",fixture.Ciphertext); err != nil { t.Fatal(err) }
  restored,err := service.Load(ctx,"compatibility"); if err != nil || restored != fixture.Plaintext { t.Fatal("prepared compatibility record became unreadable") }
 })
 t.Run("tamper-and-malformed",func(t *testing.T) {
  valid,err := current.Encrypt("must not escape authentication"); if err != nil { t.Fatal(err) }
  values := []string{"", "not-base64",base64.StdEncoding.EncodeToString([]byte("{}")),atlasTamperPayload(t,valid,"iv"),atlasTamperPayload(t,valid,"value"),atlasTamperPayload(t,valid,"mac")}
  for _,encoded := range values {
   if err := repository.Put(ctx,"corrupt",encoded); err != nil { t.Fatal(err) }
   plaintext,err := service.Load(ctx,"corrupt"); if err == nil || plaintext != "" { t.Fatal("unauthenticated bytes produced plaintext") }
  }
 })
 t.Run("wrong-key",func(t *testing.T) {
  foreign,err := atlasPayloadReference(t,0x44,32).Encrypt("foreign record"); if err != nil { t.Fatal(err) }
  if err := repository.Put(ctx,"foreign",foreign); err != nil { t.Fatal(err) }
  plaintext,err := service.Load(ctx,"foreign"); if err == nil || plaintext != "" { t.Fatal("wrong key produced plaintext") }
 })
 t.Run("frozen-key-snapshot",func(t *testing.T) {
  t.Setenv("APP_KEY",atlasPayloadKey(0x44,32)); t.Setenv("APP_PREVIOUS_KEYS","")
  if err := service.Save(ctx,"snapshot","frozen keys"); err != nil { t.Fatal(err) }
  raw,err := repository.Get(ctx,"snapshot"); if err != nil { t.Fatal(err) }
  if value,err := current.Decrypt(raw); err != nil || value != "frozen keys" { t.Fatal("service reread mutable key configuration") }
  if _,err := atlasPayloadReference(t,0x44,32).Decrypt(raw); err == nil { t.Fatal("changed process key encrypted through a preexisting service") }
 })
 t.Run("aes128",func(t *testing.T) {
  t.Setenv("APP_KEY",atlasPayloadKey(0x55,16)); t.Setenv("APP_PREVIOUS_KEYS","")
  cipher,err := partnerpayloads.NewCipher(); if err != nil { t.Fatal("valid 16-byte key rejected") }
  service := partnerpayloads.NewService(partnerpayloads.NewRepository(),cipher)
  if err := service.Save(ctx,"small-key","16-byte key"); err != nil { t.Fatal(err) }
  if value,err := service.Load(ctx,"small-key"); err != nil || value != "16-byte key" { t.Fatal("16-byte key round trip failed") }
 })
 t.Run("invalid-key-composition",func(t *testing.T) {
  tests := []struct { name,current,previous string }{
   {"missing","",""},{"unprefixed","unprefixed-fake-key",""},{"malformed-current","base64:???",""},{"current-24",atlasPayloadKey(0x66,24),""},
   {"malformed-previous",atlasPayloadKey(0x22,32),"base64:???"},{"previous-24",atlasPayloadKey(0x22,32),atlasPayloadKey(0x66,24)},
  }
  for _,test := range tests { t.Run(test.name,func(t *testing.T) {
   t.Setenv("APP_KEY",test.current); t.Setenv("APP_PREVIOUS_KEYS",test.previous)
   cipher,err := partnerpayloads.NewCipher(); if err == nil || cipher != nil { t.Fatal("invalid keys returned a usable cipher") }
   for _,failure := range []error{err} { for _,key := range []string{test.current,test.previous} { if key != "" && strings.Contains(failure.Error(),key) { t.Fatal("key diagnostic exposed submitted key material") } } }
  }) }
 })
 t.Run("operational-errors-and-context",func(t *testing.T) {
  failure := errors.New("fixture operational error")
  repository := &atlasPayloadRepository{}
  cipher := &atlasPayloadCipher{encryptError:failure}
  service := partnerpayloads.NewService(repository,cipher)
  if err := service.Save(ctx,"encrypt-error","private"); !errors.Is(err,failure) || repository.putCalls != 0 { t.Fatal("encryption failure wrote data or lost its identity") }
  repository = &atlasPayloadRepository{putError:failure}; cipher = &atlasPayloadCipher{}; service = partnerpayloads.NewService(repository,cipher)
  if err := service.Save(ctx,"put-error","private"); !errors.Is(err,failure) || repository.value != "opaque" { t.Fatal("storage failure lost identity or received plaintext") }
  repository = &atlasPayloadRepository{value:"untrusted",getError:failure}; cipher = &atlasPayloadCipher{}; service = partnerpayloads.NewService(repository,cipher)
  if value,err := service.Load(ctx,"get-error"); !errors.Is(err,failure) || value != "" || cipher.decryptCalls != 0 { t.Fatal("storage read failure returned data or invoked decryption") }
  repository = &atlasPayloadRepository{value:"opaque"}; cipher = &atlasPayloadCipher{decryptError:failure}; service = partnerpayloads.NewService(repository,cipher)
  if value,err := service.Load(ctx,"decrypt-error"); !errors.Is(err,failure) || value != "" { t.Fatal("decryption error returned partial plaintext or lost identity") }
  caller := context.WithValue(ctx,atlasPayloadContextKey{},"caller")
  repository = &atlasPayloadRepository{}; cipher = &atlasPayloadCipher{}; service = partnerpayloads.NewService(repository,cipher)
  if err := service.Save(caller,"attributed","private"); err != nil || repository.context.Value(atlasPayloadContextKey{}) != "caller" || repository.id != "attributed" { t.Fatal("save detached the repository context or ID") }
  if _,err := service.Load(caller,"attributed"); err != nil || repository.context.Value(atlasPayloadContextKey{}) != "caller" || repository.id != "attributed" { t.Fatal("load detached the repository context or ID") }
  canceled,cancel := context.WithCancel(ctx); cancel()
  repository = &atlasPayloadRepository{}; cipher = &atlasPayloadCipher{}; service = partnerpayloads.NewService(repository,cipher)
  if err := service.Save(canceled,"cancel","private"); !errors.Is(err,context.Canceled) || cipher.encryptCalls != 0 || repository.putCalls != 0 { t.Fatal("canceled save did work") }
  if value,err := service.Load(canceled,"cancel"); !errors.Is(err,context.Canceled) || value != "" || repository.getCalls != 0 || cipher.decryptCalls != 0 { t.Fatal("canceled load did work or returned plaintext") }
  memory := partnerpayloads.NewRepository()
  if err := memory.Put(canceled,"cancel","opaque"); !errors.Is(err,context.Canceled) { t.Fatal("repository ignored write cancellation") }
  if value,err := memory.Get(canceled,"cancel"); !errors.Is(err,context.Canceled) || value != "" { t.Fatal("repository ignored read cancellation") }
  service = partnerpayloads.NewService(memory,cipher)
  if value,err := service.Load(ctx,"missing"); !errors.Is(err,partnerpayloads.ErrNotFound) || value != "" { t.Fatal("not-found error lost identity") }
 })
}
`
