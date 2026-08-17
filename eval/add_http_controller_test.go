package eval

import (
	"context"
	"errors"
	"go/ast"
	"go/importer"
	"go/parser"
	"go/token"
	"go/types"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// fakeCommandRunner supplies deterministic black-box evidence without executing candidate code in unit tests.
type fakeCommandRunner struct {
	failCommand  string
	failContains string
	commands     [][]string
	files        map[string][]byte
	opens        int
}

// Open records isolated phases while retaining supervisor files for assertions.
func (runner *fakeCommandRunner) Open(_ context.Context, project VerifierProject) (CommandSession, error) {
	if runner.files == nil {
		runner.files = make(map[string][]byte)
	}
	for _, test := range project.BaselineTests {
		runner.files[test.Path] = append([]byte(nil), test.Body...)
	}
	runner.opens++
	return runner, nil
}

// WriteFile records supervisor-owned oracle source separately from candidate fixtures.
func (runner *fakeCommandRunner) WriteFile(path string, body []byte) error {
	if _, exists := runner.files[path]; exists {
		return errors.New("verifier file already exists")
	}
	runner.files[path] = append([]byte(nil), body...)
	return nil
}

// Run records verifier commands and returns the promoted route listing.
func (runner *fakeCommandRunner) Run(_ context.Context, command []string) (string, error) {
	runner.commands = append(runner.commands, append([]string(nil), command...))
	if len(command) > 1 && command[1] == runner.failCommand {
		return "", errors.New("command failed")
	}
	if runner.failContains != "" && strings.Contains(strings.Join(command, "\x00"), runner.failContains) {
		return "", errors.New("command failed")
	}
	if len(command) > 1 && command[1] == "route:list" {
		return "GET /api/v1/invoices/:id", nil
	}
	if len(command) > 2 && command[1] == "test" && command[2] == "-json" {
		return "{\"Action\":\"run\",\"Test\":\"TestAtlasInvoiceHTTPBehavior\"}\n{\"Action\":\"pass\",\"Test\":\"TestAtlasInvoiceHTTPBehavior\"}\n", nil
	}
	output := "ok"
	for path, body := range runner.files {
		if strings.Contains(filepath.Base(path), "atlas_eval_completion_marker_") && strings.HasSuffix(path, "_test.go") {
			start := strings.Index(string(body), "ATLAS_SUPERVISOR_COMPLETION_")
			if start >= 0 {
				end := start
				for end < len(body) && (body[end] == '_' || body[end] >= '0' && body[end] <= '9' || body[end] >= 'A' && body[end] <= 'Z' || body[end] >= 'a' && body[end] <= 'z') {
					end++
				}
				output += "\n" + string(body[start:end])
			}
		}
	}
	return output, nil
}

// Close releases no resources because the fake session owns no filesystem state.
func (*fakeCommandRunner) Close(context.Context) error {
	return nil
}

// TestAddHTTPControllerVerifierAcceptsIndependentBoundaryShapes calibrates away golden-source coupling.
func TestAddHTTPControllerVerifierAcceptsIndependentBoundaryShapes(t *testing.T) {
	tests := []struct {
		name       string
		dependency string
	}{
		{name: "service boundary", dependency: "service *Service"},
		{name: "query boundary", dependency: "service InvoiceQuery"},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			root := writeControllerFixture(t, validControllerSource(test.dependency), true)
			runner := &fakeCommandRunner{}
			result, err := NewAddHTTPControllerVerifier(runner).Verify(context.Background(), VerificationInput{ProjectRoot: root})
			if err != nil {
				t.Fatalf("Verify(): %v", err)
			}
			if result.FrameworkOutcome.Status != EndpointPassed {
				t.Fatalf("framework outcome = %#v; checks = %#v", result.FrameworkOutcome, result.Checks)
			}
			if len(runner.commands) != 4 || runner.opens != 4 {
				t.Fatalf("verifier commands = %#v", runner.commands)
			}
			if len(runner.files) != 2 {
				t.Fatalf("verifier files = %#v", runner.files)
			}
		})
	}
}

