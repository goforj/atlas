package eval

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"fmt"
	"go/ast"
	"go/parser"
	"go/token"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"unicode"
	"unicode/utf8"
)

// surfaceContract describes one framework surface without turning evaluation manifests into executable policy.
type surfaceContract struct {
	id                  string
	allowedChanges      []string
	qualityTestPatterns []string
	sources             []sourceContract
	forbiddenText       []textExclusion
	commands            []commandContract
}

// textExclusion rejects one semantic leak into a protected Project surface.
type textExclusion struct {
	id    string
	paths []string
	text  string
}

// sourceContract requires syntax-bearing facts from one or more candidate-owned files.
type sourceContract struct {
	id                string
	paths             []string
	identifiers       []string
	identifierChoices [][]string
	selectorCalls     []string
	forbiddenCalls    []string
	stringLiterals    []string
	forbiddenLiterals []string
	declarations      []declarationContract
	assignments       []assignmentContract
	text              []string
}

// assignmentContract relates a named local value to the calls and identifiers used to build it.
type assignmentContract struct {
	name                 string
	identifiers          []string
	forbiddenIdentifiers []string
	selectorCalls        []string
}

// declarationContract requires related syntax to occur inside one named declaration instead of anywhere in a package.
type declarationContract struct {
	name                 string
	nameChoices          []string
	receiver             string
	identifiers          []string
	forbiddenIdentifiers []string
	selectorCalls        []string
	forbiddenCalls       []string
	stringLiterals       []string
	forbiddenLiterals    []string
	nestedCalls          []nestedCallContract
}

// nestedCallContract proves an inner call occurs within an outer call expression, including callback arguments.
type nestedCallContract struct {
	outer string
	inner string
}

// commandContract defines one supervisor-owned executable check.
type commandContract struct {
	id              string
	arguments       []string
	contains        string
	supervisorFiles []supervisorFile
	probe           func(context.Context, CommandRunner, VerifierProject) EndpointResult
}

// supervisorFile installs verifier-owned executable evidence after candidate tests have been removed.
type supervisorFile struct {
	path string
	body string
}

// surfaceVerifier verifies one promoted framework surface through syntax, ownership, and isolated commands.
type surfaceVerifier struct {
	runner   CommandRunner
	contract surfaceContract
}

// newSurfaceVerifier creates a verifier from a reviewed in-code contract.
func newSurfaceVerifier(runner CommandRunner, contract surfaceContract) *surfaceVerifier {
	return &surfaceVerifier{runner: runner, contract: contract}
}

// ID returns the promoted verifier contract identity.
func (verifier *surfaceVerifier) ID() string {
	return verifier.contract.id
}

// Capabilities returns no agent-observation requirements because checks consume sealed candidate evidence.
func (*surfaceVerifier) Capabilities() []Capability {
	return nil
}

// Verify checks semantic source facts, change ownership, and isolated executable behavior.
func (verifier *surfaceVerifier) Verify(ctx context.Context, input VerificationInput) (VerificationResult, error) {
	if verifier == nil || verifier.runner == nil {
		return VerificationResult{}, fmt.Errorf("surface verifier requires an isolated command runner")
	}
	ownedPatterns := append(append([]string(nil), verifier.contract.allowedChanges...), verifier.contract.qualityTestPatterns...)
	checks := []EndpointResult{verifySurfaceOwnership(input.Changes, ownedPatterns)}
	if len(verifier.contract.qualityTestPatterns) > 0 {
		checks = append(checks, verifyCandidateTestQuality(input.ProjectRoot, input.Changes, verifier.contract.qualityTestPatterns))
	}
	for _, contract := range verifier.contract.sources {
		checks = append(checks, verifySurfaceSource(input.ProjectRoot, contract))
	}
	for _, exclusion := range verifier.contract.forbiddenText {
		checks = append(checks, verifySurfaceTextAbsent(input.ProjectRoot, exclusion))
	}
	if surfaceChecksFailed(checks) {
		return VerificationResult{
			FrameworkOutcome:    summarizeSurfaceChecks(verifier.ID(), checks),
			WorkflowConformance: EndpointResult{ID: "workflow-owned-by-runner", Status: EndpointIneligible},
			Checks:              checks,
		}, nil
	}
	for _, contract := range verifier.contract.commands {
		if contract.probe != nil {
			checks = append(checks, contract.probe(ctx, verifier.runner, VerifierProject{Root: input.ProjectRoot, BaselineTests: input.BaselineTests}))
			continue
		}
		checks = append(checks, runIsolatedCommand(ctx, verifier.runner, VerifierProject{Root: input.ProjectRoot, BaselineTests: input.BaselineTests}, contract))
	}
	framework := summarizeSurfaceChecks(verifier.ID(), checks)
	return VerificationResult{
		FrameworkOutcome:    framework,
		WorkflowConformance: EndpointResult{ID: "workflow-owned-by-runner", Status: EndpointIneligible},
		Checks:              checks,
	}, nil
}

