package eval

import (
	"context"
	"fmt"
	"go/ast"
	"go/parser"
	"go/token"
	"os"
	"path/filepath"
	"strings"
	"time"
)

const verifierCleanupTimeout = 10 * time.Second

// CommandRunner opens verifier-owned black-box checks in an isolated clone of the sealed candidate tree.
type CommandRunner interface {
	// Open creates one disposable verifier session from sealed candidate evidence.
	Open(context.Context, string) (CommandSession, error)
}

// CommandSession executes a bounded sequence against one disposable candidate clone.
type CommandSession interface {
	// WriteFile installs supervisor-owned verifier input inside the disposable Project.
	WriteFile(string, []byte) error
	// Run executes one allowlisted verifier command.
	Run(context.Context, []string) (string, error)
	// Close destroys the disposable Project and any supervisor-owned input.
	Close(context.Context) error
}

// AddHTTPControllerVerifier verifies invoice HTTP behavior independently from GoForj's golden scenario steps.
type AddHTTPControllerVerifier struct {
	runner CommandRunner
}

// NewAddHTTPControllerVerifier creates the promoted verifier with a trusted isolated command runner.
func NewAddHTTPControllerVerifier(runner CommandRunner) *AddHTTPControllerVerifier {
	return &AddHTTPControllerVerifier{runner: runner}
}

// PromotedVerifiers returns every live verifier that can run through the selected isolated command boundary.
func PromotedVerifiers(runner CommandRunner) []Verifier {
	return []Verifier{NewAddHTTPControllerVerifier(runner)}
}

// ID returns the promoted verifier contract identity.
func (*AddHTTPControllerVerifier) ID() string {
	return "add-http-controller/v1"
}

// Capabilities returns no agent-observation requirements because outcome checks inspect a sealed tree through supervisor-owned boundaries.
func (*AddHTTPControllerVerifier) Capabilities() []Capability {
	return nil
}

// Verify checks structure, compilation, registration, and route visibility without comparing candidate source to the golden recipe.
func (verifier *AddHTTPControllerVerifier) Verify(ctx context.Context, input VerificationInput) (VerificationResult, error) {
	if verifier == nil || verifier.runner == nil {
		return VerificationResult{}, fmt.Errorf("add HTTP controller verifier requires an isolated command runner")
	}
	session, err := verifier.runner.Open(ctx, input.ProjectRoot)
	if err != nil {
		return VerificationResult{}, fmt.Errorf("open isolated verifier session: %w", err)
	}
	checks := make([]EndpointResult, 0, 10)
	checks = append(checks, inspectInvoiceController(input.ProjectRoot)...)
	checks = append(checks, runCheck(ctx, session, "project-tests", []string{"go", "test", "./..."}, ""))
	checks = append(checks, runInvoiceBehaviorProbe(ctx, session, input.ProjectRoot))
	checks = append(checks, runCheck(ctx, session, "app-build", []string{"forj", "build"}, ""))
	checks = append(checks, runCheck(ctx, session, "route-visible", []string{"forj", "route:list"}, "/api/v1/invoices/:id"))

	failed := 0
	for _, check := range checks {
		if check.Status != EndpointPassed {
			failed++
		}
	}
	framework := EndpointResult{ID: verifier.ID(), Status: EndpointPassed}
	if failed > 0 {
		framework.Status = EndpointFailed
		framework.Details = fmt.Sprintf("%d of %d framework checks failed", failed, len(checks))
	}
	result := VerificationResult{
		FrameworkOutcome:    framework,
		WorkflowConformance: EndpointResult{ID: "workflow-owned-by-runner", Status: EndpointIneligible},
		Checks:              checks,
	}
	cleanupContext, cancel := context.WithTimeout(context.Background(), verifierCleanupTimeout)
	defer cancel()
	if err := session.Close(cleanupContext); err != nil {
		return VerificationResult{}, fmt.Errorf("close isolated verifier session: %w", err)
	}
	return result, nil
}

// runCheck delegates executable behavior to the trusted verifier environment rather than the candidate's agent session.
func runCheck(ctx context.Context, session CommandSession, id string, command []string, contains string) EndpointResult {
	output, err := session.Run(ctx, command)
	if err != nil {
		return EndpointResult{ID: id, Status: EndpointFailed, Details: err.Error()}
	}
	if contains != "" && !strings.Contains(output, contains) {
		return EndpointResult{ID: id, Status: EndpointFailed, Details: fmt.Sprintf("output does not contain %q", contains)}
	}
	return EndpointResult{ID: id, Status: EndpointPassed}
}

