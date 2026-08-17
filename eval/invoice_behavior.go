package eval

import (
	"bufio"
	"bytes"
	"context"
	"fmt"
	"go/ast"
	"go/format"
	"go/token"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"text/template"
)

// invoiceControllerSource retains the semantic controller and its path without imposing the golden package placement.
type invoiceControllerSource struct {
	path string
	file *ast.File
}

// invoiceBehaviorProbe describes the smallest supervisor-owned test that can exercise the discovered controller family.
type invoiceBehaviorProbe struct {
	relativeDirectory string
	packageName       string
	ownerName         string
	handlerName       string
	boundaryField     string
	boundaryType      string
	modulePath        string
}

// invoiceBehaviorTemplateData names every implementation-family choice projected into the trusted oracle.
type invoiceBehaviorTemplateData struct {
	PackageName        string
	HTTPImport         string
	HTTPPackage        string
	InvoiceImport      string
	InvoicePackage     string
	UsesBoundaryStub   bool
	OwnerName          string
	BoundaryField      string
	BoundaryExpression string
	HandlerName        string
}

const invoiceBehaviorProbeTemplate = `package {{.PackageName}}

import (
{{- if .UsesBoundaryStub}}
	"context"
{{- end}}
	"encoding/json"
	{{.HTTPImport}}
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/goforj/web/webtest"
{{- if .InvoiceImport}}
	{{.InvoiceImport}}
{{- end}}
)
{{if .UsesBoundaryStub}}
// atlasInvoiceQueryStub isolates transport behavior from candidate-authored tests while preserving the expected application boundary.
type atlasInvoiceQueryStub struct{}

// Find returns the stable scenario behavior through any compatible query-style boundary.
func (atlasInvoiceQueryStub) Find(_ context.Context, id string) ({{.InvoicePackage}}Invoice, error) {
	if id == "missing" {
		return {{.InvoicePackage}}Invoice{}, {{.InvoicePackage}}ErrInvoiceNotFound
	}
	return {{.InvoicePackage}}Invoice{ID: id, CustomerID: "customer-42", TotalCents: 12500, Status: "open"}, nil
}
{{end}}
// TestAtlasInvoiceHTTPBehavior independently proves the required success and not-found HTTP contract.
func TestAtlasInvoiceHTTPBehavior(t *testing.T) {
	controller := &{{.OwnerName}}{ {{- .BoundaryField}}: {{.BoundaryExpression}}}
	tests := []struct {
		name       string
		id         string
		statusCode int
	}{
		{name: "found", id: "inv-42", statusCode: {{.HTTPPackage}}.StatusOK},
		{name: "missing", id: "missing", statusCode: {{.HTTPPackage}}.StatusNotFound},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			request := httptest.NewRequest({{.HTTPPackage}}.MethodGet, "/invoices/"+test.id, nil)
			recorder := httptest.NewRecorder()
			requestContext := webtest.NewContext(request, recorder, "/invoices/:id", webtest.PathParams{"id": test.id})
			if err := controller.{{.HandlerName}}(requestContext); err != nil {
				t.Fatalf("{{.HandlerName}}(): %v", err)
			}
			if recorder.Code != test.statusCode {
				t.Fatalf("status = %d, want %d; body = %s", recorder.Code, test.statusCode, recorder.Body.String())
			}
			if test.statusCode == {{.HTTPPackage}}.StatusOK {
				var invoice {{.InvoicePackage}}Invoice
				if err := json.Unmarshal(recorder.Body.Bytes(), &invoice); err != nil {
					t.Fatalf("decode invoice: %v", err)
				}
				if invoice.ID != "inv-42" || invoice.CustomerID != "customer-42" || invoice.TotalCents != 12500 || invoice.Status != "open" {
					t.Fatalf("invoice = %#v", invoice)
				}
				return
			}
			var response map[string]any
			if err := json.Unmarshal(recorder.Body.Bytes(), &response); err != nil {
				t.Fatalf("decode error response: %v", err)
			}
			errorMessage, ok := response["error"].(string)
			if !ok || strings.TrimSpace(errorMessage) == "" {
				t.Fatalf("error response = %#v", response)
			}
		})
	}
}
`