// surfaceChecksFailed avoids executing candidate code after sealed static evidence already proves failure.
func surfaceChecksFailed(checks []EndpointResult) bool {
	for _, check := range checks {
		if check.Kind != RequirementQuality && check.Status == EndpointFailed {
			return true
		}
	}
	return false
}

// verifyCandidateTestQuality reports whether the candidate authored focused tests without trusting those tests as outcome evidence.
func verifyCandidateTestQuality(root string, changes []ProjectChange, patterns []string) EndpointResult {
	for _, change := range changes {
		candidatePath := filepath.ToSlash(change.Path)
		if change.After.Kind == "" || !strings.HasSuffix(candidatePath, "_test.go") {
			continue
		}
		for _, pattern := range patterns {
			matched, _ := filepath.Match(pattern, candidatePath)
			if matched {
				file, err := parser.ParseFile(token.NewFileSet(), filepath.Join(root, filepath.FromSlash(candidatePath)), nil, 0)
				if err != nil {
					return EndpointResult{ID: "focused-tests-added", Kind: RequirementQuality, Status: EndpointFailed, Details: fmt.Sprintf("parse focused test %q: %v", candidatePath, err)}
				}
				for _, declaration := range file.Decls {
					function, ok := declaration.(*ast.FuncDecl)
					if ok && isGoTestFunction(file, function) {
						return EndpointResult{ID: "focused-tests-added", Kind: RequirementQuality, Status: EndpointPassed}
					}
				}
				return EndpointResult{ID: "focused-tests-added", Kind: RequirementQuality, Status: EndpointFailed, Details: fmt.Sprintf("focused test %q does not declare a Go test function", candidatePath)}
			}
		}
	}
	return EndpointResult{
		ID:      "focused-tests-added",
		Kind:    RequirementQuality,
		Status:  EndpointFailed,
		Details: "candidate did not add or update a focused test in the evaluated surface",
	}
}

// isGoTestFunction recognizes the testing package through its actual import name so helpers cannot impersonate focused tests.
func isGoTestFunction(file *ast.File, function *ast.FuncDecl) bool {
	if function.Recv != nil || !isGoTestName(function.Name.Name) || function.Type.Params == nil || len(function.Type.Params.List) != 1 || len(function.Type.Params.List[0].Names) > 1 {
		return false
	}
	testingName := ""
	for _, imported := range file.Imports {
		if imported.Path.Value != `"testing"` {
			continue
		}
		if imported.Name == nil {
			testingName = "testing"
		} else if imported.Name.Name != "." && imported.Name.Name != "_" {
			testingName = imported.Name.Name
		}
		break
	}
	pointer, ok := function.Type.Params.List[0].Type.(*ast.StarExpr)
	if !ok {
		return false
	}
	selector, ok := pointer.X.(*ast.SelectorExpr)
	if !ok || selector.Sel.Name != "T" {
		return false
	}
	packageName, ok := selector.X.(*ast.Ident)
	return ok && testingName != "" && packageName.Name == testingName
}