// TestAddHTTPControllerVerifierBuildsProbeForBothPackageFamilies keeps the oracle independent from golden source placement.
func TestAddHTTPControllerVerifierBuildsProbeForBothPackageFamilies(t *testing.T) {
	tests := []struct {
		name       string
		transport  bool
		wantPath   string
		wantSource []string
	}{
		{
			name:       "domain package",
			wantPath:   "internal/invoices/atlas_eval_invoice_behavior_test.go",
			wantSource: []string{"package invoices", "controller := &Controller{service: NewService(NewRepository())}", "controller.Show(requestContext)"},
		},
		{
			name:       "transport package",
			transport:  true,
			wantPath:   "internal/http/atlas_eval_invoice_behavior_test.go",
			wantSource: []string{"package http", `stdhttp "net/http"`, `"example.com/invoiceeval/internal/invoices"`, "controller := &InvoiceController{service: invoices.NewService(invoices.NewRepository())}"},
		},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			root := writeControllerFixture(t, validControllerSource("service *Service"), true)
			if test.transport {
				if err := os.Remove(filepath.Join(root, "internal", "invoices", "controller.go")); err != nil {
					t.Fatal(err)
				}
				writeFixtureFile(t, root, "internal/http/invoice_controller.go", transportControllerSource())
				writeFixtureFile(t, root, "app/routes.go", "package app\nfunc ProvideRoutes(invoiceController *http.InvoiceController) { _ = invoiceController.Routes() }\n")
				writeFixtureFile(t, root, "app/wire/inject_http_controllers_app.go", "package wire\nvar appHTTPControllerSet = wire.NewSet(http.NewInvoiceController)\n")
			}
			runner := &fakeCommandRunner{}
			result, err := NewAddHTTPControllerVerifier(runner).Verify(context.Background(), VerificationInput{ProjectRoot: root})
			if err != nil {
				t.Fatalf("Verify(): %v", err)
			}
			if result.FrameworkOutcome.Status != EndpointPassed {
				t.Fatalf("framework outcome = %#v; checks = %#v", result.FrameworkOutcome, result.Checks)
			}
			body, exists := runner.files[filepath.FromSlash(test.wantPath)]
			if !exists {
				t.Fatalf("verifier files = %#v, want %q", runner.files, test.wantPath)
			}
			for _, token := range test.wantSource {
				if !strings.Contains(string(body), token) {
					t.Fatalf("probe does not contain %q:\n%s", token, body)
				}
			}
		})
	}
}

