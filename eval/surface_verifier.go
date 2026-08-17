package eval

import (
	"context"
	"crypto/rand"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"go/ast"
	"go/parser"
	"go/token"
	"os"
	"path/filepath"
	"slices"
	"sort"
	"strings"
	"unicode"
	"unicode/utf8"
)

const maxWireOutputFiles = 64

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
	compactText       []string
	commentOnly       bool
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
	argumentFlows        []callArgumentFlowContract
}

// nestedCallContract proves an inner call occurs within an outer call expression, including callback arguments.
type nestedCallContract struct {
	outer string
	inner string
}

// callArgumentFlowContract requires a call argument to originate from an accessor call containing a configured literal.
type callArgumentFlowContract struct {
	call    string
	literal string
}

// commandContract defines one supervisor-owned executable check.
type commandContract struct {
	id              string
	arguments       []string
	contains        string
	supervisorFiles []supervisorFile
	probe           func(context.Context, CommandRunner, VerifierProject) EndpointResult
	standard        bool
	standardBuilds  [][]string
}

// readableCommandSession exposes candidate-derived files from a verifier-owned clone without expanding the public command-session contract.
type readableCommandSession interface {
	ReadFile(string) ([]byte, error)
	FilesNamed(string) ([]string, error)
}

// verifyWireOutputParity proves checked-in Wire output is the result of the supported full build path.
func verifyWireOutputParity() func(context.Context, CommandRunner, VerifierProject) EndpointResult {
	return func(ctx context.Context, runner CommandRunner, project VerifierProject) (result EndpointResult) {
		session, err := runner.Open(ctx, project)
		if err != nil {
			return EndpointResult{ID: "wire-output-parity", Status: EndpointFailed, Details: fmt.Sprintf("open isolated verifier session: %v", err)}
		}
		defer func() {
			cleanupContext, cancel := context.WithTimeout(context.Background(), verifierCleanupTimeout)
			defer cancel()
			if closeErr := session.Close(cleanupContext); closeErr != nil && result.Status == EndpointPassed {
				result = EndpointResult{ID: "wire-output-parity", Status: EndpointFailed, Details: fmt.Sprintf("close isolated verifier session: %v", closeErr)}
			}
		}()
		return verifyWireOutputParityInSession(ctx, session, defaultWireBuildCommands())
	}
}

