package eval

import (
	"bytes"
	"context"
	"fmt"
	"go/ast"
	"go/format"
	"go/parser"
	"go/token"
	"os"
	"path/filepath"
	"sort"
	"strconv"
	"strings"
	"text/template"
)

// cacheBehaviorProbe identifies the candidate's cache decorator without prescribing its domain names.
type cacheBehaviorProbe struct {
	relativeDirectory string
	packageName       string
	constructor       string
	sourceArgument    int
	cacheArgument     int
	cachedType        string
	sourceField       string
	cacheField        string
}

// cacheBehaviorTemplateData projects the discovered constructor into the trusted cache oracle.
type cacheBehaviorTemplateData struct {
	PackageName        string
	MissExpression     string
	FailureExpression  string
	CanceledExpression string
}

const cacheBehaviorProbeTemplate = `package {{.PackageName}}

import (
	"context"
	"errors"
	"testing"

	"github.com/goforj/cache"
)

// atlasCacheSource makes cache misses observable without relying on a candidate-owned source implementation.
type atlasCacheSource struct {
	calls int
	err   error
}

// Find supplies a stable successful value while preserving cancellation at the authoritative boundary.
func (source *atlasCacheSource) Find(ctx context.Context, id string) (User, error) {
	source.calls++
	if err := ctx.Err(); err != nil {
		return User{}, err
	}
	if source.err != nil {
		return User{}, source.err
	}
	return User{ID: id}, nil
}

// TestAtlasCacheAsideBehavior proves an initial miss is cached, subsequent reads hit the cache, and cancellation is propagated.
func TestAtlasCacheAsideBehavior(t *testing.T) {
	ctx := context.Background()
	profileCache := cache.NewCache(cache.NewMemoryStore(ctx))
	source := &atlasCacheSource{}
	repository := {{.MissExpression}}
	first, err := repository.Find(ctx, "42")
	if err != nil {
		t.Fatalf("first Find(): %v", err)
	}
	if first.ID != "42" {
		t.Fatalf("first user ID = %q, want 42", first.ID)
	}
	if source.calls != 1 {
		t.Fatalf("source calls after miss = %d, want 1", source.calls)
	}
	secondRepository := {{.MissExpression}}
	second, err := secondRepository.Find(ctx, "42")
	if err != nil {
		t.Fatalf("second Find(): %v", err)
	}
	if second.ID != "42" {
		t.Fatalf("cached user ID = %q, want 42", second.ID)
	}
	if source.calls != 1 {
		t.Fatalf("source calls after cache hit = %d, want 1", source.calls)
	}
	third, err := secondRepository.Find(ctx, "43")
	if err != nil {
		t.Fatalf("third Find(): %v", err)
	}
	if third.ID != "43" {
		t.Fatalf("second key user ID = %q, want 43", third.ID)
	}
	if source.calls != 2 {
		t.Fatalf("source calls after second key miss = %d, want 2", source.calls)
	}
	thirdRepository := {{.MissExpression}}
	fourth, err := thirdRepository.Find(ctx, "43")
	if err != nil {
		t.Fatalf("fourth Find(): %v", err)
	}
	if fourth.ID != "43" {
		t.Fatalf("second key cached user ID = %q, want 43", fourth.ID)
	}
	if source.calls != 2 {
		t.Fatalf("source calls after second key hit = %d, want 2", source.calls)
	}

	sourceFailure := errors.New("source unavailable")
	failingSource := &atlasCacheSource{err: sourceFailure}
	failureCache := cache.NewCache(cache.NewMemoryStore(ctx))
	failingRepository := {{.FailureExpression}}
	if _, err := failingRepository.Find(ctx, "44"); !errors.Is(err, sourceFailure) {
		t.Fatalf("source failure = %v, want %v", err, sourceFailure)
	}
	failingSource.err = nil
	recovered, err := failingRepository.Find(ctx, "44")
	if err != nil {
		t.Fatalf("Find() after source recovery: %v", err)
	}
	if recovered.ID != "44" {
		t.Fatalf("recovered user ID = %q, want 44", recovered.ID)
	}
	if failingSource.calls != 2 {
		t.Fatalf("source calls across failed and recovered lookups = %d, want 2", failingSource.calls)
	}

	canceledContext, cancel := context.WithCancel(ctx)
	cancel()
	canceledSource := &atlasCacheSource{}
	canceledRepository := {{.CanceledExpression}}
	if _, err := canceledRepository.Find(canceledContext, "42"); !errors.Is(err, context.Canceled) {
		t.Fatalf("Find() error = %v, want context cancellation", err)
	}
}
`