// TestAddHTTPControllerVerifierAcceptsIndependentPackagePlacement prevents the golden scenario path from becoming the contract.
func TestAddHTTPControllerVerifierAcceptsIndependentPackagePlacement(t *testing.T) {
	root := writeControllerFixture(t, validControllerSource("service *Service"), true)
	source := filepath.Join(root, "internal", "invoices", "controller.go")
	destination := filepath.Join(root, "internal", "invoicehttp", "controller.go")
	if err := os.MkdirAll(filepath.Dir(destination), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.Rename(source, destination); err != nil {
		t.Fatal(err)
	}
	result, err := NewAddHTTPControllerVerifier(&fakeCommandRunner{}).Verify(context.Background(), VerificationInput{ProjectRoot: root})
	if err != nil {
		t.Fatalf("Verify(): %v", err)
	}
	if result.FrameworkOutcome.Status != EndpointPassed {
		t.Fatalf("framework outcome = %#v; checks = %#v", result.FrameworkOutcome, result.Checks)
	}
}

// TestAddHTTPControllerVerifierAcceptsTransportNamedController calibrates the verifier against a distinct transport package and named controller.
func TestAddHTTPControllerVerifierAcceptsTransportNamedController(t *testing.T) {
	root := writeControllerFixture(t, transportControllerSource(), true)
	if err := os.Remove(filepath.Join(root, "internal", "invoices", "controller.go")); err != nil {
		t.Fatal(err)
	}
	writeFixtureFile(t, root, "internal/http/invoice_controller.go", transportControllerSource())
	writeFixtureFile(t, root, "app/routes.go", `package app

func ProvideRoutes(invoiceController *http.InvoiceController) {
	_ = invoiceController.Routes()
}
`)
	writeFixtureFile(t, root, "app/wire/inject_http_controllers_app.go", `package wire

var appHTTPControllerSet = wire.NewSet(http.NewInvoiceController)
`)
	result, err := NewAddHTTPControllerVerifier(&fakeCommandRunner{}).Verify(context.Background(), VerificationInput{ProjectRoot: root})
	if err != nil {
		t.Fatalf("Verify(): %v", err)
	}
	if result.FrameworkOutcome.Status != EndpointPassed {
		t.Fatalf("framework outcome = %#v; checks = %#v", result.FrameworkOutcome, result.Checks)
	}
}

// TestAddHTTPControllerVerifierRejectsMissingDerivedConstructorRegistration proves candidate-specific constructor discovery remains mandatory.
func TestAddHTTPControllerVerifierRejectsMissingDerivedConstructorRegistration(t *testing.T) {
	root := writeControllerFixture(t, validControllerSource("service *Service"), true)
	writeFixtureFile(t, root, "app/wire/inject_http_controllers_app.go", "package wire\n")
	result, err := NewAddHTTPControllerVerifier(&fakeCommandRunner{}).Verify(context.Background(), VerificationInput{ProjectRoot: root})
	if err != nil {
		t.Fatalf("Verify(): %v", err)
	}
	if !checkHasStatus(result.Checks, "wire-registration", EndpointFailed) {
		t.Fatalf("checks do not reject missing derived constructor registration: %#v", result.Checks)
	}
}

// TestAddHTTPControllerVerifierRejectsWrongTransportRoute retains useful route diagnostics for named transport controllers.
func TestAddHTTPControllerVerifierRejectsWrongTransportRoute(t *testing.T) {
	root := writeControllerFixture(t, strings.Replace(transportControllerSource(), "/invoices/:id", "/payments/:id", 1), true)
	if err := os.Remove(filepath.Join(root, "internal", "invoices", "controller.go")); err != nil {
		t.Fatal(err)
	}
	writeFixtureFile(t, root, "internal/http/invoice_controller.go", strings.Replace(transportControllerSource(), "/invoices/:id", "/payments/:id", 1))
	result, err := NewAddHTTPControllerVerifier(&fakeCommandRunner{}).Verify(context.Background(), VerificationInput{ProjectRoot: root})
	if err != nil {
		t.Fatalf("Verify(): %v", err)
	}
	if !checkHasStatus(result.Checks, "invoice-route", EndpointFailed) {
		t.Fatalf("checks do not retain route diagnostics for named transport controllers: %#v", result.Checks)
	}
}

// TestAddHTTPControllerVerifierRejectsTargetedMutants proves each core invariant can fail independently.
func TestAddHTTPControllerVerifierRejectsTargetedMutants(t *testing.T) {
	valid := validControllerSource("service *Service")
	tests := []struct {
		name         string
		controller   string
		registration bool
		failCommand  string
		failContains string
		wantCheck    string
	}{
		{name: "repository in controller", controller: strings.Replace(valid, "service *Service", "repository Repository", 1), registration: true, wantCheck: "thin-controller"},
		{name: "qualified repository in controller", controller: strings.Replace(valid, "service *Service", "repository *invoices.Repository", 1), registration: true, wantCheck: "thin-controller"},
		{name: "wrong route", controller: strings.Replace(valid, "/invoices/:id", "/payments/:id", 1), registration: true, wantCheck: "invoice-route"},
		{name: "missing context", controller: strings.Replace(valid, "request.Context()", "context.Background()", 1), registration: true, wantCheck: "invoice-handler"},
		{name: "missing registration", controller: valid, registration: false, wantCheck: "route-registration"},
		{name: "behavior oracle failure", controller: valid, registration: true, failContains: "./internal/invoices", wantCheck: "invoice-behavior"},
		{name: "build failure", controller: valid, registration: true, failCommand: "build", wantCheck: "app-build"},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			root := writeControllerFixture(t, test.controller, test.registration)
			result, err := NewAddHTTPControllerVerifier(&fakeCommandRunner{failCommand: test.failCommand, failContains: test.failContains}).Verify(context.Background(), VerificationInput{ProjectRoot: root})
			if err != nil {
				t.Fatalf("Verify(): %v", err)
			}
			if result.FrameworkOutcome.Status != EndpointFailed {
				t.Fatalf("mutant passed: %#v", result)
			}
			if !checkHasStatus(result.Checks, test.wantCheck, EndpointFailed) {
				t.Fatalf("checks do not fail %q: %#v", test.wantCheck, result.Checks)
			}
		})
	}
}