// inspectInvoiceController accepts multiple application-boundary shapes while rejecting repository work in the transport.
func inspectInvoiceController(root string) []EndpointResult {
	source, err := findInvoiceController(root)
	if err != nil {
		return []EndpointResult{{ID: "controller-source", Status: EndpointFailed, Details: err.Error()}}
	}
	controller := source.file
	constructors := controllerConstructorNames(controller)
	checks := []EndpointResult{
		controllerBoundaryCheck(controller),
		controllerRouteCheck(controller),
		controllerHandlerCheck(controller),
		selectorRegistrationCheck(filepath.Join(root, "app", "routes.go"), "route-registration", func(selector *ast.SelectorExpr) bool {
			return selector.Sel.Name == "Routes" && expressionContainsInvoice(selector.X)
		}),
		selectorRegistrationCheck(filepath.Join(root, "app", "wire", "inject_http_controllers_app.go"), "wire-registration", func(selector *ast.SelectorExpr) bool {
			return constructors[selector.Sel.Name]
		}),
	}
	return checks
}

// findInvoiceController locates the semantic route owner without requiring the golden package layout.
func findInvoiceController(root string) (invoiceControllerSource, error) {
	internalRoot := filepath.Join(root, "internal")
	var controller invoiceControllerSource
	var fallback invoiceControllerSource
	err := filepath.WalkDir(internalRoot, func(path string, entry os.DirEntry, walkErr error) error {
		if walkErr != nil {
			return walkErr
		}
		if controller.file != nil || entry.IsDir() || filepath.Ext(path) != ".go" || strings.HasSuffix(path, "_test.go") {
			return nil
		}
		file, err := parser.ParseFile(token.NewFileSet(), path, nil, parser.SkipObjectResolution)
		if err != nil {
			return nil
		}
		if controllerRouteCheck(file).Status == EndpointPassed {
			controller = invoiceControllerSource{path: path, file: file}
		} else if fallback.file == nil && declaresController(file) {
			fallback = invoiceControllerSource{path: path, file: file}
		}
		return nil
	})
	if err != nil {
		return invoiceControllerSource{}, err
	}
	if controller.file == nil {
		controller = fallback
	}
	if controller.file == nil {
		return invoiceControllerSource{}, fmt.Errorf("no controller source exists under internal/")
	}
	return controller, nil
}

// declaresController identifies conventional transport candidates when the expected route is absent.
func declaresController(file *ast.File) bool {
	for _, declaration := range file.Decls {
		general, ok := declaration.(*ast.GenDecl)
		if !ok {
			continue
		}
		for _, specification := range general.Specs {
			typeSpec, ok := specification.(*ast.TypeSpec)
			if ok && strings.HasSuffix(typeSpec.Name.Name, "Controller") {
				return true
			}
		}
	}
	return false
}

// controllerBoundaryCheck requires an application boundary and rejects direct persistence collaborators in the HTTP controller.
func controllerBoundaryCheck(file *ast.File) EndpointResult {
	result := EndpointResult{ID: "thin-controller", Status: EndpointFailed, Details: "Controller must depend on an invoice service, query, use case, or domain handler rather than a repository"}
	owners := invoiceRouteOwnerNames(file)
	for _, declaration := range file.Decls {
		general, ok := declaration.(*ast.GenDecl)
		if !ok {
			continue
		}
		for _, specification := range general.Specs {
			typeSpec, ok := specification.(*ast.TypeSpec)
			if !ok || !owners[typeSpec.Name.Name] {
				continue
			}
			structure, ok := typeSpec.Type.(*ast.StructType)
			if !ok {
				return result
			}
			allowed := false
			for _, field := range structure.Fields.List {
				name := expressionTypeName(field.Type)
				if strings.Contains(strings.ToLower(name), "repository") || strings.Contains(strings.ToLower(name), "database") {
					return result
				}
				if isApplicationBoundaryName(name) {
					allowed = true
				}
			}
			if allowed {
				result.Status = EndpointPassed
				result.Details = ""
			}
			return result
		}
	}
	return result
}

// controllerRouteCheck proves the package declares the expected method and relative route without requiring a specific receiver name.
func controllerRouteCheck(file *ast.File) EndpointResult {
	foundMethod := false
	foundPath := false
	ast.Inspect(file, func(node ast.Node) bool {
		selector, ok := node.(*ast.SelectorExpr)
		if ok && selector.Sel.Name == "MethodGet" {
			foundMethod = true
		}
		literal, ok := node.(*ast.BasicLit)
		if ok && literal.Kind == token.STRING && strings.Trim(literal.Value, "\"") == "/invoices/:id" {
			foundPath = true
		}
		return true
	})
	if foundMethod && foundPath {
		return EndpointResult{ID: "invoice-route", Status: EndpointPassed}
	}
	return EndpointResult{ID: "invoice-route", Status: EndpointFailed, Details: "GET /invoices/:id is not declared by the invoice controller"}
}