// isGoTestName follows the Go tool's naming boundary so ordinary helpers such as Tester are not reported as runnable tests.
func isGoTestName(name string) bool {
	if !strings.HasPrefix(name, "Test") {
		return false
	}
	suffix := name[len("Test"):]
	if suffix == "" {
		return true
	}
	next, _ := utf8.DecodeRuneInString(suffix)
	return !unicode.IsLower(next)
}

// verifySurfaceTextAbsent protects an owning App or generated boundary from cross-surface registration.
func verifySurfaceTextAbsent(root string, exclusion textExclusion) EndpointResult {
	paths, err := matchingSurfacePaths(root, exclusion.paths)
	if err != nil {
		return EndpointResult{ID: exclusion.id, Status: EndpointFailed, Details: err.Error()}
	}
	for _, path := range paths {
		if strings.HasSuffix(path, "_test.go") {
			continue
		}
		body, err := os.ReadFile(path)
		if err != nil {
			return EndpointResult{ID: exclusion.id, Status: EndpointFailed, Details: err.Error()}
		}
		contains, err := surfaceSourceContains(path, body, exclusion.text)
		if err != nil {
			return EndpointResult{ID: exclusion.id, Status: EndpointFailed, Details: err.Error()}
		}
		if contains {
			return EndpointResult{ID: exclusion.id, Status: EndpointFailed, Details: fmt.Sprintf("protected path %q contains %q", filepath.ToSlash(path), exclusion.text)}
		}
	}
	return EndpointResult{ID: exclusion.id, Status: EndpointPassed}
}

// surfaceSourceContains ignores Go comments while preserving checks against syntax and string literals.
func surfaceSourceContains(path string, body []byte, text string) (bool, error) {
	if filepath.Ext(path) != ".go" {
		return strings.Contains(string(body), text), nil
	}
	file, err := parser.ParseFile(token.NewFileSet(), path, body, 0)
	if err != nil {
		return false, fmt.Errorf("parse protected path %q: %w", filepath.ToSlash(path), err)
	}
	found := false
	ast.Inspect(file, func(node ast.Node) bool {
		if found || node == nil {
			return false
		}
		switch typed := node.(type) {
		case *ast.Ident:
			found = strings.Contains(typed.Name, text)
		case *ast.BasicLit:
			found = strings.Contains(typed.Value, text)
		}
		return !found
	})
	return found, nil
}

// verifySurfaceOwnership limits candidate changes while allowing regenerated Wire output as derived evidence.
func verifySurfaceOwnership(changes []ProjectChange, patterns []string) EndpointResult {
	var unrelated []string
	for _, change := range changes {
		path := filepath.ToSlash(change.Path)
		if derivedSurfaceProjectChange(change) {
			continue
		}
		allowed := false
		for _, pattern := range patterns {
			matched, _ := filepath.Match(pattern, path)
			ownsDescendant := (change.After.Kind == "directory" || change.Before.Kind == "directory") && strings.HasPrefix(pattern, path+"/")
			if matched || ownsDescendant || strings.HasSuffix(pattern, "/**") && strings.HasPrefix(path, strings.TrimSuffix(pattern, "**")) {
				allowed = true
				break
			}
		}
		if !allowed {
			unrelated = append(unrelated, path)
		}
	}
	if len(unrelated) > 0 {
		return EndpointResult{ID: "change-ownership", Status: EndpointFailed, Details: fmt.Sprintf("candidate changed unrelated paths: %s", strings.Join(unrelated, ", "))}
	}
	return EndpointResult{ID: "change-ownership", Status: EndpointPassed}
}

// derivedSurfaceProjectChange accepts empty runtime storage directories without exempting source files in a package named storage.
func derivedSurfaceProjectChange(change ProjectChange) bool {
	path := filepath.ToSlash(change.Path)
	if derivedSurfaceChange(path) {
		return true
	}
	kind := change.After.Kind
	if kind == "" {
		kind = change.Before.Kind
	}
	return kind == "directory" && (strings.Contains(path, "/storage/") || strings.HasSuffix(path, "/storage"))
}