// TestAddHTTPControllerVerifierRejectsUnrelatedChanges relies only on the supervisor baseline projection.
func TestAddHTTPControllerVerifierRejectsUnrelatedChanges(t *testing.T) {
	root := writeControllerFixture(t, validControllerSource("service *Service"), true)
	for _, test := range []struct {
		name   string
		path   string
		wantID string
	}{
		{name: "unrelated readme", path: "README.md", wantID: "change-ownership"},
		{name: "seeded service", path: "internal/invoices/service.go", wantID: "change-ownership"},
		{name: "seeded repository", path: "internal/invoices/repository.go", wantID: "change-ownership"},
		{name: "seeded model", path: "internal/invoices/invoice.go", wantID: "change-ownership"},
	} {
		t.Run(test.name, func(t *testing.T) {
			result, err := NewAddHTTPControllerVerifier(&fakeCommandRunner{}).Verify(context.Background(), VerificationInput{
				ProjectRoot: root,
				Changes:     []ProjectChange{{Path: test.path, Before: ProjectPathState{Kind: "file"}, After: ProjectPathState{Kind: "file"}}},
			})
			if err != nil {
				t.Fatalf("Verify(): %v", err)
			}
			if !checkHasStatus(result.Checks, test.wantID, EndpointFailed) {
				t.Fatalf("checks = %#v, want %q failure", result.Checks, test.wantID)
			}
		})
	}
}

// TestOwnershipChecksAllowsDerivedWireOutput leaves authorship enforcement to trusted workflow evidence while executable checks prove the result.
func TestOwnershipChecksAllowsDerivedWireOutput(t *testing.T) {
	checks := ownershipChecks([]ProjectChange{{Path: "app/wire/wire_gen.go", Before: ProjectPathState{Kind: "file"}, After: ProjectPathState{Kind: "file"}}})
	if !checkHasStatus(checks, "generated-file-ownership", EndpointPassed) || !checkHasStatus(checks, "change-ownership", EndpointPassed) {
		t.Fatalf("ownership checks = %#v", checks)
	}
}

// TestOwnershipChecksAllowsDerivedBuildOutputs keeps controller ownership aligned with the shared framework verifier policy.
func TestOwnershipChecksAllowsDerivedBuildOutputs(t *testing.T) {
	checks := ownershipChecks([]ProjectChange{
		{Path: "go.sum"},
		{Path: "bin/.app.ready"},
		{Path: "build/api_index.json"},
	})
	if !checkHasStatus(checks, "change-ownership", EndpointPassed) {
		t.Fatalf("ownership checks = %#v", checks)
	}
}

// TestOwnershipChecksAllowsFocusedControllerTests permits candidate tests that exercise the controller without expanding the domain change budget.
func TestOwnershipChecksAllowsFocusedControllerTests(t *testing.T) {
	for _, path := range []string{
		"internal/invoices/controller_test.go",
		"internal/http/invoice_controller_test.go",
	} {
		t.Run(path, func(t *testing.T) {
			checks := ownershipChecks([]ProjectChange{{Path: path, Before: ProjectPathState{Kind: "file"}, After: ProjectPathState{Kind: "file"}}})
			if !checkHasStatus(checks, "change-ownership", EndpointPassed) {
				t.Fatalf("ownership checks = %#v", checks)
			}
		})
	}
}

// TestControllerHandlerCheckRequiresFindRequestContext rejects unrelated Context calls when Find receives a background context.
func TestControllerHandlerCheckRequiresFindRequestContext(t *testing.T) {
	valid, err := parser.ParseFile(token.NewFileSet(), "controller.go", validControllerSource("service *Service"), 0)
	if err != nil {
		t.Fatalf("parse valid controller: %v", err)
	}
	if result := controllerHandlerCheck(valid); result.Status != EndpointPassed {
		t.Fatalf("valid handler result = %#v", result)
	}
	standardRequestSource := strings.Replace(validControllerSource("service *Service"), "request.Context()", "request.Request().Context()", 1)
	standardRequest, err := parser.ParseFile(token.NewFileSet(), "controller.go", standardRequestSource, 0)
	if err != nil {
		t.Fatalf("parse standard request controller: %v", err)
	}
	if result := controllerHandlerCheck(standardRequest); result.Status != EndpointPassed {
		t.Fatalf("standard request handler result = %#v", result)
	}
	decoySource := strings.Replace(validControllerSource("service *Service"), "invoice, err := controller.service.Find(request.Context(), request.Param(\"id\"))", "_ = request.Context()\n\tinvoice, err := controller.service.Find(context.Background(), request.Param(\"id\"))", 1)
	decoy, err := parser.ParseFile(token.NewFileSet(), "controller.go", decoySource, 0)
	if err != nil {
		t.Fatalf("parse decoy controller: %v", err)
	}
	if result := controllerHandlerCheck(decoy); result.Status != EndpointFailed || !strings.Contains(result.Details, "Context") {
		t.Fatalf("decoy handler result = %#v", result)
	}
}