// runCacheBehaviorProbe installs a generated black-box cache oracle after candidate tests have been removed.
func runCacheBehaviorProbe(ctx context.Context, runner CommandRunner, project VerifierProject) (result EndpointResult) {
	probe, err := resolveCacheBehaviorProbe(project.Root)
	if err != nil {
		return EndpointResult{ID: "cache-aside-behavior", Status: EndpointFailed, Details: err.Error()}
	}
	if err := verifyCacheDecoratorRegistration(project.Root, probe.constructor); err != nil {
		return EndpointResult{ID: "cache-aside-behavior", Status: EndpointFailed, Details: err.Error()}
	}
	body, err := renderCacheBehaviorProbe(probe)
	if err != nil {
		return EndpointResult{ID: "cache-aside-behavior", Status: EndpointFailed, Details: err.Error()}
	}
	session, err := runner.Open(ctx, project)
	if err != nil {
		return EndpointResult{ID: "cache-aside-behavior", Status: EndpointFailed, Details: fmt.Sprintf("open isolated verifier session: %v", err)}
	}
	defer func() {
		cleanupContext, cancel := context.WithTimeout(context.Background(), verifierCleanupTimeout)
		defer cancel()
		if closeErr := session.Close(cleanupContext); closeErr != nil && result.Status == EndpointPassed {
			result = EndpointResult{ID: "cache-aside-behavior", Status: EndpointFailed, Details: fmt.Sprintf("close isolated verifier session: %v", closeErr)}
		}
	}()
	relativePath := filepath.Join(probe.relativeDirectory, "atlas_eval_cache_aside_test.go")
	if err := session.WriteFile(relativePath, body); err != nil {
		return EndpointResult{ID: "cache-aside-behavior", Status: EndpointFailed, Details: err.Error()}
	}
	command := []string{"go", "test", "./" + filepath.ToSlash(probe.relativeDirectory), "-run", "^TestAtlasCacheAsideBehavior$", "-count=1"}
	command, marker, err := installSupervisorCompletionMarker(session, supervisorFile{path: relativePath, body: string(body)}, command)
	if err != nil {
		return EndpointResult{ID: "cache-aside-behavior", Status: EndpointFailed, Details: err.Error()}
	}
	output, err := session.Run(ctx, command)
	return cacheBehaviorExecutionResult(output, marker, err)
}

// cacheBehaviorExecutionResult accepts success only after the supervisor-owned test reaches its marker.
func cacheBehaviorExecutionResult(output, marker string, err error) EndpointResult {
	if err != nil {
		return EndpointResult{ID: "cache-aside-behavior", Status: EndpointFailed, Details: err.Error()}
	}
	if marker == "" || !strings.Contains(output, marker) {
		return EndpointResult{ID: "cache-aside-behavior", Status: EndpointFailed, Details: "supervisor completion marker was not observed"}
	}
	return EndpointResult{ID: "cache-aside-behavior", Status: EndpointPassed}
}