// derivedSurfaceChange identifies framework and Go tool outputs that are verified through isolated commands rather than authored ownership.
func derivedSurfaceChange(path string) bool {
	if path == "go.sum" || filepath.Base(path) == "wire_gen.go" {
		return true
	}
	if path == "_data" || strings.Contains(path, "/_data/") || strings.HasSuffix(path, "/_data") {
		return true
	}
	return path == "bin" || path == "build" || path == "storage" ||
		strings.HasPrefix(path, "bin/") || strings.HasPrefix(path, "build/") || strings.HasPrefix(path, "storage/")
}

// verifySurfaceSource finds syntax-bearing facts without counting comments as implementation evidence.
func verifySurfaceSource(root string, contract sourceContract) EndpointResult {
	paths, err := matchingSurfacePaths(root, contract.paths)
	if err != nil {
		return EndpointResult{ID: contract.id, Status: EndpointFailed, Details: err.Error()}
	}
	facts := newSourceFacts()
	declarations := map[string]*sourceFacts{}
	assignments := map[string]*sourceFacts{}
	var text strings.Builder
	for _, path := range paths {
		if strings.HasSuffix(path, "_test.go") {
			continue
		}
		body, err := os.ReadFile(path)
		if err != nil {
			return EndpointResult{ID: contract.id, Status: EndpointFailed, Details: err.Error()}
		}
		text.Write(body)
		if filepath.Ext(path) != ".go" {
			continue
		}
		file, err := parser.ParseFile(token.NewFileSet(), path, body, 0)
		if err != nil {
			return EndpointResult{ID: contract.id, Status: EndpointFailed, Details: fmt.Sprintf("parse %s: %v", filepath.ToSlash(path), err)}
		}
		collectSourceFacts(file, &facts, declarations, assignments)
	}
	if details := verifySourceFacts(facts, contract.identifiers, contract.identifierChoices, contract.selectorCalls, contract.forbiddenCalls, contract.stringLiterals, contract.forbiddenLiterals); details != "" {
		return EndpointResult{ID: contract.id, Status: EndpointFailed, Details: details}
	}
	for _, declaration := range contract.declarations {
		names := append([]string(nil), declaration.nameChoices...)
		if declaration.name != "" {
			names = append([]string{declaration.name}, names...)
		}
		key := ""
		var scope *sourceFacts
		for _, name := range names {
			candidate := name
			if declaration.receiver != "" {
				candidate = declaration.receiver + "." + name
			}
			if declarations[candidate] != nil {
				key = candidate
				scope = declarations[candidate]
				break
			}
		}
		if scope == nil {
			return EndpointResult{ID: contract.id, Status: EndpointFailed, Details: fmt.Sprintf("required declaration %q is absent", names)}
		}
		if details := verifySourceFacts(*scope, declaration.identifiers, nil, declaration.selectorCalls, declaration.forbiddenCalls, declaration.stringLiterals, declaration.forbiddenLiterals); details != "" {
			return EndpointResult{ID: contract.id, Status: EndpointFailed, Details: fmt.Sprintf("declaration %q: %s", key, details)}
		}
		for _, identifier := range declaration.forbiddenIdentifiers {
			if scope.identifiers[identifier] {
				return EndpointResult{ID: contract.id, Status: EndpointFailed, Details: fmt.Sprintf("declaration %q contains forbidden identifier %q", key, identifier)}
			}
		}
		for _, nested := range declaration.nestedCalls {
			if !scope.nestedCalls[nested.outer+">"+nested.inner] {
				return EndpointResult{ID: contract.id, Status: EndpointFailed, Details: fmt.Sprintf("declaration %q does not call %q within %q", key, nested.inner, nested.outer)}
			}
		}
	}
	for _, assignment := range contract.assignments {
		scope := assignments[assignment.name]
		if scope == nil {
			return EndpointResult{ID: contract.id, Status: EndpointFailed, Details: fmt.Sprintf("required assignment %q is absent", assignment.name)}
		}
		if details := verifySourceFacts(*scope, assignment.identifiers, nil, assignment.selectorCalls, nil, nil, nil); details != "" {
			return EndpointResult{ID: contract.id, Status: EndpointFailed, Details: fmt.Sprintf("assignment %q: %s", assignment.name, details)}
		}
		for _, identifier := range assignment.forbiddenIdentifiers {
			if scope.identifiers[identifier] {
				return EndpointResult{ID: contract.id, Status: EndpointFailed, Details: fmt.Sprintf("assignment %q contains forbidden identifier %q", assignment.name, identifier)}
			}
		}
	}
	for _, required := range contract.text {
		if !strings.Contains(text.String(), required) {
			return EndpointResult{ID: contract.id, Status: EndpointFailed, Details: fmt.Sprintf("required configuration %q is absent", required)}
		}
	}
	return EndpointResult{ID: contract.id, Status: EndpointPassed}
}