// verifyWireOutputParityInSession keeps regeneration and compilation in one private clone so the latter can reuse the former's build cache.
func verifyWireOutputParityInSession(ctx context.Context, session CommandSession, buildCommands [][]string) EndpointResult {
	reader, ok := session.(readableCommandSession)
	if !ok {
		return EndpointResult{ID: "wire-output-parity", Status: EndpointFailed, Details: "isolated verifier session cannot read regenerated Wire output"}
	}
	beforePaths, listErr := reader.FilesNamed("wire_gen.go")
	if listErr != nil {
		return EndpointResult{ID: "wire-output-parity", Status: EndpointFailed, Details: fmt.Sprintf("list checked-in Wire output: %v", listErr)}
	}
	if len(beforePaths) > maxWireOutputFiles {
		return EndpointResult{ID: "wire-output-parity", Status: EndpointFailed, Details: fmt.Sprintf("checked-in Wire output exceeds %d files", maxWireOutputFiles)}
	}
	before := make(map[string][sha256.Size]byte, len(beforePaths))
	for _, path := range beforePaths {
		body, readErr := reader.ReadFile(path)
		if readErr != nil {
			return EndpointResult{ID: "wire-output-parity", Status: EndpointFailed, Details: fmt.Sprintf("read checked-in Wire output %q: %v", path, readErr)}
		}
		before[path] = sha256.Sum256(body)
	}
	for _, command := range buildCommands {
		if _, err := session.Run(ctx, command); err != nil {
			return EndpointResult{ID: "wire-output-parity", Status: EndpointFailed, Details: fmt.Sprintf("regenerate Wire through supported build path %q: %v", strings.Join(command, " "), err)}
		}
	}
	afterPaths, listErr := reader.FilesNamed("wire_gen.go")
	if listErr != nil {
		return EndpointResult{ID: "wire-output-parity", Status: EndpointFailed, Details: fmt.Sprintf("list regenerated Wire output: %v", listErr)}
	}
	if !slices.Equal(beforePaths, afterPaths) {
		return EndpointResult{ID: "wire-output-parity", Status: EndpointFailed, Details: "checked-in Wire output paths differ from supported regeneration"}
	}
	for _, path := range beforePaths {
		after, readErr := reader.ReadFile(path)
		if readErr != nil {
			return EndpointResult{ID: "wire-output-parity", Status: EndpointFailed, Details: fmt.Sprintf("read regenerated Wire output %q: %v", path, readErr)}
		}
		if before[path] != sha256.Sum256(after) {
			return EndpointResult{ID: "wire-output-parity", Status: EndpointFailed, Details: fmt.Sprintf("checked-in Wire output %q differs from supported regeneration", path)}
		}
	}
	return EndpointResult{ID: "wire-output-parity", Status: EndpointPassed}
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
		if contract.standard {
			checks = append(checks, runStandardProjectChecks(ctx, verifier.runner, VerifierProject{Root: input.ProjectRoot, BaselineTests: input.BaselineTests}, contract.standardBuilds)...)
			continue
		}
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

// runStandardProjectChecks proves generated parity and compilation in one isolated phase without sharing state with hidden probes.
func runStandardProjectChecks(ctx context.Context, runner CommandRunner, project VerifierProject, buildCommands [][]string) (checks []EndpointResult) {
	session, err := runner.Open(ctx, project)
	if err != nil {
		return []EndpointResult{{ID: "wire-output-parity", Status: EndpointFailed, Details: fmt.Sprintf("open isolated verifier session: %v", err)}}
	}
	defer func() {
		cleanupContext, cancel := context.WithTimeout(context.Background(), verifierCleanupTimeout)
		defer cancel()
		if closeErr := session.Close(cleanupContext); closeErr != nil && !surfaceChecksFailed(checks) {
			checks = append(checks, EndpointResult{ID: "project-compile", Status: EndpointFailed, Details: fmt.Sprintf("close isolated verifier session: %v", closeErr)})
		}
	}()
	parity := verifyWireOutputParityInSession(ctx, session, buildCommands)
	checks = append(checks, parity)
	if parity.Status != EndpointPassed {
		return checks
	}
	return append(checks, runCheck(ctx, session, "project-compile", []string{"go", "test", "./..."}, ""))
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
	declarationNodes := map[string]ast.Node{}
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
		collectSourceFacts(file, &facts, declarations, declarationNodes, assignments)
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
		for _, flow := range declaration.argumentFlows {
			if !declarationArgumentFlowsFromLiteral(declarationNodes[key], flow) {
				return EndpointResult{ID: contract.id, Status: EndpointFailed, Details: fmt.Sprintf("declaration %q does not pass a value resolved from %q to %q", key, flow.literal, flow.call)}
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
	compact := compactSurfaceText(text.String())
	for _, required := range contract.compactText {
		if !strings.Contains(compact, compactSurfaceText(required)) {
			return EndpointResult{ID: contract.id, Status: EndpointFailed, Details: fmt.Sprintf("required configuration %q is absent", required)}
		}
	}
	if contract.commentOnly && !sqlCommentsOnly(text.String()) {
		return EndpointResult{ID: contract.id, Status: EndpointFailed, Details: "migration must not contain executable SQL"}
	}
	return EndpointResult{ID: contract.id, Status: EndpointPassed}
}

// compactSurfaceText removes presentation-only spacing so configuration contracts can verify one complete semantic expression.
func compactSurfaceText(source string) string {
	return strings.Map(func(value rune) rune {
		if unicode.IsSpace(value) {
			return -1
		}
		return value
	}, source)
}

// sqlCommentsOnly accepts SQL comment syntax without treating a generator's particular comment wording as an outcome requirement.
func sqlCommentsOnly(source string) bool {
	inBlockComment := false
	for index := 0; index < len(source); {
		if inBlockComment {
			end := strings.Index(source[index:], "*/")
			if end < 0 {
				return false
			}
			index += end + 2
			inBlockComment = false
			continue
		}
		if source[index] == '/' && index+1 < len(source) && source[index+1] == '*' {
			inBlockComment = true
			index += 2
			continue
		}
		if source[index] == '-' && index+1 < len(source) && source[index+1] == '-' {
			newline := strings.IndexByte(source[index:], '\n')
			if newline < 0 {
				return true
			}
			index += newline + 1
			continue
		}
		character, size := utf8.DecodeRuneInString(source[index:])
		if !unicode.IsSpace(character) {
			return false
		}
		index += size
	}
	return !inBlockComment
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
func collectSourceFacts(file *ast.File, facts *sourceFacts, declarations map[string]*sourceFacts, declarationNodes map[string]ast.Node, assignments map[string]*sourceFacts) {
	collectNodeFacts(file, facts)
	for _, declaration := range file.Decls {
		switch value := declaration.(type) {
		case *ast.FuncDecl:
			collectDeclarationFacts(value.Name.Name, value, declarations, declarationNodes)
			if receiver := functionReceiverName(value); receiver != "" {
				collectDeclarationFacts(receiver+"."+value.Name.Name, value, declarations, declarationNodes)
			}
		case *ast.GenDecl:
			for _, spec := range value.Specs {
				if typeSpec, ok := spec.(*ast.TypeSpec); ok {
					collectDeclarationFacts(typeSpec.Name.Name, typeSpec, declarations, declarationNodes)
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
func collectDeclarationFacts(name string, node ast.Node, declarations map[string]*sourceFacts, declarationNodes map[string]ast.Node) {
	facts := declarations[name]
	if facts == nil {
		value := newSourceFacts()
		facts = &value
		declarations[name] = facts
	}
	declarationNodes[name] = node
	collectNodeFacts(node, facts)
}

// declarationArgumentFlowsFromLiteral follows local values from a literal-bearing accessor call into a constructor call.
func declarationArgumentFlowsFromLiteral(node ast.Node, contract callArgumentFlowContract) bool {
	function, ok := node.(*ast.FuncDecl)
	if !ok || function.Body == nil {
		return false
	}
	values := map[string]ast.Expr{}
	ast.Inspect(function.Body, func(node ast.Node) bool {
		switch statement := node.(type) {
		case *ast.AssignStmt:
			for index, left := range statement.Lhs {
				name, ok := left.(*ast.Ident)
				if ok && index < len(statement.Rhs) {
					values[name.Name] = statement.Rhs[index]
				}
			}
		case *ast.ValueSpec:
			for index, name := range statement.Names {
				if index < len(statement.Values) {
					values[name.Name] = statement.Values[index]
				}
			}
		}
		return true
	})
	resolved := map[string]bool{}
	for changed := true; changed; {
		changed = false
		for name, value := range values {
			if resolved[name] || expressionFlowsFromLiteral(value, contract.literal, resolved) {
				if !resolved[name] {
					resolved[name] = true
					changed = true
				}
			}
		}
	}
	flows := false
	ast.Inspect(function.Body, func(node ast.Node) bool {
		call, ok := node.(*ast.CallExpr)
		if !ok || callName(call) != contract.call {
			return true
		}
		for _, argument := range call.Args {
			if expressionFlowsFromLiteral(argument, contract.literal, resolved) {
				flows = true
				return false
			}
		}
		return true
	})
	return flows
}

// expressionFlowsFromLiteral recognizes an accessor call carrying the literal or a local value derived from one.
func expressionFlowsFromLiteral(expression ast.Expr, literal string, resolved map[string]bool) bool {
	found := false
	ast.Inspect(expression, func(node ast.Node) bool {
		switch value := node.(type) {
		case *ast.Ident:
			if resolved[value.Name] {
				found = true
			}
		case *ast.CallExpr:
			if callName(value) != "" && callContainsStringLiteral(value, literal) {
				found = true
			}
		}
		return !found
	})
	return found
}

// callContainsStringLiteral keeps provider checks independent of the environment or configuration accessor spelling.
func callContainsStringLiteral(call *ast.CallExpr, literal string) bool {
	found := false
	for _, argument := range call.Args {
		ast.Inspect(argument, func(node ast.Node) bool {
			value, ok := node.(*ast.BasicLit)
			if ok && value.Kind == token.STRING && strings.Trim(value.Value, "`\"") == literal {
				found = true
			}
			return !found
		})
		if found {
			return true
		}
	}
	return false
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