// TestControllerHandlerCheckRejectsRouteBoundHardCodedResponse ensures unrelated compliant methods cannot satisfy the selected route's contract.
func TestControllerHandlerCheckRejectsRouteBoundHardCodedResponse(t *testing.T) {
	valid := validControllerSource("service *Service")
	hardCodedShow := `func (controller *Controller) Show(request web.Context) error {
	return request.JSON(http.StatusOK, map[string]string{"id": "inv-42", "status": "open"})
}
`
	unusedCompliantMethod := strings.Replace(valid[strings.Index(valid, "func (controller *Controller) Show"):], "Show", "LookupInvoice", 1)
	mutant := valid[:strings.Index(valid, "func (controller *Controller) Show")] + hardCodedShow + "\n" + unusedCompliantMethod
	file, err := parser.ParseFile(token.NewFileSet(), "controller.go", mutant, 0)
	if err != nil {
		t.Fatalf("parse hard-coded response mutant: %v", err)
	}
	if result := controllerHandlerCheck(file); result.Status != EndpointFailed || !strings.Contains(result.Details, "Context") {
		t.Fatalf("hard-coded Show was accepted: %#v", result)
	}
}

// TestInvoiceRouteOwnerRejectsDecoyReceiver prevents a compliant decoy handler from substituting for the handler registered by Routes.
func TestInvoiceRouteOwnerRejectsDecoyReceiver(t *testing.T) {
	fileSet := token.NewFileSet()
	file, err := parser.ParseFile(fileSet, "controller.go", decoyReceiverControllerSource(), 0)
	if err != nil {
		t.Fatalf("parse decoy receiver controller: %v", err)
	}
	if _, err := (&types.Config{Importer: importer.Default()}).Check("invoices", fileSet, []*ast.File{file}, nil); err != nil {
		t.Fatalf("type-check decoy receiver controller: %v", err)
	}
	if ownerName, handlerName := invoiceRouteOwner(file); ownerName != "" || handlerName != "" {
		t.Fatalf("decoy route owner = %q, handler = %q", ownerName, handlerName)
	}
	if result := controllerHandlerCheck(file); result.Status != EndpointFailed {
		t.Fatalf("decoy handler result = %#v", result)
	}

	root := t.TempDir()
	writeFixtureFile(t, root, "go.mod", "module example.com/invoiceeval\n\ngo 1.25.0\n")
	writeFixtureFile(t, root, "internal/invoices/controller.go", decoyReceiverControllerSource())
	if _, err := resolveInvoiceBehaviorProbe(root); err == nil {
		t.Fatal("behavior probe resolved a handler that Routes does not invoke on its receiver")
	}
}

// TestInvoiceRouteOwnerRejectsShadowedReceiver prevents a same-spelling local from impersonating the Routes receiver.
func TestInvoiceRouteOwnerRejectsShadowedReceiver(t *testing.T) {
	source := strings.Replace(decoyReceiverControllerSource(), `func (controller *Controller) Routes() []Route {
	return []Route{NewRoute(http.MethodGet, "/invoices/:id", controller.decoy.Show)}
}`, `func (controller *Controller) Routes() []Route {
	{
		controller := controller.decoy
		return []Route{NewRoute(http.MethodGet, "/invoices/:id", controller.Show)}
	}
}`, 1)
	fileSet := token.NewFileSet()
	file, err := parser.ParseFile(fileSet, "controller.go", source, 0)
	if err != nil {
		t.Fatalf("parse shadowed receiver controller: %v", err)
	}
	if _, err := (&types.Config{Importer: importer.Default()}).Check("invoices", fileSet, []*ast.File{file}, nil); err != nil {
		t.Fatalf("type-check shadowed receiver controller: %v", err)
	}
	if ownerName, handlerName := invoiceRouteOwner(file); ownerName != "" || handlerName != "" {
		t.Fatalf("shadowed route owner = %q, handler = %q", ownerName, handlerName)
	}
}