// sourceFacts retains the syntax classes used by promoted surface contracts.
type sourceFacts struct {
	identifiers    map[string]bool
	selectorCalls  map[string]bool
	stringLiterals map[string]bool
	nestedCalls    map[string]bool
}

// newSourceFacts creates one isolated syntax evidence set.
func newSourceFacts() sourceFacts {
	return sourceFacts{
		identifiers:    map[string]bool{},
		selectorCalls:  map[string]bool{},
		stringLiterals: map[string]bool{},
		nestedCalls:    map[string]bool{},
	}
}

// verifySourceFacts applies shared syntax requirements without weakening declaration-scoped checks into file-wide evidence.
func verifySourceFacts(facts sourceFacts, identifiers []string, identifierChoices [][]string, selectorCalls, forbiddenCalls, stringLiterals, forbiddenLiterals []string) string {
	for _, identifier := range identifiers {
		if !facts.identifiers[identifier] {
			return fmt.Sprintf("required identifier %q is absent", identifier)
		}
	}
	for _, choices := range identifierChoices {
		matched := false
		for _, identifier := range choices {
			if facts.identifiers[identifier] {
				matched = true
				break
			}
		}
		if !matched {
			return fmt.Sprintf("one of the required identifiers %q is absent", choices)
		}
	}
	for _, selector := range selectorCalls {
		if !facts.selectorCalls[selector] {
			return fmt.Sprintf("required call %q is absent", selector)
		}
	}
	for _, selector := range forbiddenCalls {
		if facts.selectorCalls[selector] {
			return fmt.Sprintf("forbidden call %q is present", selector)
		}
	}
	for _, literal := range stringLiterals {
		if !facts.stringLiterals[literal] {
			return fmt.Sprintf("required string literal %q is absent", literal)
		}
	}
	for _, literal := range forbiddenLiterals {
		if facts.stringLiterals[literal] {
			return fmt.Sprintf("forbidden string literal %q is present", literal)
		}
	}
	return ""
}

// collectSourceFacts records package-wide and named-declaration evidence while excluding comments.
func collectSourceFacts(file *ast.File, facts *sourceFacts, declarations, assignments map[string]*sourceFacts) {
	collectNodeFacts(file, facts)
	for _, declaration := range file.Decls {
		switch value := declaration.(type) {
		case *ast.FuncDecl:
			collectDeclarationFacts(value.Name.Name, value, declarations)
			if receiver := functionReceiverName(value); receiver != "" {
				collectDeclarationFacts(receiver+"."+value.Name.Name, value, declarations)
			}
		case *ast.GenDecl:
			for _, spec := range value.Specs {
				if typeSpec, ok := spec.(*ast.TypeSpec); ok {
					collectDeclarationFacts(typeSpec.Name.Name, typeSpec, declarations)
				}
			}
		}
	}
	ast.Inspect(file, func(node ast.Node) bool {
		assignment, ok := node.(*ast.AssignStmt)
		if !ok {
			return true
		}
		for index, expression := range assignment.Lhs {
			identifier, ok := expression.(*ast.Ident)
			if !ok || index >= len(assignment.Rhs) {
				continue
			}
			facts := assignments[identifier.Name]
			if facts == nil {
				value := newSourceFacts()
				facts = &value
				assignments[identifier.Name] = facts
			}
			collectNodeFacts(assignment.Rhs[index], facts)
		}
		return true
	})
}