// resolveCacheBehaviorProbe finds a conventional two-dependency cache decorator in the users package.
func resolveCacheBehaviorProbe(root string) (cacheBehaviorProbe, error) {
	directory := filepath.Join(root, "internal", "users")
	packages, err := parser.ParseDir(token.NewFileSet(), directory, func(info os.FileInfo) bool {
		return !strings.HasSuffix(info.Name(), "_test.go")
	}, 0)
	if err != nil {
		return cacheBehaviorProbe{}, fmt.Errorf("parse users package: %w", err)
	}
	packageNames := make([]string, 0, len(packages))
	for packageName := range packages {
		packageNames = append(packageNames, packageName)
	}
	sort.Strings(packageNames)
	for _, packageName := range packageNames {
		parsedPackage := packages[packageName]
		if probe, ok := cacheDecoratorStruct(parsedPackage); ok {
			probe.constructor = cacheDecoratorConstructor(parsedPackage, probe.cachedType)
			if probe.constructor == "" {
				continue
			}
			probe.relativeDirectory = "internal/users"
			probe.packageName = packageName
			return probe, nil
		}
		for _, file := range sortedPackageFiles(parsedPackage) {
			for _, declaration := range file.Decls {
				function, ok := declaration.(*ast.FuncDecl)
				if !ok || function.Recv != nil || !strings.HasPrefix(function.Name.Name, "New") || function.Type.Params == nil || len(function.Type.Params.List) != 2 {
					continue
				}
				sourceArgument, cacheArgument, ok := cacheConstructorArguments(function)
				if !ok {
					continue
				}
				return cacheBehaviorProbe{relativeDirectory: "internal/users", packageName: packageName, constructor: function.Name.Name, sourceArgument: sourceArgument, cacheArgument: cacheArgument}, nil
			}
		}
	}
	return cacheBehaviorProbe{}, fmt.Errorf("a two-dependency cache repository constructor could not be derived")
}

// verifyCacheDecoratorRegistration requires App Wire to register the decorator directly or through
// a local provider that constructs it, preserving the production dependency relationship.
func verifyCacheDecoratorRegistration(root, constructor string) error {
	path := filepath.Join(root, "app", "wire", "inject_services_app.go")
	file, err := parser.ParseFile(token.NewFileSet(), path, nil, 0)
	if err != nil {
		return fmt.Errorf("parse cache repository registration: %w", err)
	}
	registeredProviders := map[string]bool{}
	registeredRepositoryProviders := map[string]bool{}
	userAliases := importedPackageAliases(file, "/internal/users")
	directConstructor := false
	ast.Inspect(file, func(node ast.Node) bool {
		call, ok := node.(*ast.CallExpr)
		if !ok || !isWireNewSet(call) {
			return true
		}
		for _, argument := range call.Args {
			switch expression := argument.(type) {
			case *ast.SelectorExpr:
				if expression.Sel.Name == constructor {
					directConstructor = true
				}
				if qualifier, ok := expression.X.(*ast.Ident); ok && userAliases[qualifier.Name] {
					registeredRepositoryProviders[expression.Sel.Name] = true
				}
			case *ast.Ident:
				registeredProviders[expression.Name] = true
			}
		}
		return true
	})
	if directConstructor {
		return nil
	}
	for _, declaration := range file.Decls {
		function, ok := declaration.(*ast.FuncDecl)
		if !ok || function.Recv != nil || !registeredProviders[function.Name.Name] || function.Body == nil {
			continue
		}
		constructsDecorator := false
		ast.Inspect(function.Body, func(node ast.Node) bool {
			selector, ok := node.(*ast.SelectorExpr)
			if ok && selector.Sel.Name == constructor {
				constructsDecorator = true
				return false
			}
			return !constructsDecorator
		})
		if constructsDecorator {
			return nil
		}
	}
	usersDirectory := filepath.Join(root, "internal", "users")
	usersPackages, err := parser.ParseDir(token.NewFileSet(), usersDirectory, func(info os.FileInfo) bool {
		return !strings.HasSuffix(info.Name(), "_test.go")
	}, 0)
	if err == nil {
		for _, parsedPackage := range usersPackages {
			for provider := range registeredRepositoryProviders {
				if packageFunctionCalls(parsedPackage, provider, constructor) {
					return nil
				}
			}
		}
	}
	if !directConstructor {
		return fmt.Errorf("cache repository constructor %q is not registered in app/wire/inject_services_app.go", constructor)
	}
	return nil
}

// importedPackageAliases returns the local names that refer to one application package.
func importedPackageAliases(file *ast.File, suffix string) map[string]bool {
	aliases := map[string]bool{}
	for _, specification := range file.Imports {
		path, err := strconv.Unquote(specification.Path.Value)
		if err != nil || !strings.HasSuffix(path, suffix) {
			continue
		}
		name := filepath.Base(path)
		if specification.Name != nil && specification.Name.Name != "_" && specification.Name.Name != "." {
			name = specification.Name.Name
		}
		aliases[name] = true
	}
	return aliases
}