// writeControllerFixture materializes only the candidate-owned sources inspected by the verifier.
func writeControllerFixture(t *testing.T, controller string, registered bool) string {
	t.Helper()
	root := t.TempDir()
	writes := map[string]string{
		"go.mod":                          "module example.com/invoiceeval\n\ngo 1.25.0\n",
		"internal/invoices/controller.go": controller,
		"app/routes.go": `package app

func ProvideRoutes(invoicesController *invoices.Controller) {
	_ = invoicesController.Routes()
}
`,
		"app/wire/inject_http_controllers_app.go": `package wire

var appHTTPControllerSet = wire.NewSet(invoices.NewController)
`,
	}
	if !registered {
		writes["app/routes.go"] = "package app\n"
	}
	for path, body := range writes {
		writeFixtureFile(t, root, path, body)
	}
	return root
}

// writeFixtureFile writes one verifier fixture source beneath its disposable project root.
func writeFixtureFile(t *testing.T, root string, path string, body string) {
	t.Helper()
	absolute := filepath.Join(root, filepath.FromSlash(path))
	if err := os.MkdirAll(filepath.Dir(absolute), 0o755); err != nil {
		t.Fatalf("create fixture directory: %v", err)
	}
	if err := os.WriteFile(absolute, []byte(body), 0o644); err != nil {
		t.Fatalf("write fixture file: %v", err)
	}
}

// validControllerSource returns one candidate shape whose boundary type can vary independently from behavior.
func validControllerSource(dependency string) string {
	return `package invoices

import (
	"errors"
	"net/http"
)

type Controller struct {
	` + dependency + `
}

func NewController() *Controller {
	return &Controller{}
}

func (controller *Controller) Routes() []web.Route {
	return []web.Route{web.NewRoute(http.MethodGet, "/invoices/:id", controller.Show)}
}

func (controller *Controller) Show(request web.Context) error {
	invoice, err := controller.service.Find(request.Context(), request.Param("id"))
	if errors.Is(err, ErrInvoiceNotFound) {
		return request.JSON(http.StatusNotFound, map[string]string{"error": "invoice not found"})
	}
	if err != nil {
		return err
	}
	return request.JSON(http.StatusOK, invoice)
}
`
}

// decoyReceiverControllerSource defines a type-correct controller whose route points at a different receiver.
func decoyReceiverControllerSource() string {
	return `package invoices

import (
	"errors"
	"net/http"
)

type Route struct{}

func NewRoute(_ string, _ string, _ func(Context) error) Route { return Route{} }

type Context struct{}

func (Context) Context() any { return nil }
func (Context) Param(string) string { return "" }
func (Context) JSON(int, any) error { return nil }

type Invoice struct{}

type Service struct{}

func (*Service) Find(any, string) (Invoice, error) { return Invoice{}, nil }

var ErrInvoiceNotFound = errors.New("not found")

type decoyReceiver struct {
	service *Service
}

func (decoy *decoyReceiver) Show(request Context) error {
	invoice, err := decoy.service.Find(request.Context(), request.Param("id"))
	if errors.Is(err, ErrInvoiceNotFound) {
		return request.JSON(http.StatusNotFound, map[string]string{"error": "invoice not found"})
	}
	if err != nil {
		return err
	}
	return request.JSON(http.StatusOK, invoice)
}

type Controller struct {
	service *Service
	decoy   *decoyReceiver
}

func (controller *Controller) Routes() []Route {
	return []Route{NewRoute(http.MethodGet, "/invoices/:id", controller.decoy.Show)}
}
`
}

// transportControllerSource returns the independent implementation family observed in the unassisted live trial.
func transportControllerSource() string {
	return `package http

import (
	"errors"
	"net/http"
)

type InvoiceController struct {
	service *invoices.Service
}

func NewInvoiceController(service *invoices.Service) *InvoiceController {
	return &InvoiceController{service: service}
}

func (controller *InvoiceController) Routes() []web.Route {
	return []web.Route{web.NewRoute(http.MethodGet, "/invoices/:id", controller.Show)}
}

func (controller *InvoiceController) Show(request web.Context) error {
	invoice, err := controller.service.Find(request.Context(), request.Param("id"))
	if errors.Is(err, invoices.ErrInvoiceNotFound) {
		return request.JSON(http.StatusNotFound, map[string]string{"error": "invoice not found"})
	}
	if err != nil {
		return err
	}
	return request.JSON(http.StatusOK, invoice)
}
`
}

// checkHasStatus finds one named verifier check without depending on result order.
func checkHasStatus(checks []EndpointResult, id string, status EndpointStatus) bool {
	for _, check := range checks {
		if check.ID == id && check.Status == status {
			return true
		}
	}
	return false
}