// functionReceiverName identifies a method owner without coupling contracts to pointer or value syntax.
func functionReceiverName(declaration *ast.FuncDecl) string {
	if declaration == nil || declaration.Recv == nil || len(declaration.Recv.List) != 1 {
		return ""
	}
	expression := declaration.Recv.List[0].Type
	if pointer, ok := expression.(*ast.StarExpr); ok {
		expression = pointer.X
	}
	identifier, _ := expression.(*ast.Ident)
	if identifier == nil {
		return ""
	}
	return identifier.Name
}

// collectDeclarationFacts merges declarations with the same name so reviewed method families remain supported.
func collectDeclarationFacts(name string, node ast.Node, declarations map[string]*sourceFacts) {
	facts := declarations[name]
	if facts == nil {
		value := newSourceFacts()
		facts = &value
		declarations[name] = facts
	}
	collectNodeFacts(node, facts)
}

// collectNodeFacts records identifiers, calls, literals, and call containment for one syntax boundary.
func collectNodeFacts(node ast.Node, facts *sourceFacts) {
	ast.Inspect(node, func(node ast.Node) bool {
		switch value := node.(type) {
		case *ast.Ident:
			facts.identifiers[value.Name] = true
		case *ast.CallExpr:
			outer := callName(value)
			if outer != "" {
				facts.selectorCalls[outer] = true
				for _, argument := range value.Args {
					ast.Inspect(argument, func(inner ast.Node) bool {
						call, ok := inner.(*ast.CallExpr)
						if ok && call != value {
							if innerName := callName(call); innerName != "" {
								facts.nestedCalls[outer+">"+innerName] = true
							}
						}
						return true
					})
				}
			}
		case *ast.BasicLit:
			if value.Kind == token.STRING {
				facts.stringLiterals[strings.Trim(value.Value, "`\"")] = true
			}
		}
		return true
	})
}

// callName returns the stable terminal name used by reviewed contracts across package and receiver choices.
func callName(call *ast.CallExpr) string {
	return expressionCallName(call.Fun)
}

// expressionCallName unwraps generic instantiation so calls such as cache.Get[User] retain the same semantic name as ordinary calls.
func expressionCallName(expression ast.Expr) string {
	switch value := expression.(type) {
	case *ast.Ident:
		return value.Name
	case *ast.SelectorExpr:
		return value.Sel.Name
	case *ast.IndexExpr:
		return expressionCallName(value.X)
	case *ast.IndexListExpr:
		return expressionCallName(value.X)
	default:
		return ""
	}
}

// matchingSurfacePaths resolves reviewed Project-relative globs without following arbitrary external paths.
func matchingSurfacePaths(root string, patterns []string) ([]string, error) {
	set := map[string]bool{}
	for _, pattern := range patterns {
		matches, err := filepath.Glob(filepath.Join(root, filepath.FromSlash(pattern)))
		if err != nil {
			return nil, err
		}
		for _, match := range matches {
			set[match] = true
		}
	}
	paths := make([]string, 0, len(set))
	for path := range set {
		paths = append(paths, path)
	}
	sort.Strings(paths)
	if len(paths) == 0 {
		return nil, fmt.Errorf("none of the required paths exist: %s", strings.Join(patterns, ", "))
	}
	return paths, nil
}