// packageFunctionCalls follows a registered package constructor through one explicit composition wrapper.
func packageFunctionCalls(parsedPackage *ast.Package, functionName, constructor string) bool {
	for _, file := range sortedPackageFiles(parsedPackage) {
		for _, declaration := range file.Decls {
			function, ok := declaration.(*ast.FuncDecl)
			if !ok || function.Recv != nil || function.Name.Name != functionName || function.Body == nil {
				continue
			}
			called := false
			ast.Inspect(function.Body, func(node ast.Node) bool {
				call, ok := node.(*ast.CallExpr)
				if !ok {
					return true
				}
				identifier, identified := call.Fun.(*ast.Ident)
				if identified && identifier.Name == constructor {
					called = true
					return false
				}
				return !called
			})
			if called {
				return true
			}
		}
	}
	return false
}

// cacheDecoratorStruct discovers the conventional repository and cache fields used by a cache-aside decorator.
func cacheDecoratorStruct(parsedPackage *ast.Package) (cacheBehaviorProbe, bool) {
	interfaces := packageInterfaceNames(parsedPackage)
	findReceivers := make(map[string]bool)
	for _, file := range sortedPackageFiles(parsedPackage) {
		for _, declaration := range file.Decls {
			function, ok := declaration.(*ast.FuncDecl)
			if ok && function.Name.Name == "Find" {
				findReceivers[functionReceiverName(function)] = true
			}
		}
	}
	for _, file := range sortedPackageFiles(parsedPackage) {
		for _, declaration := range file.Decls {
			general, ok := declaration.(*ast.GenDecl)
			if !ok {
				continue
			}
			for _, specification := range general.Specs {
				typeSpec, ok := specification.(*ast.TypeSpec)
				if !ok || !findReceivers[typeSpec.Name.Name] {
					continue
				}
				structure, ok := typeSpec.Type.(*ast.StructType)
				if !ok {
					continue
				}
				sourceField, cacheField := cacheDecoratorFields(structure, interfaces)
				if sourceField != "" && cacheField != "" {
					return cacheBehaviorProbe{cachedType: typeSpec.Name.Name, sourceField: sourceField, cacheField: cacheField}, true
				}
			}
		}
	}
	return cacheBehaviorProbe{}, false
}

// cacheDecoratorConstructor finds the constructor that returns the discovered decorator implementation.
func cacheDecoratorConstructor(parsedPackage *ast.Package, cachedType string) string {
	for _, file := range sortedPackageFiles(parsedPackage) {
		for _, declaration := range file.Decls {
			function, ok := declaration.(*ast.FuncDecl)
			if !ok || function.Recv != nil || function.Body == nil {
				continue
			}
			constructsType := false
			ast.Inspect(function.Body, func(node ast.Node) bool {
				literal, ok := node.(*ast.CompositeLit)
				identifier, identified := literalTypeIdentifier(literal)
				if ok && identified && identifier == cachedType {
					constructsType = true
				}
				return !constructsType
			})
			if constructsType {
				return function.Name.Name
			}
		}
	}
	return ""
}

// literalTypeIdentifier unwraps one composite literal type without relying on formatting.
func literalTypeIdentifier(literal *ast.CompositeLit) (string, bool) {
	if literal == nil {
		return "", false
	}
	identifier, ok := literal.Type.(*ast.Ident)
	if !ok {
		return "", false
	}
	return identifier.Name, true
}

// packageInterfaceNames records the repository contracts that an oracle source can safely implement.
func packageInterfaceNames(parsedPackage *ast.Package) map[string]bool {
	interfaces := make(map[string]bool)
	for _, file := range sortedPackageFiles(parsedPackage) {
		for _, declaration := range file.Decls {
			general, ok := declaration.(*ast.GenDecl)
			if !ok {
				continue
			}
			for _, specification := range general.Specs {
				typeSpec, ok := specification.(*ast.TypeSpec)
				if ok {
					if _, ok := typeSpec.Type.(*ast.InterfaceType); ok {
						interfaces[typeSpec.Name.Name] = true
					}
				}
			}
		}
	}
	return interfaces
}