// controllerHandlerCheck verifies context propagation, ID extraction, application lookup, and success/not-found response handling.
func controllerHandlerCheck(file *ast.File) EndpointResult {
	required := map[string]bool{"Context": false, "Param": false, "Find": false, "JSON": false, "StatusOK": false, "StatusNotFound": false}
	ast.Inspect(file, func(node ast.Node) bool {
		if call, ok := node.(*ast.CallExpr); ok {
			if selector, ok := call.Fun.(*ast.SelectorExpr); ok {
				if _, exists := required[selector.Sel.Name]; exists {
					required[selector.Sel.Name] = true
				}
			}
		}
		if selector, ok := node.(*ast.SelectorExpr); ok && (selector.Sel.Name == "StatusOK" || selector.Sel.Name == "StatusNotFound") {
			required[selector.Sel.Name] = true
		}
		return true
	})
	for name, present := range required {
		if !present {
			return EndpointResult{ID: "invoice-handler", Status: EndpointFailed, Details: "handler does not demonstrate " + name + " behavior"}
		}
	}
	return EndpointResult{ID: "invoice-handler", Status: EndpointPassed}
}

// selectorRegistrationCheck verifies an App-owned registration point references the discovered controller shape.
func selectorRegistrationCheck(path, id string, matches func(*ast.SelectorExpr) bool) EndpointResult {
	body, err := os.ReadFile(path)
	if err != nil {
		return EndpointResult{ID: id, Status: EndpointFailed, Details: err.Error()}
	}
	file, err := parser.ParseFile(token.NewFileSet(), path, body, parser.SkipObjectResolution)
	if err != nil {
		return EndpointResult{ID: id, Status: EndpointFailed, Details: err.Error()}
	}
	found := false
	ast.Inspect(file, func(node ast.Node) bool {
		selector, ok := node.(*ast.SelectorExpr)
		if !ok || !matches(selector) {
			return true
		}
		found = true
		return true
	})
	if found {
		return EndpointResult{ID: id, Status: EndpointPassed}
	}
	return EndpointResult{ID: id, Status: EndpointFailed, Details: "invoice registration is missing"}
}

// invoiceRouteOwnerNames returns the receiver types that own the required invoice route.
func invoiceRouteOwnerNames(file *ast.File) map[string]bool {
	owners := make(map[string]bool)
	for _, declaration := range file.Decls {
		function, ok := declaration.(*ast.FuncDecl)
		if !ok || function.Recv == nil || function.Name.Name != "Routes" || function.Body == nil {
			continue
		}
		methodGet := false
		invoicePath := false
		ast.Inspect(function.Body, func(node ast.Node) bool {
			if selector, ok := node.(*ast.SelectorExpr); ok && selector.Sel.Name == "MethodGet" {
				methodGet = true
			}
			if literal, ok := node.(*ast.BasicLit); ok && literal.Kind == token.STRING && strings.Trim(literal.Value, "\"") == "/invoices/:id" {
				invoicePath = true
			}
			return true
		})
		if methodGet && invoicePath && len(function.Recv.List) > 0 {
			owners[expressionTypeName(function.Recv.List[0].Type)] = true
		}
	}
	return owners
}

// controllerConstructorNames derives the constructors that return the route-owning controller type.
func controllerConstructorNames(file *ast.File) map[string]bool {
	owners := invoiceRouteOwnerNames(file)
	constructors := make(map[string]bool)
	for _, declaration := range file.Decls {
		function, ok := declaration.(*ast.FuncDecl)
		if !ok || function.Recv != nil || function.Type.Results == nil || !strings.HasPrefix(function.Name.Name, "New") {
			continue
		}
		for _, result := range function.Type.Results.List {
			if owners[expressionTypeName(result.Type)] {
				constructors[function.Name.Name] = true
			}
		}
	}
	return constructors
}

// expressionContainsInvoice identifies route variables and package references associated with invoices.
func expressionContainsInvoice(expression ast.Expr) bool {
	switch value := expression.(type) {
	case *ast.Ident:
		return strings.Contains(strings.ToLower(value.Name), "invoice")
	case *ast.SelectorExpr:
		return expressionContainsInvoice(value.X) || strings.Contains(strings.ToLower(value.Sel.Name), "invoice")
	default:
		return false
	}
}

// expressionTypeName returns the semantic suffix used to admit equivalent application-boundary shapes.
func expressionTypeName(expression ast.Expr) string {
	switch value := expression.(type) {
	case *ast.Ident:
		return value.Name
	case *ast.StarExpr:
		return expressionTypeName(value.X)
	case *ast.SelectorExpr:
		return value.Sel.Name
	default:
		return ""
	}
}

// isApplicationBoundaryName permits established application-layer vocabulary without requiring the golden Service name.
func isApplicationBoundaryName(name string) bool {
	lower := strings.ToLower(name)
	return strings.Contains(lower, "service") || strings.Contains(lower, "query") || strings.Contains(lower, "usecase") || strings.Contains(lower, "handler")
}