// runIsolatedCommand executes one check in a private clone and always destroys its writable state.
func runIsolatedCommand(ctx context.Context, runner CommandRunner, project VerifierProject, contract commandContract) (result EndpointResult) {
	session, err := runner.Open(ctx, project)
	if err != nil {
		return EndpointResult{ID: contract.id, Status: EndpointFailed, Details: fmt.Sprintf("open isolated verifier session: %v", err)}
	}
	defer func() {
		cleanupContext, cancel := context.WithTimeout(context.Background(), verifierCleanupTimeout)
		defer cancel()
		if closeErr := session.Close(cleanupContext); closeErr != nil && result.Status == EndpointPassed {
			result = EndpointResult{ID: contract.id, Status: EndpointFailed, Details: fmt.Sprintf("close isolated verifier session: %v", closeErr)}
		}
	}()
	for _, file := range contract.supervisorFiles {
		if err := session.WriteFile(file.path, []byte(file.body)); err != nil {
			return EndpointResult{ID: contract.id, Status: EndpointFailed, Details: fmt.Sprintf("install supervisor probe: %v", err)}
		}
	}
	arguments := append([]string(nil), contract.arguments...)
	contains := contract.contains
	if len(contract.supervisorFiles) > 0 {
		arguments, contains, err = installSupervisorCompletionMarker(session, contract.supervisorFiles[0], arguments)
		if err != nil {
			return EndpointResult{ID: contract.id, Status: EndpointFailed, Details: fmt.Sprintf("install supervisor completion marker: %v", err)}
		}
	}
	return runCheck(ctx, session, contract.id, arguments, contains)
}

// installSupervisorCompletionMarker rejects bare successful exits; adversarial anti-forgery remains the authoritative backend's responsibility.
func installSupervisorCompletionMarker(session CommandSession, source supervisorFile, arguments []string) ([]string, string, error) {
	file, err := parser.ParseFile(token.NewFileSet(), source.path, source.body, parser.PackageClauseOnly)
	if err != nil {
		return nil, "", fmt.Errorf("parse supervisor probe package: %w", err)
	}
	random := make([]byte, 16)
	if _, err := rand.Read(random); err != nil {
		return nil, "", fmt.Errorf("generate completion nonce: %w", err)
	}
	nonce := hex.EncodeToString(random)
	marker := "ATLAS_SUPERVISOR_COMPLETION_" + nonce
	functionName := "TestAtlasSupervisorCompletionMarker" + nonce
	body := fmt.Sprintf("package %s\n\nimport (\n\t\"fmt\"\n\t\"testing\"\n)\n\n// %s proves the supervisor-owned test body executed.\nfunc %s(t *testing.T) {\n\tfmt.Println(%q)\n}\n", file.Name.Name, functionName, functionName, marker)
	markerPath := filepath.Join(filepath.Dir(source.path), "atlas_eval_completion_marker_"+nonce+"_test.go")
	if err := session.WriteFile(markerPath, []byte(body)); err != nil {
		return nil, "", err
	}
	arguments = append([]string(nil), arguments...)
	foundRun := false
	for index := 0; index+1 < len(arguments); index++ {
		if arguments[index] == "-run" {
			original := strings.Trim(arguments[index+1], "^$")
			arguments[index+1] = "^(" + original + "|" + functionName + ")$"
			foundRun = true
			break
		}
	}
	if !foundRun {
		arguments = append(arguments, "-run", "^"+functionName+"$")
	}
	arguments = append(arguments, "-v")
	return arguments, marker, nil
}

// summarizeSurfaceChecks produces one endpoint without concealing individual failures.
func summarizeSurfaceChecks(id string, checks []EndpointResult) EndpointResult {
	failed := 0
	ineligible := 0
	for _, check := range checks {
		if check.Kind == RequirementQuality {
			continue
		}
		switch check.Status {
		case EndpointFailed:
			failed++
		case EndpointIneligible:
			ineligible++
		}
	}
	if failed > 0 {
		return EndpointResult{ID: id, Status: EndpointFailed, Details: fmt.Sprintf("%d of %d framework checks failed", failed, len(checks))}
	}
	if ineligible > 0 {
		return EndpointResult{ID: id, Status: EndpointIneligible, Details: fmt.Sprintf("%d of %d framework checks lack authoritative evidence", ineligible, len(checks))}
	}
	return EndpointResult{ID: id, Status: EndpointPassed}
}