// runInvoiceBehaviorProbe installs a hidden black-box oracle in a dedicated verifier clone after the agent has stopped.
func runInvoiceBehaviorProbe(ctx context.Context, runner CommandRunner, project VerifierProject) (result EndpointResult) {
	probe, err := resolveInvoiceBehaviorProbe(project.Root)
	if err != nil {
		return EndpointResult{ID: "invoice-behavior", Status: EndpointFailed, Details: err.Error()}
	}
	body, err := renderInvoiceBehaviorProbe(probe)
	if err != nil {
		return EndpointResult{ID: "invoice-behavior", Status: EndpointFailed, Details: err.Error()}
	}
	session, err := runner.Open(ctx, project)
	if err != nil {
		return EndpointResult{ID: "invoice-behavior", Status: EndpointFailed, Details: fmt.Sprintf("open isolated verifier session: %v", err)}
	}
	defer func() {
		cleanupContext, cancel := context.WithTimeout(context.Background(), verifierCleanupTimeout)
		defer cancel()
		if closeErr := session.Close(cleanupContext); closeErr != nil && result.Status == EndpointPassed {
			result = EndpointResult{ID: "invoice-behavior", Status: EndpointFailed, Details: fmt.Sprintf("close isolated verifier session: %v", closeErr)}
		}
	}()
	relativePath := filepath.Join(probe.relativeDirectory, "atlas_eval_invoice_behavior_test.go")
	if err := session.WriteFile(relativePath, body); err != nil {
		return EndpointResult{ID: "invoice-behavior", Status: EndpointFailed, Details: err.Error()}
	}
	const testName = "TestAtlasInvoiceHTTPBehavior"
	command := []string{"go", "test", "./" + filepath.ToSlash(probe.relativeDirectory), "-run", "^" + testName + "$", "-count=1"}
	command, marker, err := installSupervisorCompletionMarker(session, supervisorFile{path: relativePath, body: string(body)}, command)
	if err != nil {
		return EndpointResult{ID: "invoice-behavior", Status: EndpointFailed, Details: err.Error()}
	}
	output, err := session.Run(ctx, command)
	return invoiceBehaviorExecutionResult(output, marker, err)
}

// invoiceBehaviorExecutionResult accepts diagnostic success only after the supervisor-authored test reaches its completion marker.
func invoiceBehaviorExecutionResult(output, marker string, err error) EndpointResult {
	if err != nil {
		return EndpointResult{ID: "invoice-behavior", Status: EndpointFailed, Details: err.Error()}
	}
	if marker == "" || !strings.Contains(output, marker) {
		return EndpointResult{ID: "invoice-behavior", Status: EndpointFailed, Details: "supervisor completion marker was not observed"}
	}
	return EndpointResult{ID: "invoice-behavior", Status: EndpointPassed}
}

// resolveInvoiceBehaviorProbe derives only names and placement while keeping expected behavior owned by the verifier contract.
func resolveInvoiceBehaviorProbe(root string) (invoiceBehaviorProbe, error) {
	source, err := findInvoiceController(root)
	if err != nil {
		return invoiceBehaviorProbe{}, err
	}
	ownerName, handlerName := invoiceRouteOwner(source.file)
	if ownerName == "" || handlerName == "" {
		return invoiceBehaviorProbe{}, fmt.Errorf("invoice route owner and handler could not be derived")
	}
	fieldName, boundaryType := invoiceBoundaryField(source.file, ownerName)
	if fieldName == "" || boundaryType == "" {
		return invoiceBehaviorProbe{}, fmt.Errorf("invoice controller application boundary could not be derived")
	}
	relativeDirectory, err := filepath.Rel(root, filepath.Dir(source.path))
	if err != nil {
		return invoiceBehaviorProbe{}, fmt.Errorf("resolve invoice controller directory: %w", err)
	}
	if !filepath.IsLocal(relativeDirectory) {
		return invoiceBehaviorProbe{}, fmt.Errorf("invoice controller directory %q escapes the Project", relativeDirectory)
	}
	modulePath, err := readModulePath(filepath.Join(root, "go.mod"))
	if err != nil {
		return invoiceBehaviorProbe{}, err
	}
	return invoiceBehaviorProbe{
		relativeDirectory: relativeDirectory,
		packageName:       source.file.Name.Name,
		ownerName:         ownerName,
		handlerName:       handlerName,
		boundaryField:     fieldName,
		boundaryType:      boundaryType,
		modulePath:        modulePath,
	}, nil
}

// invoiceRouteOwner returns the receiver and handler attached to the exact required route declaration.
func invoiceRouteOwner(file *ast.File) (string, string) {
	for _, declaration := range file.Decls {
		function, ok := declaration.(*ast.FuncDecl)
		if !ok || function.Recv == nil || function.Name.Name != "Routes" || function.Body == nil || len(function.Recv.List) == 0 {
			continue
		}
		ownerName := expressionTypeName(function.Recv.List[0].Type)
		handlerName := ""
		ast.Inspect(function.Body, func(node ast.Node) bool {
			call, ok := node.(*ast.CallExpr)
			if !ok || len(call.Args) < 3 || !isInvoiceGETRoute(call.Args[0], call.Args[1]) {
				return true
			}
			selector, ok := call.Args[2].(*ast.SelectorExpr)
			if ok && routeHandlerSelectorUsesReceiver(function, selector) {
				handlerName = selector.Sel.Name
			}
			return false
		})
		if handlerName != "" {
			return ownerName, handlerName
		}
	}
	return "", ""
}

