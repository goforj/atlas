package workflows

// tenantRateLimitTask recognizes request budget policy without conflating it with cache ownership locks.
func tenantRateLimitTask(task string) bool {
	return containsAny(task, "rate limit", "rate-limit", "request budget", "tenant request quota")
}

// init composes request budget work with independent configuration and HTTP surface requests.
func init() {
	registerApplicationWorkflow(applicationWorkflowDefinition{
		id: "goforj-tenant-request-rate-limits", matches: tenantRateLimitTask, docsTopic: tenantRateLimitTask,
		commands: []string{"{build}", "go test ./..."},
		docs:     []DocReference{{Path: "applications/middleware.md", Heading: "Route Middleware"}, {Path: "libraries/cache.md", Heading: "Increment {#cache-increment}"}},
		files:    []string{"internal/<domain>/limiter.go", "internal/<domain>/limiter_test.go", "{composition}/routes.go", "{wire}/inject_http_controllers_app.go", "internal/caches/accessors_gen.go", ".env"},
		steps: []PlanStep{
			{Action: "inspect the authenticated principal and selected route's middleware order", Purpose: "derive tenant identity only after credential validation"},
			{Action: "declare local or shared deployment scope, the budget/window, and backend failure response", Purpose: "prevent an in-process counter from masquerading as a cross-replica policy"},
			{Action: "borrow the named cache through App composition and apply policy only to the selected route", Purpose: "keep shared atomic counters and unrelated routes independent"},
			{Action: "test two instances, tenant isolation, exhaustion, controlled-clock reset, backend failure, and the unaffected route", Purpose: "prove deployment policy without sleeps"},
			{Action: "{build}", Purpose: "refresh the selected App dependency graph before testing"},
		},
		warnings: []string{"Read goforj-tenant-request-rate-limits before choosing local middleware or a shared cache-backed budget.", "Use a verified tenant principal, never an untrusted identity header. Declare fail-open or fail-closed behavior explicitly.", "RateLimit uses wall time in the pinned cache root; an injected-clock fixed-window policy can use atomic Increment with explicit tenant and time bucket keys."},
	})
}
