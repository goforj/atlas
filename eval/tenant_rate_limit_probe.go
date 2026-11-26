package eval

import "context"

// runTenantRateLimitLibraryProbe records executed cache adoption without changing application outcome.
func runTenantRateLimitLibraryProbe(ctx context.Context, runner CommandRunner, project VerifierProject) EndpointResult {
	result := runIsolatedCommand(ctx, runner, project, commandContract{
		id:              "tenant-rate-limit-library-use",
		arguments:       []string{"go", "test", "./internal/requests", "-run", "^TestAtlasTenantRateLimitLibraryPath$", "-count=1"},
		supervisorFiles: tenantRateLimitSupervisorFiles(),
	})
	result.Kind = RequirementQuality
	return result
}

// tenantRateLimitSupervisorFiles installs policy probes only after the candidate Project has been sealed.
func tenantRateLimitSupervisorFiles() []supervisorFile {
	return []supervisorFile{{path: "internal/requests/atlas_eval_rate_limit_test.go", body: tenantRateLimitBehaviorProbe}}
}

const tenantRateLimitBehaviorProbe = `package requests

import (
 "context"
 "crypto/rand"
 "errors"
 "encoding/json"
 "io"
 "net/http"
 "net/http/httptest"
 "os"
 "strconv"
 "sync"
 "testing"
 "time"

 "github.com/alicebob/miniredis/v2"
 "github.com/goforj/cache"
 "github.com/goforj/cache/cachecore"
 "github.com/goforj/web"
 "github.com/goforj/web/webtest"
 "example.com/invoiceeval/internal/caches"
)

// atlasRequestFailure preserves the real store except for a controlled counter outage.
type atlasRequestFailure struct { cachecore.Store }

// Increment makes failure policy deterministic without depending on network retry timing.
func (store atlasRequestFailure) Increment(context.Context,string,int64,time.Duration)(int64,error) { return 0,errors.New("counter unavailable") }

// atlasRequestCaches opens independent framework resources in a private namespace on one Redis endpoint.
func atlasRequestCaches(t *testing.T) (*cache.Cache,*cache.Cache,*miniredis.Miniredis) {
 t.Helper()
 address:=os.Getenv("ATLAS_RATE_LIMIT_REDIS_ADDR")
 var server *miniredis.Miniredis
 if address=="" {server=miniredis.RunT(t);address=server.Addr()}
 t.Setenv("CACHE_DRIVER","memory")
 t.Setenv("CACHE_REQUESTS_DRIVER","redis")
 t.Setenv("CACHE_REQUESTS_ADDR",address)
 t.Setenv("CACHE_REQUESTS_PREFIX","atlas-"+rand.Text())
 first,err:=caches.NewManager();if err!=nil{t.Fatal(err)}
 second,err:=caches.NewManager();if err!=nil{t.Fatal(err)}
 return first.Requests(),second.Requests(),server
}

// atlasRequestRoute executes the actual declared route middleware in its framework order.
func atlasRequestRoute(t *testing.T,controller *Controller,path,token,assertedTenant string) *httptest.ResponseRecorder {
 t.Helper()
 for _,route:=range controller.Routes() {
  if route.Path()!=path {continue}
  request:=httptest.NewRequest(http.MethodGet,path,nil)
  request.Header.Set("Authorization",token)
  request.Header.Set("X-Tenant-ID",assertedTenant)
  response:=httptest.NewRecorder()
  target:=webtest.NewContext(request,response,path,nil)
  original:=route.Handler()
  calls:=0
  handler:=func(request web.Context)error{calls++;return original(request)}
  middleware:=route.Middlewares()
  for index:=len(middleware)-1;index>=0;index-- {handler=middleware[index](handler)}
  if err:=handler(target);err!=nil{t.Fatal(err)}
  if (response.Code==http.StatusTooManyRequests || response.Code==http.StatusServiceUnavailable || response.Code==http.StatusUnauthorized) && calls!=0 {
   t.Fatalf("protected operation executed %d times after status %d",calls,response.Code)
  }
  return response
 }
 t.Fatalf("missing actual route %s",path)
 return nil
}

// atlasRequestError rejects partial error output followed by a protected handler response.
func atlasRequestError(t *testing.T,response *httptest.ResponseRecorder,status int,message string) {
 t.Helper()
 decoder:=json.NewDecoder(response.Body)
 var payload map[string]string
 if err:=decoder.Decode(&payload);err!=nil{t.Fatal(err)}
 if response.Code!=status || len(payload)!=1 || payload["error"]!=message{t.Fatalf("error response: %d %#v",response.Code,payload)}
 var trailing any
 if err:=decoder.Decode(&trailing);err!=io.EOF{t.Fatalf("trailing error response data: %v",err)}
}

// TestAtlasTenantRateLimitBehavior verifies shared tenant isolation, reset, trusted identity, and route-local failure behavior.
func TestAtlasTenantRateLimitBehavior(t *testing.T) {
 first,second,_:=atlasRequestCaches(t)
 now:=time.Date(2030,1,2,3,4,10,0,time.UTC)
 clock:=func()time.Time{return now}
 controllers:=[]*Controller{NewController(NewLimiter(first,clock)),NewController(NewLimiter(second,func()time.Time{return clock().In(time.FixedZone("replica",7*60*60))}))}
 for _,controller:=range controllers {
  response:=atlasRequestRoute(t,controller,"/exports","Bearer tenant-a-token","spoofed")
  if response.Code!=http.StatusOK{t.Fatalf("allowed request: %d %s",response.Code,response.Body.String())}
 }
 denied:=atlasRequestRoute(t,controllers[0],"/exports","Bearer tenant-a-token","tenant-b")
 retry,err:=strconv.Atoi(denied.Header().Get("Retry-After"))
 if denied.Code!=http.StatusTooManyRequests || err!=nil || retry!=50 {t.Fatalf("exhausted budget: %d retry=%q %s",denied.Code,denied.Header().Get("Retry-After"),denied.Body.String())}
 atlasRequestError(t,denied,http.StatusTooManyRequests,"rate limit exceeded")
 if response:=atlasRequestRoute(t,controllers[1],"/exports","Bearer tenant-b-token","tenant-a");response.Code!=http.StatusOK{t.Fatalf("tenant isolation: %d",response.Code)}
 if response:=atlasRequestRoute(t,controllers[0],"/exports","","tenant-a");response.Code!=http.StatusUnauthorized{t.Fatalf("unverified tenant accepted: %d",response.Code)}
 if response:=atlasRequestRoute(t,controllers[0],"/exports","Bearer invalid","tenant-b");response.Code!=http.StatusUnauthorized{t.Fatalf("invalid credential accepted: %d",response.Code)}
 if response:=atlasRequestRoute(t,controllers[0],"/health","","");response.Code!=http.StatusOK{t.Fatalf("unrelated route limited: %d",response.Code)}
 now=now.Add(50*time.Second)
 for _,controller:=range controllers {
  if response:=atlasRequestRoute(t,controller,"/exports","Bearer tenant-a-token","");response.Code!=http.StatusOK{t.Fatalf("controlled reset: %d",response.Code)}
 }
 if response:=atlasRequestRoute(t,controllers[1],"/exports","Bearer tenant-a-token","");response.Code!=http.StatusTooManyRequests{t.Fatalf("new shared window not enforced: %d",response.Code)}
 failing:=NewController(NewLimiter(cache.NewCache(atlasRequestFailure{Store:first.Store()}),clock))
 atlasRequestError(t,atlasRequestRoute(t,failing,"/exports","Bearer tenant-b-token",""),http.StatusServiceUnavailable,"rate limit unavailable")
 if response:=atlasRequestRoute(t,failing,"/health","","");response.Code!=http.StatusOK{t.Fatalf("outage blocked health: %d",response.Code)}
}

// TestAtlasTenantRateLimitConcurrency checks the atomic budget across two independent clients without sleeps.
func TestAtlasTenantRateLimitConcurrency(t *testing.T) {
 first,second,_:=atlasRequestCaches(t)
 clock:=func()time.Time{return time.Date(2030,1,2,5,6,59,500000000,time.UTC)}
 controllers:=[]*Controller{NewController(NewLimiter(first,clock)),NewController(NewLimiter(second,func()time.Time{return clock().In(time.FixedZone("replica",7*60*60))}))}
 statuses:=make(chan int,16)
 var group sync.WaitGroup
 for index:=0;index<16;index++ {
  group.Add(1)
  go func(controller *Controller){defer group.Done();statuses<-atlasRequestRoute(t,controller,"/exports","Bearer tenant-a-token","").Code}(controllers[index%2])
 }
 group.Wait();close(statuses)
 allowed:=0
 for status:=range statuses {if status==http.StatusOK{allowed++} else if status!=http.StatusTooManyRequests{t.Fatalf("unexpected concurrent status %d",status)}}
 if allowed!=2{t.Fatalf("shared atomic budget allowed %d of16",allowed)}
 denied:=atlasRequestRoute(t,controllers[0],"/exports","Bearer tenant-a-token","")
 if denied.Header().Get("Retry-After")!="1"{t.Fatalf("fractional reset must round up: %q",denied.Header().Get("Retry-After"))}
}

// TestAtlasTenantRateLimitTTL preserves a consumed budget inside its minute and eventually reclaims counter keys.
func TestAtlasTenantRateLimitTTL(t *testing.T) {
 t.Setenv("ATLAS_RATE_LIMIT_REDIS_ADDR","")
 first,second,server:=atlasRequestCaches(t)
 now:=time.Date(2030,1,2,3,4,10,0,time.UTC)
 clock:=func()time.Time{return now}
 controllers:=[]*Controller{NewController(NewLimiter(first,clock)),NewController(NewLimiter(second,func()time.Time{return clock().In(time.FixedZone("replica",7*60*60))}))}
 for _,controller:=range controllers{if response:=atlasRequestRoute(t,controller,"/exports","Bearer tenant-a-token","");response.Code!=http.StatusOK{t.Fatal(response.Code)}}
 now=now.Add(20*time.Second)
 server.FastForward(20*time.Second)
 denied:=atlasRequestRoute(t,controllers[1],"/exports","Bearer tenant-a-token","")
 atlasRequestError(t,denied,http.StatusTooManyRequests,"rate limit exceeded")
 keys:=server.Keys()
 if len(keys)==0{t.Fatal("shared backend did not retain a budget")}
 for _,key:=range keys{if ttl:=server.TTL(key);ttl<30*time.Second{t.Fatalf("counter TTL %v does not cover the remaining minute",ttl)}}
 server.FastForward(24*time.Hour)
 if len(server.Keys())!=0{t.Fatal("expired request buckets were never reclaimed")}
}

// TestAtlasTenantRateLimitLibraryPath observes root API execution independently from the required HTTP contract.
func TestAtlasTenantRateLimitLibraryPath(t *testing.T) {
 first,_,_:=atlasRequestCaches(t)
 increments:=0
 observed:=first.WithObserver(cache.ObserverFunc(func(_ context.Context,event cache.CacheOpEvent){if event.Operation=="increment"{increments++}}))
 controller:=NewController(NewLimiter(observed,func()time.Time{return time.Date(2030,1,2,7,8,0,0,time.UTC)}))
 if response:=atlasRequestRoute(t,controller,"/exports","Bearer tenant-a-token","");response.Code!=http.StatusOK{t.Fatal(response.Code)}
 if increments!=1{t.Fatalf("executed cache increments: %d",increments)}
}

`