// routeHandlerSelectorUsesReceiver reports whether a route handler selector is bound to the Routes method receiver.
func routeHandlerSelectorUsesReceiver(function *ast.FuncDecl, selector *ast.SelectorExpr) bool {
	if function.Recv == nil || len(function.Recv.List) != 1 || len(function.Recv.List[0].Names) != 1 {
		return false
	}
	receiver, ok := selector.X.(*ast.Ident)
	receiverDeclaration := function.Recv.List[0].Names[0]
	return ok && receiver.Obj != nil && receiver.Obj == receiverDeclaration.Obj
}

// isInvoiceGETRoute recognizes the contract route without depending on the router constructor's imported name.
func isInvoiceGETRoute(method ast.Expr, path ast.Expr) bool {
	selector, ok := method.(*ast.SelectorExpr)
	if !ok || selector.Sel.Name != "MethodGet" {
		return false
	}
	literal, ok := path.(*ast.BasicLit)
	return ok && literal.Kind == token.STRING && strings.Trim(literal.Value, "\"") == "/invoices/:id"
}

// invoiceBoundaryField locates the field admitted by the controller boundary policy for the discovered route owner.
func invoiceBoundaryField(file *ast.File, ownerName string) (string, string) {
	for _, declaration := range file.Decls {
		general, ok := declaration.(*ast.GenDecl)
		if !ok {
			continue
		}
		for _, specification := range general.Specs {
			typeSpec, ok := specification.(*ast.TypeSpec)
			if !ok || typeSpec.Name.Name != ownerName {
				continue
			}
			structure, ok := typeSpec.Type.(*ast.StructType)
			if !ok {
				return "", ""
			}
			for _, field := range structure.Fields.List {
				boundaryType := expressionTypeName(field.Type)
				if !isApplicationBoundaryName(boundaryType) || len(field.Names) != 1 {
					continue
				}
				return field.Names[0].Name, boundaryType
			}
		}
	}
	return "", ""
}

// readModulePath reads the one module identity needed to import the stable invoice fixture from an independent transport package.
func readModulePath(path string) (string, error) {
	file, err := os.Open(path)
	if err != nil {
		return "", fmt.Errorf("open verifier go.mod: %w", err)
	}
	defer file.Close()
	scanner := bufio.NewScanner(file)
	for scanner.Scan() {
		fields := strings.Fields(scanner.Text())
		if len(fields) == 2 && fields[0] == "module" {
			return fields[1], nil
		}
	}
	if err := scanner.Err(); err != nil {
		return "", fmt.Errorf("read verifier go.mod: %w", err)
	}
	return "", fmt.Errorf("verifier go.mod does not declare a module")
}

// renderInvoiceBehaviorProbe creates an independent 200/404 oracle for same-package and transport-package controller families.
func renderInvoiceBehaviorProbe(probe invoiceBehaviorProbe) ([]byte, error) {
	httpPackage := "http"
	httpImport := strconv.Quote("net/http")
	if probe.packageName == "http" {
		httpPackage = "stdhttp"
		httpImport = "stdhttp " + httpImport
	}
	invoicePackage := ""
	invoiceImport := ""
	if probe.packageName != "invoices" {
		invoicePackage = "invoices."
		invoiceImport = strconv.Quote(probe.modulePath + "/internal/invoices")
	}
	usesBoundaryStub := !strings.Contains(strings.ToLower(probe.boundaryType), "service")
	boundaryExpression := invoicePackage + "NewService(" + invoicePackage + "NewRepository())"
	if usesBoundaryStub {
		boundaryExpression = "atlasInvoiceQueryStub{}"
	}
	templateBody, err := template.New("invoice behavior probe").Parse(invoiceBehaviorProbeTemplate)
	if err != nil {
		return nil, fmt.Errorf("parse invoice behavior probe: %w", err)
	}
	var source bytes.Buffer
	if err := templateBody.Execute(&source, invoiceBehaviorTemplateData{
		PackageName:        probe.packageName,
		HTTPImport:         httpImport,
		HTTPPackage:        httpPackage,
		InvoiceImport:      invoiceImport,
		InvoicePackage:     invoicePackage,
		UsesBoundaryStub:   usesBoundaryStub,
		OwnerName:          probe.ownerName,
		BoundaryField:      probe.boundaryField,
		BoundaryExpression: boundaryExpression,
		HandlerName:        probe.handlerName,
	}); err != nil {
		return nil, fmt.Errorf("execute invoice behavior probe: %w", err)
	}
	formatted, err := format.Source(source.Bytes())
	if err != nil {
		return nil, fmt.Errorf("format invoice behavior probe: %w", err)
	}
	return formatted, nil
}