// sortedPackageFiles keeps probe discovery stable when Go map iteration order changes.
func sortedPackageFiles(parsedPackage *ast.Package) []*ast.File {
	paths := make([]string, 0, len(parsedPackage.Files))
	for path := range parsedPackage.Files {
		paths = append(paths, path)
	}
	sort.Strings(paths)
	files := make([]*ast.File, 0, len(paths))
	for _, path := range paths {
		files = append(files, parsedPackage.Files[path])
	}
	return files
}

// cacheDecoratorFields identifies the source and cache dependencies from their field types.
func cacheDecoratorFields(structure *ast.StructType, interfaces map[string]bool) (string, string) {
	sourceField := ""
	cacheField := ""
	for _, field := range structure.Fields.List {
		if len(field.Names) != 1 {
			continue
		}
		typeName := strings.ToLower(expressionTypeName(field.Type))
		switch {
		case strings.Contains(typeName, "cache"):
			cacheField = field.Names[0].Name
		case strings.Contains(typeName, "repository") && interfaces[expressionTypeName(field.Type)]:
			sourceField = field.Names[0].Name
		}
	}
	return sourceField, cacheField
}

// cacheConstructorArguments identifies the source and cache parameters by their repository and cache conventions.
func cacheConstructorArguments(function *ast.FuncDecl) (int, int, bool) {
	if function.Type.Params == nil || len(function.Type.Params.List) != 2 {
		return 0, 0, false
	}
	cacheArgument := -1
	for index, field := range function.Type.Params.List {
		if strings.Contains(strings.ToLower(expressionTypeName(field.Type)), "cache") {
			cacheArgument = index
		}
	}
	if cacheArgument < 0 {
		return 0, 0, false
	}
	sourceArgument := 1 - cacheArgument
	if !strings.Contains(strings.ToLower(expressionTypeName(function.Type.Params.List[sourceArgument].Type)), "repository") {
		return 0, 0, false
	}
	return sourceArgument, cacheArgument, true
}

// renderCacheBehaviorProbe creates a same-package probe so unexported repository constructors remain testable.
func renderCacheBehaviorProbe(probe cacheBehaviorProbe) ([]byte, error) {
	templateBody, err := template.New("cache behavior probe").Parse(cacheBehaviorProbeTemplate)
	if err != nil {
		return nil, fmt.Errorf("parse cache behavior probe: %w", err)
	}
	var source bytes.Buffer
	missExpression, err := cacheRepositoryExpression(probe, "source", "profileCache")
	if err != nil {
		return nil, err
	}
	failureExpression, err := cacheRepositoryExpression(probe, "failingSource", "failureCache")
	if err != nil {
		return nil, err
	}
	canceledExpression, err := cacheRepositoryExpression(probe, "canceledSource", "cache.NewCache(cache.NewMemoryStore(ctx))")
	if err != nil {
		return nil, err
	}
	if err := templateBody.Execute(&source, cacheBehaviorTemplateData{PackageName: probe.packageName, MissExpression: missExpression, FailureExpression: failureExpression, CanceledExpression: canceledExpression}); err != nil {
		return nil, fmt.Errorf("execute cache behavior probe: %w", err)
	}
	formatted, err := format.Source(source.Bytes())
	if err != nil {
		return nil, fmt.Errorf("format cache behavior probe: %w", err)
	}
	return formatted, nil
}

// cacheRepositoryExpression renders either a discovered decorator literal or its two-dependency constructor call.
func cacheRepositoryExpression(probe cacheBehaviorProbe, source, cache string) (string, error) {
	if probe.cachedType != "" {
		return fmt.Sprintf("&%s{%s: %s, %s: %s}", probe.cachedType, probe.sourceField, source, probe.cacheField, cache), nil
	}
	if probe.constructor == "" {
		return "", fmt.Errorf("cache repository construction could not be derived")
	}
	if probe.sourceArgument == 0 && probe.cacheArgument == 1 {
		return fmt.Sprintf("%s(%s, %s)", probe.constructor, source, cache), nil
	}
	return fmt.Sprintf("%s(%s, %s)", probe.constructor, cache, source), nil
}
